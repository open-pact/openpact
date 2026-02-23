<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useMessage, useDialog } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import Card from '@/components/shared/Card.vue'
import {
  NSpace,
  NButton,
  NText,
  NTag,
  NInput,
  NDescriptions,
  NDescriptionsItem,
  NSpin,
  NAlert,
  NIcon,
} from 'naive-ui'
import { ArrowBackOutline } from '@vicons/ionicons5'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const dialog = useDialog()
const api = useApi()

const script = ref(null)
const source = ref('')
const loading = ref(true)
const saving = ref(false)
const hasChanges = computed(() => script.value && source.value !== script.value.source)

async function loadScript() {
  loading.value = true
  try {
    const response = await api.get(`/api/scripts/${route.params.name}`)
    if (response.ok) {
      script.value = await response.json()
      source.value = script.value.source
    } else if (response.status === 404) {
      message.error('Script not found')
      router.push('/scripts')
    }
  } catch (e) {
    message.error('Failed to load script')
  } finally {
    loading.value = false
  }
}

async function saveScript() {
  saving.value = true
  try {
    const response = await api.put(`/api/scripts/${script.value.name}`, {
      source: source.value,
    })

    if (response.ok) {
      script.value = await response.json()
      script.value.source = source.value
      message.success('Script saved (status reset to pending)')
    } else {
      message.error('Failed to save script')
    }
  } catch (e) {
    message.error('Failed to save script')
  } finally {
    saving.value = false
  }
}

async function approveScript() {
  try {
    const response = await api.post(`/api/scripts/${script.value.name}/approve`)
    if (response.ok) {
      script.value = await response.json()
      message.success('Script approved')
    }
  } catch (e) {
    message.error('Failed to approve script')
  }
}

async function rejectScript() {
  dialog.warning({
    title: 'Reject Script',
    content: 'Are you sure you want to reject this script?',
    positiveText: 'Reject',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        const response = await api.post(`/api/scripts/${script.value.name}/reject`, {
          reason: 'Rejected via admin UI',
        })
        if (response.ok) {
          script.value = await response.json()
          message.success('Script rejected')
        }
      } catch (e) {
        message.error('Failed to reject script')
      }
    },
  })
}

const statusType = computed(() => {
  const map = { approved: 'success', pending: 'warning', rejected: 'error' }
  return map[script.value?.status] || 'default'
})

onMounted(loadScript)
</script>

<template>
  <div class="editor-page">
    <n-spin :show="loading">
      <template v-if="script">
        <div class="page-header">
          <n-space align="center" :size="12">
            <n-button quaternary circle size="small" @click="router.push('/scripts')">
              <template #icon>
                <n-icon><ArrowBackOutline /></n-icon>
              </template>
            </n-button>
            <h2 class="page-title">{{ script.name }}</h2>
            <n-tag :type="statusType" round size="small">{{ script.status }}</n-tag>
          </n-space>
          <n-space :size="8">
            <n-button
              v-if="script.status === 'pending'"
              type="success"
              secondary
              @click="approveScript"
              :disabled="hasChanges"
            >
              Approve
            </n-button>
            <n-button
              v-if="script.status === 'pending'"
              type="error"
              secondary
              @click="rejectScript"
              :disabled="hasChanges"
            >
              Reject
            </n-button>
            <n-button
              type="primary"
              @click="saveScript"
              :loading="saving"
              :disabled="!hasChanges"
            >
              Save
            </n-button>
          </n-space>
        </div>

        <n-alert
          v-if="hasChanges"
          type="warning"
          title="Unsaved Changes"
          style="margin-bottom: 16px"
        >
          You have unsaved changes. Saving will reset the script status to pending.
        </n-alert>

        <Card title="Script Info" style="margin-bottom: 16px">
          <n-descriptions :column="2">
            <n-descriptions-item label="Description">
              {{ script.description || 'No description' }}
            </n-descriptions-item>
            <n-descriptions-item label="Hash">
              <n-text code>{{ script.hash?.substring(0, 20) }}...</n-text>
            </n-descriptions-item>
            <n-descriptions-item label="Required Secrets">
              <n-space v-if="script.required_secrets?.length">
                <n-tag v-for="s in script.required_secrets" :key="s" size="small" round>{{ s }}</n-tag>
              </n-space>
              <n-text v-else depth="3">None</n-text>
            </n-descriptions-item>
            <n-descriptions-item label="Approved By" v-if="script.approved_by">
              {{ script.approved_by }}
            </n-descriptions-item>
          </n-descriptions>
        </Card>

        <Card title="Source Code">
          <n-input
            v-model:value="source"
            type="textarea"
            :rows="20"
            placeholder="Script source code"
            style="font-family: 'Fira Code', 'Monaco', 'Consolas', monospace; font-size: 13px"
          />
        </Card>
      </template>
    </n-spin>
  </div>
</template>
