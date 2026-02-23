<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import {
  NCard,
  NSpace,
  NButton,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NSwitch,
  NTag,
  NIcon,
  NDynamicTags,
  NEmpty,
  NSpin,
  NAlert,
} from 'naive-ui'
import { h } from 'vue'

const message = useMessage()
const api = useApi()

const providers = ref([])
const loading = ref(true)
let pollTimer = null

// Edit modal
const showEditModal = ref(false)
const editProvider = ref({
  name: '',
  enabled: false,
  allowed_users: [],
  allowed_chans: [],
})
const editTokens = ref({})
const saving = ref(false)

const providerLabels = {
  discord: 'Discord',
  telegram: 'Telegram',
  slack: 'Slack',
}

const tokenKeyLabels = {
  token: 'Bot Token',
  bot_token: 'Bot Token',
  app_token: 'App Token',
}

function statusType(state) {
  switch (state) {
    case 'connected': return 'success'
    case 'starting': return 'warning'
    case 'error': return 'error'
    default: return 'default'
  }
}

function statusLabel(state) {
  switch (state) {
    case 'connected': return 'Connected'
    case 'starting': return 'Starting...'
    case 'error': return 'Error'
    case 'stopped': return 'Stopped'
    default: return 'Unknown'
  }
}

function tokenSourceLabel(info) {
  if (info.token_source === 'store') return 'Stored'
  if (info.token_source === 'env') return 'Env var'
  return 'Not set'
}

function tokenSourceType(info) {
  if (info.token_source === 'store') return 'success'
  if (info.token_source === 'env') return 'info'
  return 'default'
}

function providerState(p) {
  return p.status?.state || 'stopped'
}

function isRunning(p) {
  const s = providerState(p)
  return s === 'connected' || s === 'starting'
}

function hasTokens(p) {
  return Object.values(p.tokens || {}).some(t => t.token_source !== 'none')
}

async function loadProviders() {
  try {
    const response = await api.get('/api/providers')
    if (response.ok) {
      const data = await response.json()
      providers.value = data.providers || []
    }
  } catch (e) {
    console.error('Failed to load providers', e)
  } finally {
    loading.value = false
  }
}

async function startProvider(name) {
  try {
    const response = await api.post(`/api/providers/${name}/start`)
    if (response.ok) {
      message.success(`${providerLabels[name]} started`)
    } else {
      const data = await response.json()
      message.error(data.error || 'Failed to start')
    }
    await loadProviders()
  } catch (e) {
    message.error('Failed to start provider')
  }
}

async function stopProvider(name) {
  try {
    const response = await api.post(`/api/providers/${name}/stop`)
    if (response.ok) {
      message.success(`${providerLabels[name]} stopped`)
    } else {
      const data = await response.json()
      message.error(data.error || 'Failed to stop')
    }
    await loadProviders()
  } catch (e) {
    message.error('Failed to stop provider')
  }
}

async function restartProvider(name) {
  try {
    const response = await api.post(`/api/providers/${name}/restart`)
    if (response.ok) {
      message.success(`${providerLabels[name]} restarted`)
    } else {
      const data = await response.json()
      message.error(data.error || 'Failed to restart')
    }
    await loadProviders()
  } catch (e) {
    message.error('Failed to restart provider')
  }
}

function openEditModal(p) {
  editProvider.value = {
    name: p.name,
    enabled: p.enabled,
    allowed_users: [...(p.allowed_users || [])],
    allowed_chans: [...(p.allowed_chans || [])],
  }
  // Initialize token fields as empty (password fields)
  editTokens.value = {}
  showEditModal.value = true
}

const showChannels = computed(() => {
  return editProvider.value.name === 'discord' || editProvider.value.name === 'slack'
})

const editTokenKeys = computed(() => {
  switch (editProvider.value.name) {
    case 'discord': return ['token']
    case 'telegram': return ['token']
    case 'slack': return ['bot_token', 'app_token']
    default: return []
  }
})

function currentTokenHint(key) {
  const p = providers.value.find(p => p.name === editProvider.value.name)
  if (!p) return ''
  const info = p.tokens?.[key]
  return info?.token_hint || ''
}

