<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import {
  NCard,
  NForm,
  NFormItem,
  NInput,
  NButton,
  NText,
  NProgress,
  NSelect,
  NSpace,
  NIcon,
  NTag,
  NAlert,
  NModal,
} from 'naive-ui'
import { CheckmarkCircle } from '@vicons/ionicons5'
import { useAuth } from '../composables/useAuth'

const router = useRouter()
const message = useMessage()
const auth = useAuth()

// authedFetch adds the Bearer token to outbound requests so setup
// steps 2 and 3 authenticate like every other admin call. POST /api/setup
// is the one exception — no user exists yet, so it's the only
// unauthenticated call on this page.
function authedFetch(path, opts = {}) {
  const headers = { ...(opts.headers || {}), ...auth.getAuthHeader() }
  return fetch(path, { ...opts, headers, credentials: 'include' })
}

const currentStep = ref(1)
const loading = ref(false)

// Step 1: Account form
const accountForm = ref({
  username: 'admin',
  password: '',
  confirmPassword: '',
})

// Step 2: Profile form
const profileForm = ref({
  agentName: '',
  personality: null,
  userName: '',
  timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
})

// Step 3: Provider login + default model
const providers = ref([])
const apiKeyInputs = ref({ openai: '', gemini: '' })
const ollamaURL = ref('http://localhost:11434')
const models = ref([])
const selectedModel = ref(null)
const deviceFlow = ref(null)
let deviceFlowPoll = null

const personalityOptions = [
  { label: 'Friendly & Warm', value: 'friendly', description: 'Warm, conversational, and approachable' },
  { label: 'Professional & Concise', value: 'professional', description: 'Business-like and to-the-point' },
  { label: 'Witty & Playful', value: 'witty', description: 'Quick-witted with a lighthearted touch' },
  { label: 'Calm & Thoughtful', value: 'calm', description: 'Patient and considered' },
  { label: 'Direct & No-Nonsense', value: 'direct', description: 'Straight to the point' },
  { label: 'Curious & Enthusiastic', value: 'curious', description: 'Genuinely excited about problems and ideas' },
  { label: 'Dry & Sardonic', value: 'sardonic', description: 'Understated humor with a hint of sarcasm' },
  { label: 'Supportive & Encouraging', value: 'supportive', description: 'Patient, uplifting, and focused on helping' },
  { label: 'Creative & Expressive', value: 'creative', description: 'Imaginative and colorful' },
  { label: 'Balanced & Adaptive', value: 'balanced', description: 'Adjusts tone to the situation' },
]

const timezoneOptions = computed(() => {
  try {
    const zones = Intl.supportedValuesOf('timeZone')
    return zones.map(tz => ({ label: tz, value: tz }))
  } catch {
    const common = [
      'UTC', 'America/New_York', 'America/Chicago', 'America/Denver',
      'America/Los_Angeles', 'Europe/London', 'Europe/Paris', 'Europe/Berlin',
      'Asia/Tokyo', 'Asia/Shanghai', 'Asia/Kolkata', 'Australia/Sydney',
      'Pacific/Auckland',
    ]
    return common.map(tz => ({ label: tz, value: tz }))
  }
})

