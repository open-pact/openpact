<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import {
  NCard,
  NButton,
  NInput,
  NSpace,
  NTag,
  NSelect,
  NSpin,
  NIcon,
  NAlert,
  NEmpty,
  NModal,
  NText,
} from 'naive-ui'
import {
  LogoGoogle,
  LogoGithub,
  SparklesOutline,
  CloudOfflineOutline,
  CheckmarkCircle,
  LogOutOutline,
} from '@vicons/ionicons5'
import Card from '@/components/shared/Card.vue'

const api = useApi()
const message = useMessage()
const dialog = useDialog()

// Provider status
const providers = ref([])
const providersLoading = ref(true)

// Models
const models = ref([])
const modelsLoading = ref(false)
const selectedModel = ref(null)

// Active device flow (Copilot / OpenAI OAuth)
const deviceFlow = ref(null) // { provider, userCode, verifyUrl, status }
let deviceFlowPoll = null

// Per-provider input state
const apiKeyInputs = ref({ openai: '', gemini: '' })
const ollamaURL = ref('http://localhost:11434')
const showKeys = ref({ openai: false, gemini: false })

// ---- Providers ----
async function loadProviders() {
  providersLoading.value = true
  try {
    const response = await api.get('/api/engine/providers')
    if (response.ok) {
      const data = await response.json()
      providers.value = data.providers || []
    } else {
      message.error('Failed to load providers')
    }
  } catch (e) {
    message.error('Failed to load providers')
  } finally {
    providersLoading.value = false
  }
}

function providerMeta(name) {
  switch (name) {
    case 'openai':  return { label: 'OpenAI',         icon: SparklesOutline }
    case 'gemini':  return { label: 'Google Gemini',  icon: LogoGoogle }
    case 'copilot': return { label: 'GitHub Copilot', icon: LogoGithub }
    case 'ollama':  return { label: 'Ollama',         icon: CloudOfflineOutline }
    default:        return { label: name,             icon: CloudOfflineOutline }
  }
}

async function loginAPIKey(providerName) {
  const key = (apiKeyInputs.value[providerName] || '').trim()
  if (!key) {
    message.warning('Enter an API key first')
    return
  }
  try {
    const response = await api.post(`/api/engine/providers/${providerName}/login`, { key })
    if (response.ok) {
      message.success(`${providerMeta(providerName).label} authenticated`)
      apiKeyInputs.value[providerName] = ''
      showKeys.value[providerName] = false
      await loadProviders()
      await loadModels()
    } else {
      const data = await response.json()
      message.error(data.error || 'Login failed')
    }
  } catch (e) {
    message.error('Login failed: ' + e.message)
  }
}

async function loginOllama() {
  const url = (ollamaURL.value || '').trim() || 'http://localhost:11434'
  try {
    const response = await api.post('/api/engine/providers/ollama/login', { base_url: url })
    if (response.ok) {
      message.success('Ollama base URL saved')
      await loadProviders()
      await loadModels()
    } else {
      const data = await response.json()
      message.error(data.error || 'Failed to save URL')
    }
  } catch (e) {
    message.error('Failed to save URL: ' + e.message)
  }
}

async function startDeviceFlow(providerName) {
  const path = providerName === 'openai'
    ? '/api/engine/providers/openai/oauth/login'
    : `/api/engine/providers/${providerName}/login`
  try {
    const response = await api.post(path, {})
    if (!response.ok) {
      const data = await response.json().catch(() => ({}))
      message.error(data.error || 'Failed to start device flow')
      return
    }
    const data = await response.json()
    deviceFlow.value = {
      provider: providerName,
      userCode: data.user_code || '',
      verifyUrl: data.verify_url || '',
      status: data.status || 'pending',
      error: data.error || '',
    }
    startPolling(providerName)
  } catch (e) {
    message.error('Failed to start device flow: ' + e.message)
  }
}

function startPolling(providerName) {
  if (deviceFlowPoll) clearInterval(deviceFlowPoll)
  deviceFlowPoll = setInterval(async () => {
    const path = providerName === 'openai'
      ? '/api/engine/providers/openai/oauth/status'
      : `/api/engine/providers/${providerName}/status`
    try {
      const response = await api.get(path)
      if (!response.ok) return
      const data = await response.json()
      if (!deviceFlow.value) return
      deviceFlow.value.status = data.status
      deviceFlow.value.error = data.error || ''
      if (data.status === 'authenticated') {
        stopPolling()
        message.success(`${providerMeta(providerName).label} signed in!`)
        deviceFlow.value = null
        await loadProviders()
        await loadModels()
      } else if (data.status === 'error') {
        stopPolling()
        message.error('Device flow failed: ' + (data.error || 'unknown error'))
      }
    } catch (e) {
      // Keep polling through transient errors.
    }
  }, 2000)
}

