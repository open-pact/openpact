<script setup>
import { ref, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import {
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NSwitch,
  NButton,
  NSpin,
  NSpace,
} from 'naive-ui'
import Card from '@/components/shared/Card.vue'

const api = useApi()
const message = useMessage()

const loading = ref(true)
const saving = ref(false)

const settings = ref({
  logging: { level: 'info', json: false },
  rate_limit: { rate: 10, burst: 20 },
  server: { health_addr: ':8081' },
  starlark: { enabled: true, max_execution_ms: 30000, max_memory_mb: 128 },
})

const logLevels = [
  { label: 'debug', value: 'debug' },
  { label: 'info',  value: 'info'  },
  { label: 'warn',  value: 'warn'  },
  { label: 'error', value: 'error' },
]

async function load() {
  loading.value = true
  try {
    const response = await api.get('/api/config/advanced')
    if (response.ok) {
      settings.value = await response.json()
    } else {
      message.error('Failed to load settings')
    }
  } catch (e) {
    message.error('Failed to load settings: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const response = await api.put('/api/config/advanced', settings.value)
    if (response.ok) {
      message.success('Settings saved')
    } else {
      const data = await response.json().catch(() => ({}))
      message.error(data.error || 'Failed to save')
    }
  } catch (e) {
    message.error('Failed to save: ' + e.message)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-header">
      <h2 class="page-title">Advanced settings</h2>
    </div>

    <n-spin v-if="loading" size="small" style="display: block; margin: 48px auto" />
    <div v-else>
      <Card title="Logging">
        <div class="p-6">
          <n-form>
            <n-form-item label="Log level">
              <n-select v-model:value="settings.logging.level" :options="logLevels" />
            </n-form-item>
            <n-form-item label="Emit structured JSON logs">
              <n-switch v-model:value="settings.logging.json" />
            </n-form-item>
          </n-form>
        </div>
      </Card>

      <Card title="Rate limiter" style="margin-top: 12px">
        <div class="p-6">
          <n-form>
            <n-form-item label="Requests per second">
              <n-input-number v-model:value="settings.rate_limit.rate" :min="0" :precision="1" />
            </n-form-item>
            <n-form-item label="Burst">
              <n-input-number v-model:value="settings.rate_limit.burst" :min="1" />
            </n-form-item>
          </n-form>
        </div>
      </Card>

      <Card title="Server" style="margin-top: 12px">
        <div class="p-6">
          <n-form>
            <n-form-item label="Health bind address" :feedback="'e.g. :8081 or 127.0.0.1:8081'">
              <n-input v-model:value="settings.server.health_addr" />
            </n-form-item>
          </n-form>
        </div>
      </Card>

      <Card title="Starlark limits" style="margin-top: 12px">
        <div class="p-6">
          <n-form>
            <n-form-item label="Enabled">
              <n-switch v-model:value="settings.starlark.enabled" />
            </n-form-item>
            <n-form-item label="Max execution (ms)">
              <n-input-number v-model:value="settings.starlark.max_execution_ms" :min="100" />
            </n-form-item>
            <n-form-item label="Max memory (MB)">
              <n-input-number v-model:value="settings.starlark.max_memory_mb" :min="8" />
            </n-form-item>
          </n-form>
        </div>
      </Card>

      <n-space style="margin-top: 16px" justify="end">
        <n-button type="primary" :loading="saving" @click="save">Save settings</n-button>
      </n-space>
    </div>
  </div>
</template>