const passwordStrength = computed(() => {
  const pwd = accountForm.value.password
  if (!pwd) return 0

  let score = 0
  if (pwd.length >= 16) return 100
  if (pwd.length >= 12) score += 40
  else if (pwd.length >= 8) score += 20

  if (/[A-Z]/.test(pwd)) score += 15
  if (/[a-z]/.test(pwd)) score += 15
  if (/[0-9]/.test(pwd)) score += 15
  if (/[!@#$%^&*()_+\-=\[\]{}|;':",.<>?/~`]/.test(pwd)) score += 15

  return Math.min(100, score)
})

const passwordStatus = computed(() => {
  if (passwordStrength.value >= 70) return 'success'
  if (passwordStrength.value >= 40) return 'warning'
  return 'error'
})

const passwordHint = computed(() => {
  const pwd = accountForm.value.password
  if (!pwd) return 'Enter a password'
  if (pwd.length >= 16) return 'Strong passphrase!'
  if (pwd.length < 12) return 'Must be 12+ characters with complexity, or 16+ characters'

  const checks = []
  if (!/[A-Z]/.test(pwd)) checks.push('uppercase')
  if (!/[a-z]/.test(pwd)) checks.push('lowercase')
  if (!/[0-9]/.test(pwd)) checks.push('number')
  if (!/[!@#$%^&*()_+\-=\[\]{}|;':",.<>?/~`]/.test(pwd)) checks.push('symbol')

  if (checks.length > 1) return `Add: ${checks.join(', ')}`
  return 'Good complexity!'
})

const profileValid = computed(() => {
  return profileForm.value.agentName.trim() !== '' &&
    profileForm.value.personality !== null &&
    profileForm.value.userName.trim() !== '' &&
    profileForm.value.timezone.trim() !== ''
})

const subtitle = computed(() => {
  if (currentStep.value === 1) return 'Create Admin Account'
  if (currentStep.value === 2) return 'Personalize Your AI'
  return 'Pick an LLM Provider'
})

const anyAuthenticated = computed(() => providers.value.some(p => p.authenticated))
const defaultModelChosen = computed(() => selectedModel.value !== null)

const modelOptions = computed(() =>
  models.value.map(m => ({
    label: `${m.provider}/${m.model}${m.context_window ? ' — ' + Math.round(m.context_window / 1000) + 'k' : ''}`,
    value: JSON.stringify({ provider: m.provider, model: m.model, endpoint: m.endpoint || '' }),
  }))
)

// -------- Setup status + step navigation --------
onMounted(async () => {
  try {
    const response = await fetch('/api/setup/status')
    const data = await response.json()
    if (data.setup_step === 'profile' || data.setup_step === 'provider') {
      // A refresh cookie was minted when the account was created; if
      // the user reloaded mid-wizard we can exchange it for an access
      // token so steps 2 and 3 keep authenticating cleanly. If the
      // cookie is gone (expired / cleared) the caller stays on step 1
      // effectively — the auth-required endpoints will 401 and the
      // user's only option is to start over.
      if (!auth.isAuthenticated.value) {
        await auth.refreshToken()
      }
    }
    if (data.setup_step === 'profile') {
      currentStep.value = 2
    } else if (data.setup_step === 'provider') {
      currentStep.value = 3
      await loadProviders()
      await loadModels()
    } else if (data.setup_step === 'complete') {
      router.push('/login')
    }
  } catch (e) {
    // Silent; user can still attempt step 1.
  }
})

onBeforeUnmount(() => stopPolling())

// -------- Step 1 --------
async function handleAccountSubmit() {
  if (accountForm.value.password !== accountForm.value.confirmPassword) {
    message.error('Passwords do not match')
    return
  }

  loading.value = true
  try {
    const response = await fetch('/api/setup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include', // so the refresh cookie set by the server sticks
      body: JSON.stringify({
        username: accountForm.value.username,
        password: accountForm.value.password,
        confirm_password: accountForm.value.confirmPassword,
      }),
    })
    const data = await response.json()
    if (!response.ok) {
      message.error(data.message || 'Setup failed')
      return
    }
    // The server issued a refresh cookie — swap it for an access
    // token so the rest of the wizard runs authenticated.
    const ok = await auth.refreshToken()
    if (!ok) {
      message.error('Account created but session could not be established. Please log in manually.')
      router.push('/login')
      return
    }
    message.success('Account created')
    currentStep.value = 2
  } catch (e) {
    message.error('Setup failed: ' + e.message)
  } finally {
    loading.value = false
  }
}

// -------- Step 2 --------
async function handleProfileSubmit() {
  loading.value = true
  try {
    const response = await authedFetch('/api/setup/profile', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        agent_name: profileForm.value.agentName,
        personality: profileForm.value.personality,
        user_name: profileForm.value.userName,
        timezone: profileForm.value.timezone,
      }),
    })
    const data = await response.json()
    if (!response.ok) {
      message.error(data.message || 'Profile setup failed')
      return
    }
    message.success('Profile saved')
    currentStep.value = 3
    await loadProviders()
    await loadModels()
  } catch (e) {
    message.error('Profile setup failed: ' + e.message)
  } finally {
    loading.value = false
  }
}

// -------- Step 3: provider login --------
async function loadProviders() {
  try {
    const response = await authedFetch('/api/engine/providers')
    if (response.ok) {
      const data = await response.json()
      providers.value = data.providers || []
    }
  } catch (e) { /* non-fatal */ }
}

