<script setup>
import { ref, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import {
  NForm,
  NFormItem,
  NSelect,
  NSpin,
} from 'naive-ui'
import Card from '@/components/shared/Card.vue'

const message = useMessage()
const api = useApi()

// Models
const modelOptions = ref([])
const selectedModel = ref(null)
const modelsLoading = ref(true)

async function loadModels() {
  modelsLoading.value = true
  try {
    const response = await api.get('/api/models')
    if (response.ok) {
      const data = await response.json()
      const models = data.models || []
      const defaultModel = data.default || {}

      modelOptions.value = models
        .map(m => ({
          label: `${m.provider_id}/${m.model_id}`,
          value: `${m.provider_id}/${m.model_id}`,
        }))
        .sort((a, b) => a.label.localeCompare(b.label))

      if (defaultModel.provider && defaultModel.model) {
        selectedModel.value = `${defaultModel.provider}/${defaultModel.model}`
      }
    }
  } catch (e) {
    message.error('Failed to load models')
  } finally {
    modelsLoading.value = false
  }
}

async function onModelChange(value) {
  if (!value) return
  const [provider, ...modelParts] = value.split('/')
  const model = modelParts.join('/')
  try {
    const response = await api.put('/api/models/default', { provider, model })
    if (response.ok) {
      message.success(`Default model set to ${model}`)
    } else {
      const data = await response.json()
      message.error(data.error || 'Failed to set default model')
    }
  } catch (e) {
    message.error('Failed to set default model')
  }
}

onMounted(loadModels)
</script>

<template>
  <div>
    <div class="page-header">
      <h2 class="page-title">Settings</h2>
    </div>

    <Card title="AI Model">
      <n-spin v-if="modelsLoading" size="small" style="display: block; margin: 24px auto" />
      <div v-else class="p-8 w-full md:w-3/4 mx-auto">
        <n-form size="large">
          <n-form-item label="Default Model" path="model">
            <n-select
              v-model:value="selectedModel"
              :options="modelOptions"
              placeholder="Select default model"
              filterable
              @update:value="onModelChange"
            />
          </n-form-item>
        </n-form>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          The default model is used for all new AI sessions. Changing this does not affect sessions that are already running.
        </p>
      </div>
    </Card>
  </div>
</template>