async function saveProvider() {
  saving.value = true
  try {
    // Save config (enabled, allowlists)
    const configResp = await api.put(`/api/providers/${editProvider.value.name}`, {
      enabled: editProvider.value.enabled,
      allowed_users: editProvider.value.allowed_users,
      allowed_chans: editProvider.value.allowed_chans,
    })
    if (!configResp.ok) {
      const data = await configResp.json()
      message.error(data.error || 'Failed to save config')
      return
    }

    // Save tokens if any were entered
    const nonEmptyTokens = {}
    for (const [k, v] of Object.entries(editTokens.value)) {
      if (v) nonEmptyTokens[k] = v
    }
    if (Object.keys(nonEmptyTokens).length > 0) {
      const tokenResp = await api.put(`/api/providers/${editProvider.value.name}/tokens`, {
        tokens: nonEmptyTokens,
      })
      if (!tokenResp.ok) {
        const data = await tokenResp.json()
        message.error(data.error || 'Failed to save tokens')
        return
      }
    }

    message.success(`${providerLabels[editProvider.value.name]} updated`)
    showEditModal.value = false
    await loadProviders()
  } catch (e) {
    message.error('Failed to save provider')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadProviders()
  pollTimer = setInterval(loadProviders, 5000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="providers-page" style="max-width: 900px; margin: 0 auto">
    <div class="page-header">
      <h2 class="page-title">Chat Providers</h2>
    </div>

    <n-spin :show="loading && providers.length === 0">
      <n-space vertical :size="16">
        <n-card
          v-for="p in providers"
          :key="p.name"
          :title="providerLabels[p.name] || p.name"
          size="medium"
        >
          <template #header-extra>
            <n-space :size="8" align="center">
              <n-tag :type="statusType(providerState(p))" size="small" round>
                {{ statusLabel(providerState(p)) }}
              </n-tag>
            </n-space>
          </template>

          <n-space vertical :size="12">
            <!-- Token status -->
            <div style="display: flex; gap: 12px; flex-wrap: wrap">
              <div v-for="(info, key) in p.tokens" :key="key" style="display: flex; align-items: center; gap: 6px">
                <span style="color: var(--text-color-3); font-size: 13px">{{ tokenKeyLabels[key] || key }}:</span>
                <n-tag :type="tokenSourceType(info)" size="small">
                  {{ tokenSourceLabel(info) }}
                </n-tag>
                <span v-if="info.token_hint" style="color: var(--text-color-3); font-size: 12px; font-family: monospace">
                  {{ info.token_hint }}
                </span>
              </div>
            </div>

            <!-- Allowlists summary -->
            <div style="display: flex; gap: 16px; font-size: 13px; color: var(--text-color-3)">
              <span>Users: {{ (p.allowed_users?.length || 0) === 0 ? 'All' : p.allowed_users.length + ' allowed' }}</span>
              <span v-if="p.name !== 'telegram'">Channels: {{ (p.allowed_chans?.length || 0) === 0 ? 'All' : p.allowed_chans.length + ' allowed' }}</span>
              <span>Enabled: {{ p.enabled ? 'Yes' : 'No' }}</span>
            </div>

            <!-- Error message -->
            <n-alert v-if="p.status?.error" type="error" :show-icon="false" style="font-size: 13px">
              {{ p.status.error }}
            </n-alert>
          </n-space>

          <template #action>
            <n-space :size="8">
              <n-button
                v-if="!isRunning(p)"
                size="small"
                type="primary"
                :disabled="!hasTokens(p)"
                @click="startProvider(p.name)"
              >
                Start
              </n-button>
              <n-button
                v-if="isRunning(p)"
                size="small"
                type="warning"
                @click="stopProvider(p.name)"
              >
                Stop
              </n-button>
              <n-button
                v-if="isRunning(p)"
                size="small"
                secondary
                @click="restartProvider(p.name)"
              >
                Restart
              </n-button>
              <n-button
                size="small"
                secondary
                @click="openEditModal(p)"
              >
                Configure
              </n-button>
            </n-space>
          </template>
        </n-card>
      </n-space>
    </n-spin>

    <!-- Edit Modal -->
    <n-modal
      v-model:show="showEditModal"
      :title="`Configure ${providerLabels[editProvider.name] || editProvider.name}`"
      preset="card"
      style="width: 550px; border-radius: 16px"
    >
      <n-form label-placement="left" label-width="120">
        <n-form-item label="Enabled">
          <n-switch v-model:value="editProvider.enabled" />
        </n-form-item>

        <n-form-item
          v-for="key in editTokenKeys"
          :key="key"
          :label="tokenKeyLabels[key] || key"
        >
          <n-input
            v-model:value="editTokens[key]"
            type="password"
            show-password-on="click"
            :placeholder="currentTokenHint(key) ? `Current: ${currentTokenHint(key)} (leave empty to keep)` : 'Enter token'"
          />
        </n-form-item>

        <n-form-item label="Allowed Users">
          <n-dynamic-tags v-model:value="editProvider.allowed_users" />
        </n-form-item>

        <n-form-item v-if="showChannels" label="Allowed Channels">
          <n-dynamic-tags v-model:value="editProvider.allowed_chans" />
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showEditModal = false">Cancel</n-button>
          <n-button type="primary" :loading="saving" @click="saveProvider">Save</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>
