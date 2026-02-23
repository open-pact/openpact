<script setup>
import { ref, onMounted, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import AuthTerminal from '@/components/AuthTerminal.vue'
import Card from '@/components/shared/Card.vue'
import {
  NSpace,
  NButton,
  NText,
  NTag,
  NSpin,
  NIcon,
  useMessage,
} from 'naive-ui'
import { ShieldCheckmarkOutline, KeyOutline } from '@vicons/ionicons5'

const api = useApi()
const message = useMessage()

const loading = ref(true)
const authStatus = ref(null)
const showTerminal = ref(false)
const signingOut = ref(false)

const statusColor = computed(() => {
  if (!authStatus.value) return 'default'
  return authStatus.value.authenticated ? 'success' : 'error'
})

const statusText = computed(() => {
  if (!authStatus.value) return 'Loading...'
  if (showTerminal.value) return 'Authenticating...'
  if (!authStatus.value.authenticated) return 'Not authenticated'
  const method = authStatus.value.method
  switch (method) {
    case 'oauth': return 'Authenticated (OAuth)'
    case 'env': return 'Authenticated (Environment Variable)'
    default: return 'Authenticated'
  }
})

onMounted(async () => {
  await fetchStatus()
})

async function fetchStatus() {
  loading.value = true
  try {
    const response = await api.get('/api/engine/auth')
    if (response.ok) {
      authStatus.value = await response.json()
    }
  } catch (e) {
    console.error('Failed to fetch auth status:', e)
  } finally {
    loading.value = false
  }
}

function startSignIn() {
  showTerminal.value = true
}

function cancelSignIn() {
  showTerminal.value = false
}

async function handleAuthSuccess() {
  showTerminal.value = false
  message.success('Authentication successful!')
  await fetchStatus()
}

function handleAuthError(error) {
  message.error(error || 'Authentication failed')
}

async function signOut() {
  signingOut.value = true
  try {
    const response = await api.del('/api/engine/auth')
    if (response.ok) {
      authStatus.value = await response.json()
      message.info('Credentials cleared')
    }
  } catch (e) {
    message.error('Failed to clear credentials')
  } finally {
    signingOut.value = false
  }
}
</script>

<template>
  <div class="auth-page" style="max-width: 700px; margin: 0 auto">
    <h2 class="page-title" style="margin-bottom: 20px">Engine Authentication</h2>

    <n-spin :show="loading">
      <!-- Status card -->
      <Card v-if="authStatus">
        <div class="status-row">
          <div class="status-icon" :class="authStatus.authenticated ? 'status-icon--ok' : 'status-icon--error'">
            <n-icon size="24"><ShieldCheckmarkOutline /></n-icon>
          </div>
          <div class="status-info">
            <n-space vertical :size="8">
              <n-space align="center" :size="12">
                <n-text strong>Engine:</n-text>
                <n-text>OpenCode</n-text>
              </n-space>
              <n-space align="center" :size="12">
                <n-text strong>Status:</n-text>
                <n-tag :type="statusColor" round size="small">{{ statusText }}</n-tag>
              </n-space>
              <n-space v-if="authStatus.authenticated && authStatus.expires_at" align="center" :size="12">
                <n-text strong>Expires:</n-text>
                <n-text>{{ new Date(authStatus.expires_at).toLocaleDateString() }}</n-text>
              </n-space>
            </n-space>
          </div>
        </div>
      </Card>
    </n-spin>

    <!-- Authenticated: add provider / sign out options -->
    <Card v-if="authStatus?.authenticated && !showTerminal" style="margin-top: 16px">
      <n-space vertical :size="12">
        <n-text depth="2">
          You can sign in to additional providers using the terminal.
        </n-text>
        <n-space>
          <n-button type="primary" @click="startSignIn">
            <template #icon>
              <n-icon><KeyOutline /></n-icon>
            </template>
            Add Provider
          </n-button>
          <n-button type="error" secondary :loading="signingOut" @click="signOut">
            Sign Out
          </n-button>
        </n-space>
      </n-space>
    </Card>

    <!-- Not authenticated: sign-in options -->
    <template v-if="authStatus && !authStatus.authenticated && !showTerminal">
      <Card style="margin-top: 16px">
        <n-space vertical :size="12">
          <n-text depth="2">
            Click "Start Sign In" to open a terminal and complete the OAuth flow for OpenCode.
          </n-text>
          <n-button type="primary" @click="startSignIn">
            <template #icon>
              <n-icon><KeyOutline /></n-icon>
            </template>
            Start Sign In
          </n-button>
        </n-space>
      </Card>
    </template>

    <!-- Terminal -->
    <Card v-if="showTerminal" style="margin-top: 16px">
      <template #title>
        <n-space align="center" justify="space-between" style="width: 100%">
          <n-text>Terminal</n-text>
          <n-button size="small" @click="cancelSignIn">Cancel</n-button>
        </n-space>
      </template>
      <AuthTerminal
        engine="opencode"
        @success="handleAuthSuccess"
        @error="handleAuthError"
      />
    </Card>
  </div>
</template>

<style scoped>
.status-row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.status-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.status-icon--ok {
  background: rgba(99, 226, 183, 0.12);
  color: #63e2b7;
}

.status-icon--error {
  background: rgba(224, 98, 98, 0.12);
  color: #e06262;
}
</style>