function stopPolling() {
  if (deviceFlowPoll) {
    clearInterval(deviceFlowPoll)
    deviceFlowPoll = null
  }
}

function cancelDeviceFlow() {
  stopPolling()
  deviceFlow.value = null
}

function confirmLogout(providerName) {
  const label = providerMeta(providerName).label
  dialog.warning({
    title: `Sign out of ${label}?`,
    content: 'This clears stored credentials. You will need to sign in again to use this provider.',
    positiveText: 'Sign out',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        const response = await api.post(`/api/engine/providers/${providerName}/logout`, {})
        if (response.ok) {
          message.success(`${label} signed out`)
          await loadProviders()
          await loadModels()
        } else {
          const data = await response.json()
          message.error(data.error || 'Logout failed')
        }
      } catch (e) {
        message.error('Logout failed: ' + e.message)
      }
    },
  })
}

// ---- Models ----
const modelOptions = computed(() =>
  models.value.map(m => ({
    label: `${m.provider}/${m.model}${m.context_window ? ' — ' + Math.round(m.context_window / 1000) + 'k' : ''}`,
    value: JSON.stringify({ provider: m.provider, model: m.model, endpoint: m.endpoint || '' }),
  }))
)

async function loadModels() {
  modelsLoading.value = true
  try {
    const response = await api.get('/api/engine/models')
    if (response.ok) {
      const data = await response.json()
      models.value = data.models || []
    }
    const defResp = await api.get('/api/engine/default')
    if (defResp.ok) {
      const def = await defResp.json()
      if (def.set) {
        selectedModel.value = JSON.stringify({
          provider: def.provider,
          model: def.model,
          endpoint: def.endpoint || '',
        })
      }
    }
  } catch (e) {
    // Silent — no models is a valid state until the user authenticates.
  } finally {
    modelsLoading.value = false
  }
}

async function setDefaultModel(value) {
  if (!value) return
  const info = JSON.parse(value)
  try {
    const response = await api.post('/api/engine/default', info)
    if (response.ok) {
      message.success(`Default model set to ${info.provider}/${info.model}`)
      selectedModel.value = value
    } else {
      const data = await response.json()
      message.error(data.error || 'Failed to set default model')
    }
  } catch (e) {
    message.error('Failed to set default model: ' + e.message)
  }
}

