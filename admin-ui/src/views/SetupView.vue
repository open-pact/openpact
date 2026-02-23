<script setup>
import { ref, computed, onMounted } from 'vue'
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
} from 'naive-ui'

const router = useRouter()
const message = useMessage()

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
    // Fallback for older browsers
    const common = [
      'UTC', 'America/New_York', 'America/Chicago', 'America/Denver',
      'America/Los_Angeles', 'Europe/London', 'Europe/Paris', 'Europe/Berlin',
      'Asia/Tokyo', 'Asia/Shanghai', 'Asia/Kolkata', 'Australia/Sydney',
      'Pacific/Auckland',
    ]
    return common.map(tz => ({ label: tz, value: tz }))
  }
})

// Password strength calculation
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
  return 'Personalize Your AI'
})

onMounted(async () => {
  try {
    const response = await fetch('/api/setup/status')
    const data = await response.json()
    if (data.setup_step === 'profile') {
      currentStep.value = 2
    } else if (data.setup_step === 'complete') {
      router.push('/login')
    }
  } catch (e) {
    console.error('Failed to check setup status:', e)
  }
})

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

    message.success('Account created!')
    currentStep.value = 2
  } catch (e) {
    message.error('Setup failed: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function handleProfileSubmit() {
  loading.value = true
  try {
    const response = await fetch('/api/setup/profile', {
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

    message.success('Setup complete! Please log in.')
    router.push('/login')
  } catch (e) {
    message.error('Profile setup failed: ' + e.message)
  } finally {
    loading.value = false
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
        <div class="step-dot" :class="{ active: currentStep === 2 }"></div>
      </div>

      <!-- Step 1: Account -->
      <n-card v-if="currentStep === 1" class="setup-card">
        <n-form @submit.prevent="handleAccountSubmit">
          <n-form-item label="Username">
            <n-input
              v-model:value="accountForm.username"
              placeholder="admin"
              :disabled="loading"
              size="large"
            />
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
      <n-card v-if="currentStep === 2" class="setup-card">
        <n-form @submit.prevent="handleProfileSubmit">
          <n-form-item label="Agent Name">
            <n-input
              v-model:value="profileForm.agentName"
              placeholder="e.g. Atlas, Nova, Sage..."
              :disabled="loading"
              size="large"
            />
          </n-form-item>

          <n-form-item label="Personality">
            <n-select
              v-model:value="profileForm.personality"
              :options="personalityOptions"
              placeholder="Choose a personality"
              :disabled="loading"
              size="large"
            />
          </n-form-item>

          <n-form-item label="Your Name">
            <n-input
              v-model:value="profileForm.userName"
              placeholder="What should the AI call you?"
              :disabled="loading"
              size="large"
            />
          </n-form-item>

          <n-form-item label="Timezone">
            <n-select
              v-model:value="profileForm.timezone"
              :options="timezoneOptions"
              filterable
              placeholder="Select your timezone"
              :disabled="loading"
              size="large"
            />
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
            Complete Setup
          </n-button>
        </n-form>
      </n-card>
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

.setup-card-wrapper {
  width: 100%;
  max-width: 380px;
}

.setup-branding {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 24px;
}

.setup-logo {
  width: 72px;
  height: 72px;
  border-radius: 16px;
  margin-bottom: 16px;
  filter: drop-shadow(0 4px 12px rgba(45, 215, 183, 0.15));
}

.setup-brand-title {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.5px;
  color: var(--primary-color);
}

.setup-brand-subtitle {
  margin: 4px 0 0;
  font-size: 14px;
  color: var(--primary-color);
  opacity: 0.5;
  letter-spacing: 0.5px;
}

.step-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  margin-bottom: 24px;
}

.step-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: rgba(128, 128, 128, 0.3);
  transition: background 0.3s ease;
}

.step-dot.active {
  background: #2dd7b7;
  box-shadow: 0 0 8px rgba(45, 215, 183, 0.4);
}

.step-dot.completed {
  background: #2387a7;
}

.step-line {
  width: 40px;
  height: 2px;
  background: rgba(128, 128, 128, 0.3);
  transition: background 0.3s ease;
}

.step-line.completed {
  background: #2387a7;
}

.setup-card {
  border-radius: 16px;
}
</style>
