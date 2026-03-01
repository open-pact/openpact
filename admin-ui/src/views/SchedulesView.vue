<script setup>
import { ref, computed, onMounted } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import {
  NDataTable,
  NTag,
  NSpace,
  NButton,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NSelect,
  NSwitch,
  NIcon,
  NEmpty,
} from 'naive-ui'
import { AddOutline } from '@vicons/ionicons5'
import { h } from 'vue'

const message = useMessage()
const dialog = useDialog()
const api = useApi()

const schedules = ref([])
const loading = ref(true)
const showModal = ref(false)
const isEditing = ref(false)
const saving = ref(false)

const defaultForm = () => ({
  id: '',
  name: '',
  cron_expr: '',
  type: 'script',
  enabled: true,
  run_once: false,
  script_name: '',
  prompt: '',
  output_provider: '',
  output_channel: '',
})

const form = ref(defaultForm())

const typeOptions = [
  { label: 'Script', value: 'script' },
  { label: 'Agent', value: 'agent' },
]

const columns = [
  {
    title: 'Name',
    key: 'name',
    render(row) {
      if (row.run_once) {
        return h(NSpace, { size: 6, align: 'center' }, {
          default: () => [
            row.name,
            h(NTag, { size: 'tiny', round: true, bordered: false }, { default: () => 'Once' }),
          ],
        })
      }
      return row.name
    },
  },
  {
    title: 'Type',
    key: 'type',
    width: 100,
    render(row) {
      return h(NTag, {
        type: row.type === 'script' ? 'info' : 'warning',
        round: true,
        size: 'small',
      }, { default: () => row.type })
    },
  },
  {
    title: 'Schedule',
    key: 'cron_expr',
    width: 150,
    render(row) {
      return h('code', {}, row.cron_expr)
    },
  },
  {
    title: 'Enabled',
    key: 'enabled',
    width: 100,
    render(row) {
      return h(NTag, {
        type: row.enabled ? 'success' : 'default',
        round: true,
        size: 'small',
      }, { default: () => row.enabled ? 'Active' : 'Disabled' })
    },
  },
  {
    title: 'Last Run',
    key: 'last_run_at',
    width: 180,
    render(row) {
      if (!row.last_run_at) return '—'
      const statusType = row.last_run_status === 'success' ? 'success' : row.last_run_status === 'error' ? 'error' : 'default'
      return h(NSpace, { size: 4, align: 'center' }, {
        default: () => [
          h(NTag, { type: statusType, size: 'tiny', round: true }, { default: () => row.last_run_status }),
          new Date(row.last_run_at).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }),
        ],
      })
    },
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 300,
    render(row) {
      return h(NSpace, { size: 8 }, {
        default: () => [
          h(NButton, {
            size: 'small',
            secondary: true,
            onClick: () => openEditModal(row),
          }, { default: () => 'Edit' }),
          h(NButton, {
            size: 'small',
            type: row.enabled ? 'default' : 'success',
            secondary: true,
            onClick: () => toggleEnabled(row),
          }, { default: () => row.enabled ? 'Disable' : 'Enable' }),
          h(NButton, {
            size: 'small',
            type: 'primary',
            secondary: true,
            onClick: () => runNow(row),
          }, { default: () => 'Run Now' }),
          h(NButton, {
            size: 'small',
            type: 'error',
            quaternary: true,
            onClick: () => confirmDelete(row),
          }, { default: () => 'Delete' }),
        ],
      })
    },
  },
]

async function loadSchedules() {
  loading.value = true
  try {
    const response = await api.get('/api/schedules')
    if (response.ok) {
      const data = await response.json()
      schedules.value = data.schedules || []
    }
  } catch (e) {
    message.error('Failed to load schedules')
  } finally {
    loading.value = false
  }
}

function openCreateModal() {
  form.value = defaultForm()
  isEditing.value = false
  showModal.value = true
}

function openEditModal(row) {
  form.value = {
    id: row.id,
    name: row.name,
    cron_expr: row.cron_expr,
    type: row.type,
    enabled: row.enabled,
    run_once: row.run_once || false,
    script_name: row.script_name || '',
    prompt: row.prompt || '',
    output_provider: row.output_target?.provider || '',
    output_channel: row.output_target?.channel_id || '',
  }
  isEditing.value = true
  showModal.value = true
}