async function loadModels() {
  try {
    const response = await authedFetch('/api/engine/models')
    if (response.ok) {
      const data = await response.json()
      models.value = data.models || []
    }
    const def = await authedFetch('/api/engine/default')
    if (def.ok) {
      const d = await def.json()
      if (d.set) {
        selectedModel.value = JSON.stringify({
          provider: d.provider, model: d.model, endpoint: d.endpoint || '',
        })
      }
    }
  } catch (e) { /* non-fatal */ }
}

async function loginAPIKey(providerName) {
  const key = (apiKeyInputs.value[providerName] || '').trim()
  if (!key) {
    message.warning('Enter an API key first')
    return
  }
  const response = await authedFetch(`/api/engine/providers/${providerName}/login`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key }),
  })
  if (response.ok) {
    message.success(`${providerName} authenticated`)
    apiKeyInputs.value[providerName] = ''
    await loadProviders()
    await loadModels()
  } else {
    const data = await response.json().catch(() => ({}))
    message.error(data.error || 'Login failed')
  }
}

async function loginOllama() {
  const url = (ollamaURL.value || '').trim() || 'http://localhost:11434'
  const response = await authedFetch('/api/engine/providers/ollama/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ base_url: url }),
  })
  if (response.ok) {
    message.success('Ollama URL saved')
    await loadProviders()
    await loadModels()
  } else {
    const data = await response.json().catch(() => ({}))
    message.error(data.error || 'Failed to save')
  }
}

async function startDeviceFlow(providerName) {
  const path = providerName === 'openai'
    ? '/api/engine/providers/openai/oauth/login'
    : `/api/engine/providers/${providerName}/login`
  const response = await authedFetch(path, { method: 'POST' })
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
}

function startPolling(providerName) {
  stopPolling()
  deviceFlowPoll = setInterval(async () => {
    const path = providerName === 'openai'
      ? '/api/engine/providers/openai/oauth/status'
      : `/api/engine/providers/${providerName}/status`
    try {
      const response = await authedFetch(path)
      if (!response.ok) return
      const data = await response.json()
      if (!deviceFlow.value) return
      deviceFlow.value.status = data.status
      deviceFlow.value.error = data.error || ''
      if (data.status === 'authenticated') {
        stopPolling()
        message.success('Signed in!')
        deviceFlow.value = null
        await loadProviders()
        await loadModels()
      } else if (data.status === 'error') {
        stopPolling()
        message.error(data.error || 'Sign-in failed')
      }
    } catch (e) { /* keep polling */ }
  }, 2000)
}

function stopPolling() {
  if (deviceFlowPoll) { clearInterval(deviceFlowPoll); deviceFlowPoll = null }
}

function cancelDeviceFlow() { stopPolling(); deviceFlow.value = null }

async function setDefaultModel(value) {
  if (!value) return
  const info = JSON.parse(value)
  const response = await authedFetch('/api/engine/default', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(info),
  })
  if (response.ok) {
    message.success(`Default model set to ${info.provider}/${info.model}`)
    selectedModel.value = value
  } else {
    const data = await response.json().catch(() => ({}))
    message.error(data.error || 'Failed to set default model')
  }
}

async function finishSetup() {
  // Ask the backend to mark the wizard complete. The server verifies
  // a default model is set before flipping the flag — on failure we
  // show the server's error and stay on step 3 so the user can fix it.
  // Previously this fetch was try/catch'd and the result ignored, so
  // a failed call still triggered a "Setup complete" redirect.
  let response
  try {
    response = await authedFetch('/api/setup/provider', { method: 'POST' })
  } catch (e) {
    message.error('Could not reach the server: ' + e.message)
    return
  }
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    message.error(data.message || data.error || 'Failed to finalize setup')
    return
  }

  message.success('Setup complete — please log in')
  // Drop the refresh cookie so the user goes through the normal
  // login path rather than silently staying signed in as admin.
  await auth.logout()
  router.push('/login')
}

function providerLabel(name) {
  switch (name) {
    case 'openai':  return 'OpenAI'
    case 'gemini':  return 'Google Gemini'
    case 'copilot': return 'GitHub Copilot'
    case 'ollama':  return 'Ollama'
    default:        return name
  }
}
</script>

