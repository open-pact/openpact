<script setup>
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuth } from '@/composables/useAuth'
import {
  NCard,
  NForm,
  NFormItem,
  NInput,
  NButton,
} from 'naive-ui'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const auth = useAuth()
const appVersion = ref('')

onMounted(async () => {
  try {
    const res = await fetch('/api/version')
    if (res.ok) {
      const data = await res.json()
      appVersion.value = data.version
    }
  } catch {
    // ignore - version display is non-critical
  }
})

const form = ref({
  username: '',
  password: '',
})

const loading = ref(false)

async function handleSubmit() {
  if (!form.value.username || !form.value.password) {
    message.error('Please enter username and password')
    return
  }

  loading.value = true
  try {
    await auth.login(form.value.username, form.value.password)
    message.success('Login successful')

    // Redirect to original destination or dashboard
    const redirect = route.query.redirect || '/'
    router.push(redirect)
  } catch (e) {
    message.error(e.message || 'Login failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-container">
    <div class="login-card-wrapper">
      <div class="login-branding">
        <img src="/assets/logo-full.svg" alt="OpenPact" class="login-logo" />
        <h1 class="login-brand-title">OpenPact</h1>
        <p class="login-brand-subtitle">Admin Console</p>
        <p v-if="appVersion" class="login-version">v{{ appVersion }}</p>
      </div>

      <n-card class="login-card">
        <n-form @submit.prevent="handleSubmit">
          <n-form-item label="Username">
            <n-input
              v-model:value="form.username"
              placeholder="Username"
              :disabled="loading"
              autofocus
              size="large"
            />
          </n-form-item>

          <n-form-item label="Password">
            <n-input
              v-model:value="form.password"
              type="password"
              show-password-on="click"
              placeholder="Password"
              :disabled="loading"
              @keyup.enter="handleSubmit"
              size="large"
            />
          </n-form-item>

          <n-button
            type="primary"
            attr-type="submit"
            :loading="loading"
            block
            size="large"
            style="margin-top: 8px"
          >
            Sign In
          </n-button>
        </n-form>
      </n-card>
    </div>
  </div>
</template>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: var(--background);
}

.login-card-wrapper {
  width: 100%;
  max-width: 380px;
}

.login-branding {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 32px;
}

.login-logo {
  width: 72px;
  height: 72px;
  border-radius: 16px;
  margin-bottom: 16px;
  filter: drop-shadow(0 4px 12px rgba(45, 215, 183, 0.15));
}

.login-brand-title {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.5px;
  color: var(--primary-color);
}

.login-brand-subtitle {
  margin: 4px 0 0;
  font-size: 14px;
  color: var(--primary-color);
  opacity: 0.5;
  letter-spacing: 0.5px;
}

.login-version {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--primary-color);
  opacity: 0.35;
  font-family: monospace;
}

.login-card {
  border-radius: 16px;
}
</style>