onMounted(async () => {
  await loadProviders()
  await loadModels()
})

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<template>
  <div>
    <div class="page-header">
      <h2 class="page-title">Engine</h2>
    </div>

    <n-alert type="info" style="margin-bottom: 16px">
      Sign in to at least one provider, then pick a default model.
      Credentials are stored under <code>secure/data/stackllm_auth.json</code>; delete them any time with Sign out.
    </n-alert>

    <!-- Provider grid -->
    <Card title="LLM providers">
      <n-spin v-if="providersLoading" size="small" style="display: block; margin: 24px auto" />

      <div v-else class="provider-grid">
        <n-card v-for="p in providers" :key="p.name" class="provider-card" size="small">
          <div class="provider-head">
            <n-icon size="22" :component="providerMeta(p.name).icon" />
            <span class="provider-label">{{ providerMeta(p.name).label }}</span>
            <n-tag v-if="p.authenticated" size="small" type="success" style="margin-left: auto">
              <template #icon><n-icon :component="CheckmarkCircle" /></template>
              Authenticated
            </n-tag>
            <n-tag v-else size="small" style="margin-left: auto">Not signed in</n-tag>
          </div>

          <!-- OpenAI: API key OR device flow -->
          <div v-if="p.name === 'openai'" class="provider-body">
            <n-space vertical>
              <n-input
                v-model:value="apiKeyInputs.openai"
                :type="showKeys.openai ? 'text' : 'password'"
                placeholder="sk-..."
                :disabled="p.authenticated"
              />
              <n-space>
                <n-button
                  size="small"
                  type="primary"
                  :disabled="p.authenticated"
                  @click="loginAPIKey('openai')"
                >
                  Save API key
                </n-button>
                <n-button
                  size="small"
                  :disabled="p.authenticated"
                  @click="startDeviceFlow('openai')"
                >
                  Sign in with ChatGPT
                </n-button>
                <n-button
                  v-if="p.authenticated"
                  size="small"
                  type="error"
                  quaternary
                  @click="confirmLogout('openai')"
                >
                  <template #icon><n-icon :component="LogOutOutline" /></template>
                  Sign out
                </n-button>
              </n-space>
            </n-space>
          </div>

          <!-- Gemini: API key only -->
          <div v-else-if="p.name === 'gemini'" class="provider-body">
            <n-space vertical>
              <n-input
                v-model:value="apiKeyInputs.gemini"
                :type="showKeys.gemini ? 'text' : 'password'"
                placeholder="AIza..."
                :disabled="p.authenticated"
              />
              <n-space>
                <n-button
                  size="small"
                  type="primary"
                  :disabled="p.authenticated"
                  @click="loginAPIKey('gemini')"
                >
                  Save API key
                </n-button>
                <n-button
                  v-if="p.authenticated"
                  size="small"
                  type="error"
                  quaternary
                  @click="confirmLogout('gemini')"
                >
                  <template #icon><n-icon :component="LogOutOutline" /></template>
                  Sign out
                </n-button>
              </n-space>
            </n-space>
          </div>

          <!-- Copilot: device flow only -->
          <div v-else-if="p.name === 'copilot'" class="provider-body">
            <n-space>
              <n-button
                size="small"
                type="primary"
                :disabled="p.authenticated"
                @click="startDeviceFlow('copilot')"
              >
                Sign in with GitHub
              </n-button>
              <n-button
                v-if="p.authenticated"
                size="small"
                type="error"
                quaternary
                @click="confirmLogout('copilot')"
              >
                <template #icon><n-icon :component="LogOutOutline" /></template>
                Sign out
              </n-button>
            </n-space>
          </div>

          <!-- Ollama: base URL -->
          <div v-else-if="p.name === 'ollama'" class="provider-body">
            <n-space vertical>
              <n-input
                v-model:value="ollamaURL"
                placeholder="http://localhost:11434"
              />
              <n-space>
                <n-button
                  size="small"
                  type="primary"
                  @click="loginOllama"
                >
                  {{ p.authenticated ? 'Update URL' : 'Save URL' }}
                </n-button>
                <n-button
                  v-if="p.authenticated"
                  size="small"
                  type="error"
                  quaternary
                  @click="confirmLogout('ollama')"
                >
                  <template #icon><n-icon :component="LogOutOutline" /></template>
                  Forget
                </n-button>
              </n-space>
            </n-space>
          </div>
        </n-card>
      </div>
    </Card>

    <!-- Model picker -->
    <Card title="Default model" style="margin-top: 16px">
      <n-spin v-if="modelsLoading" size="small" style="display: block; margin: 24px auto" />
      <div v-else-if="!models.length" class="p-6 text-center">
        <n-empty description="Sign in to a provider to see available models." />
      </div>
      <div v-else class="p-4">
        <n-select
          :options="modelOptions"
          :value="selectedModel"
          placeholder="Pick a default model"
          filterable
          @update:value="setDefaultModel"
        />
      </div>
    </Card>

    <!-- Device flow modal -->
    <n-modal
      :show="!!deviceFlow"
      preset="card"
      :title="deviceFlow ? `Sign in to ${providerMeta(deviceFlow.provider).label}` : ''"
      style="width: 520px; border-radius: 12px"
      :mask-closable="false"
      @update:show="v => { if (!v) cancelDeviceFlow() }"
    >
      <template v-if="deviceFlow">
        <n-space vertical size="large">
          <div>
            Enter this code at the verification URL:
          </div>
          <div class="device-code">
            <span class="code">{{ deviceFlow.userCode }}</span>
          </div>
          <div>
            <a :href="deviceFlow.verifyUrl" target="_blank" rel="noopener">{{ deviceFlow.verifyUrl }}</a>
          </div>
          <n-alert v-if="deviceFlow.status === 'pending'" type="info">
            Waiting for you to complete authorisation in your browser...
          </n-alert>
          <n-alert v-if="deviceFlow.status === 'error'" type="error">
            {{ deviceFlow.error || 'Authorisation failed.' }}
          </n-alert>
        </n-space>
      </template>
      <template #footer>
        <n-space justify="end">
          <n-button @click="cancelDeviceFlow">Close</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.provider-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
  padding: 8px;
}
.provider-card {
  border-radius: 10px;
}
.provider-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.provider-label {
  font-weight: 600;
}
.provider-body {
  margin-top: 6px;
}
.device-code {
  text-align: center;
  padding: 16px;
  background: var(--n-color);
  border-radius: 8px;
}
.device-code .code {
  font-family: monospace;
  font-size: 22px;
  letter-spacing: 2px;
  font-weight: bold;
  user-select: all;
}
</style>