<template>
  <div class="setup-container">
    <div class="setup-card-wrapper">
      <div class="setup-branding">
        <img src="/assets/logo-full.svg" alt="OpenPact" class="setup-logo" />
        <h1 class="setup-brand-title">OpenPact</h1>
        <p class="setup-brand-subtitle">{{ subtitle }}</p>
      </div>

      <!-- Step indicator -->
      <div class="step-indicator">
        <div class="step-dot" :class="{ active: currentStep === 1, completed: currentStep > 1 }"></div>
        <div class="step-line" :class="{ completed: currentStep > 1 }"></div>
        <div class="step-dot" :class="{ active: currentStep === 2, completed: currentStep > 2 }"></div>
        <div class="step-line" :class="{ completed: currentStep > 2 }"></div>
        <div class="step-dot" :class="{ active: currentStep === 3 }"></div>
      </div>

      <!-- Step 1: Account -->
      <n-card v-if="currentStep === 1" class="setup-card">
        <n-form @submit.prevent="handleAccountSubmit">
          <n-form-item label="Username">
            <n-input v-model:value="accountForm.username" placeholder="admin" :disabled="loading" size="large" />
          </n-form-item>
          <n-form-item label="Password">
            <n-input
              v-model:value="accountForm.password"
              type="password"
              show-password-on="click"
              placeholder="Enter a secure password"
              :disabled="loading"
              size="large"
            />
          </n-form-item>
          <n-progress
            type="line"
            :percentage="passwordStrength"
            :status="passwordStatus"
            :show-indicator="false"
            style="margin-bottom: 8px"
          />
          <n-text :depth="3" style="font-size: 12px">{{ passwordHint }}</n-text>
          <n-form-item label="Confirm Password" style="margin-top: 16px">
            <n-input
              v-model:value="accountForm.confirmPassword"
              type="password"
              show-password-on="click"
              placeholder="Confirm your password"
              :disabled="loading"
              size="large"
            />
          </n-form-item>
          <n-button
            type="primary"
            attr-type="submit"
            :loading="loading"
            :disabled="passwordStrength < 70"
            block
            size="large"
            style="margin-top: 8px"
          >
            Next
          </n-button>
        </n-form>
      </n-card>

      <!-- Step 2: Profile -->
      <n-card v-else-if="currentStep === 2" class="setup-card">
        <n-form @submit.prevent="handleProfileSubmit">
          <n-form-item label="Agent Name">
            <n-input v-model:value="profileForm.agentName" placeholder="e.g. Atlas, Nova, Sage..." :disabled="loading" size="large" />
          </n-form-item>
          <n-form-item label="Personality">
            <n-select v-model:value="profileForm.personality" :options="personalityOptions" placeholder="Choose a personality" :disabled="loading" size="large" />
          </n-form-item>
          <n-form-item label="Your Name">
            <n-input v-model:value="profileForm.userName" placeholder="What should the AI call you?" :disabled="loading" size="large" />
          </n-form-item>
          <n-form-item label="Timezone">
            <n-select v-model:value="profileForm.timezone" :options="timezoneOptions" filterable placeholder="Select your timezone" :disabled="loading" size="large" />
          </n-form-item>
          <n-button
            type="primary"
            attr-type="submit"
            :loading="loading"
            :disabled="!profileValid"
            block
            size="large"
            style="margin-top: 8px"
          >
            Next
          </n-button>
        </n-form>
      </n-card>

      <!-- Step 3: Provider + default model -->
      <n-card v-else-if="currentStep === 3" class="setup-card">
        <n-alert type="info" style="margin-bottom: 12px">
          Sign in to at least one LLM provider, then pick a default model.
        </n-alert>

        <div v-for="p in providers" :key="p.name" class="provider-row">
          <div class="provider-head">
            <strong>{{ providerLabel(p.name) }}</strong>
            <n-tag v-if="p.authenticated" size="small" type="success">
              <template #icon><n-icon :component="CheckmarkCircle" /></template>
              Authenticated
            </n-tag>
          </div>

          <div v-if="p.name === 'openai'" class="provider-body">
            <n-space vertical>
              <n-input
                v-model:value="apiKeyInputs.openai"
                type="password"
                show-password-on="click"
                placeholder="sk-..."
                :disabled="p.authenticated"
              />
              <n-space>
                <n-button size="small" type="primary" :disabled="p.authenticated" @click="loginAPIKey('openai')">
                  Save API key
                </n-button>
                <n-button size="small" :disabled="p.authenticated" @click="startDeviceFlow('openai')">
                  Sign in with ChatGPT
                </n-button>
              </n-space>
            </n-space>
          </div>

          <div v-else-if="p.name === 'gemini'" class="provider-body">
            <n-space vertical>
              <n-input
                v-model:value="apiKeyInputs.gemini"
                type="password"
                show-password-on="click"
                placeholder="AIza..."
                :disabled="p.authenticated"
              />
              <n-button size="small" type="primary" :disabled="p.authenticated" @click="loginAPIKey('gemini')">
                Save API key
              </n-button>
            </n-space>
          </div>

          <div v-else-if="p.name === 'copilot'" class="provider-body">
            <n-button size="small" type="primary" :disabled="p.authenticated" @click="startDeviceFlow('copilot')">
              Sign in with GitHub
            </n-button>
          </div>

          <div v-else-if="p.name === 'ollama'" class="provider-body">
            <n-space vertical>
              <n-input v-model:value="ollamaURL" placeholder="http://localhost:11434" />
              <n-button size="small" type="primary" @click="loginOllama">
                {{ p.authenticated ? 'Update URL' : 'Save URL' }}
              </n-button>
            </n-space>
          </div>
        </div>

        <div v-if="anyAuthenticated" class="mt-4">
          <n-form-item label="Default model">
            <n-select
              :options="modelOptions"
              :value="selectedModel"
              placeholder="Pick a default model"
              filterable
              @update:value="setDefaultModel"
            />
          </n-form-item>
        </div>

        <n-button
          type="primary"
          block
          size="large"
          :disabled="!defaultModelChosen"
          style="margin-top: 8px"
          @click="finishSetup"
        >
          Finish setup
        </n-button>
      </n-card>

      <!-- Device flow modal (shared between step 3 and elsewhere) -->
      <n-modal
        :show="!!deviceFlow"
        preset="card"
        :title="deviceFlow ? `Sign in to ${providerLabel(deviceFlow.provider)}` : ''"
        style="width: 460px; border-radius: 12px"
        :mask-closable="false"
        @update:show="v => { if (!v) cancelDeviceFlow() }"
      >
        <template v-if="deviceFlow">
          <n-space vertical size="large">
            <div>Enter this code at the verification URL:</div>
            <div class="device-code">
              <span class="code">{{ deviceFlow.userCode }}</span>
            </div>
            <div>
              <a :href="deviceFlow.verifyUrl" target="_blank" rel="noopener">{{ deviceFlow.verifyUrl }}</a>
            </div>
            <n-alert v-if="deviceFlow.status === 'pending'" type="info">
              Waiting for you to complete authorisation...
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
  </div>