async function saveSchedule() {
  if (!form.value.name) {
    message.warning('Name is required')
    return
  }
  if (!form.value.cron_expr) {
    message.warning('Cron expression is required')
    return
  }
  if (form.value.type === 'script' && !form.value.script_name) {
    message.warning('Script name is required')
    return
  }
  if (form.value.type === 'agent' && !form.value.prompt) {
    message.warning('Prompt is required')
    return
  }

  saving.value = true
  try {
    const body = {
      name: form.value.name,
      cron_expr: form.value.cron_expr,
      type: form.value.type,
      enabled: form.value.enabled,
      run_once: form.value.run_once,
    }

    if (form.value.type === 'script') {
      body.script_name = form.value.script_name
    } else {
      body.prompt = form.value.prompt
    }

    if (form.value.output_provider && form.value.output_channel) {
      body.output_target = {
        provider: form.value.output_provider,
        channel_id: form.value.output_channel,
      }
    }

    let response
    if (isEditing.value) {
      response = await api.put(`/api/schedules/${form.value.id}`, body)
    } else {
      response = await api.post('/api/schedules', body)
    }

    if (response.ok) {
      message.success(isEditing.value ? 'Schedule updated' : 'Schedule created')
      showModal.value = false
      await loadSchedules()
    } else {
      const data = await response.json()
      message.error(data.message || 'Failed to save schedule')
    }
  } catch (e) {
    message.error('Failed to save schedule')
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(row) {
  try {
    const action = row.enabled ? 'disable' : 'enable'
    const response = await api.post(`/api/schedules/${row.id}/${action}`)
    if (response.ok) {
      message.success(`Schedule ${action}d`)
      await loadSchedules()
    } else {
      message.error(`Failed to ${action} schedule`)
    }
  } catch (e) {
    message.error('Failed to update schedule')
  }
}

async function runNow(row) {
  try {
    const response = await api.post(`/api/schedules/${row.id}/run`)
    if (response.ok) {
      message.success(`Schedule "${row.name}" triggered`)
    } else {
      const data = await response.json()
      message.error(data.message || 'Failed to trigger schedule')
    }
  } catch (e) {
    message.error('Failed to trigger schedule')
  }
}

function confirmDelete(row) {
  dialog.error({
    title: 'Delete Schedule',
    content: `Are you sure you want to delete "${row.name}"? This cannot be undone.`,
    positiveText: 'Delete',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        const response = await api.del(`/api/schedules/${row.id}`)
        if (response.ok || response.status === 204) {
          message.success(`Schedule "${row.name}" deleted`)
          await loadSchedules()
        } else {
          message.error('Failed to delete schedule')
        }
      } catch (e) {
        message.error('Failed to delete schedule')
      }
    },
  })
}

onMounted(loadSchedules)
</script>

<template>
  <div class="schedules-page">
    <div class="page-header">
      <h2 class="page-title">Schedules</h2>
      <n-button type="primary" @click="openCreateModal">
        <template #icon>
          <n-icon><AddOutline /></n-icon>
        </template>
        New Schedule
      </n-button>
    </div>

    <n-data-table
      v-if="schedules.length > 0 || loading"
      :columns="columns"
      :data="schedules"
      :loading="loading"
      :bordered="false"
    />
    <n-empty
      v-else
      description="No schedules configured. Create a schedule to run scripts or AI agent sessions on a cron schedule."
      style="padding: 40px 0"
    />

    <!-- Create/Edit Modal -->
    <n-modal
      v-model:show="showModal"
      :title="isEditing ? 'Edit Schedule' : 'Create Schedule'"
      preset="card"
      style="width: 600px; border-radius: 16px"
    >
      <n-form>
        <n-form-item label="Name">
          <n-input
            v-model:value="form.name"
            placeholder="Daily report"
          />
        </n-form-item>
        <n-form-item label="Type">
          <n-select
            v-model:value="form.type"
            :options="typeOptions"
          />
        </n-form-item>
        <n-form-item label="Cron Expression">
          <n-input
            v-model:value="form.cron_expr"
            placeholder="*/5 * * * * (every 5 minutes)"
          />
        </n-form-item>
        <n-form-item v-if="form.type === 'script'" label="Script Name">
          <n-input
            v-model:value="form.script_name"
            placeholder="my_script.star"
          />
        </n-form-item>
        <n-form-item v-if="form.type === 'agent'" label="Prompt">
          <n-input
            v-model:value="form.prompt"
            type="textarea"
            :rows="4"
            placeholder="What should the AI agent do?"
          />
        </n-form-item>
        <n-form-item label="Enabled">
          <n-switch v-model:value="form.enabled" />
        </n-form-item>
        <n-form-item label="Run Once">
          <n-switch v-model:value="form.run_once" />
        </n-form-item>
        <n-form-item label="Output Provider (optional)">
          <n-input
            v-model:value="form.output_provider"
            placeholder="discord"
          />
        </n-form-item>
        <n-form-item label="Output Channel (optional)">
          <n-input
            v-model:value="form.output_channel"
            placeholder="channel:123456"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showModal = false">Cancel</n-button>
          <n-button type="primary" :loading="saving" @click="saveSchedule">
            {{ isEditing ? 'Save' : 'Create' }}
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>