</template>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: var(--background);
}
.setup-card-wrapper { width: 100%; max-width: 420px; }
.setup-branding { display: flex; flex-direction: column; align-items: center; margin-bottom: 24px; }
.setup-logo { width: 72px; height: 72px; border-radius: 16px; margin-bottom: 16px; filter: drop-shadow(0 4px 12px rgba(45, 215, 183, 0.15)); }
.setup-brand-title { margin: 0; font-size: 28px; font-weight: 700; letter-spacing: -0.5px; color: var(--primary-color); }
.setup-brand-subtitle { margin: 4px 0 0; font-size: 14px; color: var(--primary-color); opacity: 0.5; letter-spacing: 0.5px; }
.step-indicator { display: flex; align-items: center; justify-content: center; gap: 0; margin-bottom: 24px; }
.step-dot { width: 10px; height: 10px; border-radius: 50%; background: rgba(128, 128, 128, 0.3); transition: background 0.3s ease; }
.step-dot.active { background: #2dd7b7; box-shadow: 0 0 8px rgba(45, 215, 183, 0.4); }
.step-dot.completed { background: #2387a7; }
.step-line { width: 40px; height: 2px; background: rgba(128, 128, 128, 0.3); transition: background 0.3s ease; }
.step-line.completed { background: #2387a7; }
.setup-card { border-radius: 16px; }
.provider-row { padding: 10px 0; border-top: 1px solid rgba(128,128,128,0.15); }
.provider-row:first-of-type { border-top: 0; }
.provider-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.provider-body { margin-top: 4px; }
.device-code { text-align: center; padding: 16px; background: var(--n-color); border-radius: 8px; }
.device-code .code { font-family: monospace; font-size: 22px; letter-spacing: 2px; font-weight: bold; user-select: all; }
</style>
