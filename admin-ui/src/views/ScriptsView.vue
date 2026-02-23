<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
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
  NIcon,
} from 'naive-ui'
import { AddOutline } from '@vicons/ionicons5'
import { h } from 'vue'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const api = useApi()

const scripts = ref([])
const loading = ref(true)
const showCreateModal = ref(false)

const newScript = ref({
  name: '',
  source: '# New Script\n# @description: \n# @secrets: \n\ndef main():\n    pass\n',
})

const columns = [
  {
    title: 'Name',
    key: 'name',
    render(row) {
      return h(NButton, {
        text: true,
        type: 'primary',
        onClick: () => router.push(`/scripts/${row.name}`),
      }, { default: () => row.name })
    },
  },
  {
    title: 'Status',
    key: 'status',
    width: 120,
    render(row) {
      const typeMap = {
        approved: 'success',
        pending: 'warning',
        rejected: 'error',
      }
      return h(NTag, {
        type: typeMap[row.status] || 'default',
        round: true,
        size: 'small',
      }, { default: () => row.status })
    },
  },
  {
    title: 'Description',
    key: 'description',
    ellipsis: true,
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 280,
    render(row) {
      const buttons = []

      if (row.status === 'pending') {
        buttons.push(
          h(NButton, {
            size: 'small',
            type: 'success',
            secondary: true,
            onClick: () => approveScript(row.name),
          }, { default: () => 'Approve' }),
          h(NButton, {
            size: 'small',
            type: 'error',
            secondary: true,
            onClick: () => rejectScript(row.name),
          }, { default: () => 'Reject' })
        )
      }

      buttons.push(
        h(NButton, {
          size: 'small',
          secondary: true,
          onClick: () => router.push(`/scripts/${row.name}`),
        }, { default: () => 'Edit' }),
        h(NButton, {
          size: 'small',
          type: 'error',
          quaternary: true,
          onClick: () => confirmDelete(row.name),
        }, { default: () => 'Delete' })
      )

      return h(NSpace, { size: 8 }, { default: () => buttons })
    },
  },
]

async function loadScripts() {
  loading.value = true
  try {
    const response = await api.get('/api/scripts')
    if (response.ok) {
      const data = await response.json()
      scripts.value = data.scripts || []
    }
  } catch (e) {
    message.error('Failed to load scripts')
  } finally {
    loading.value = false
  }
}

async function createScript() {
  if (!newScript.value.name) {
    message.error('Script name is required')
    return
  }

  try {
    const response = await api.post('/api/scripts', {
      name: newScript.value.name,
      source: newScript.value.source,
    })

    if (response.ok) {
      message.success('Script created')
      showCreateModal.value = false
      newScript.value.name = ''
      await loadScripts()
    } else {
      const data = await response.json()
      message.error(data.message || 'Failed to create script')
    }
  } catch (e) {
    message.error('Failed to create script')
  }
}

async function approveScript(name) {
  try {
    const response = await api.post(`/api/scripts/${name}/approve`)
    if (response.ok) {
      message.success(`Script ${name} approved`)
      await loadScripts()
    } else {
      message.error('Failed to approve script')
    }
  } catch (e) {
    message.error('Failed to approve script')
  }
}

async function rejectScript(name) {
  dialog.warning({
    title: 'Reject Script',
    content: `Are you sure you want to reject ${name}?`,
    positiveText: 'Reject',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        const response = await api.post(`/api/scripts/${name}/reject`, {
          reason: 'Rejected via admin UI',
        })
        if (response.ok) {
          message.success(`Script ${name} rejected`)
          await loadScripts()
        }
      } catch (e) {
        message.error('Failed to reject script')
      }
    },
  })
}

function confirmDelete(name) {
  dialog.error({
    title: 'Delete Script',
    content: `Are you sure you want to delete ${name}? This cannot be undone.`,
    positiveText: 'Delete',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        const response = await api.del(`/api/scripts/${name}`)
        if (response.ok) {
          message.success(`Script ${name} deleted`)
          await loadScripts()
        }
      } catch (e) {
        message.error('Failed to delete script')
      }
    },
  })
}

onMounted(loadScripts)
</script>

<template>
  <div class="scripts-page" style="max-width: 1100px; margin: 0 auto">
    <div class="page-header">
      <h2 class="page-title">Scripts</h2>
      <n-button type="primary" @click="showCreateModal = true">
        <template #icon>
          <n-icon><AddOutline /></n-icon>
        </template>
        New Script
      </n-button>
    </div>

    <n-data-table
      :columns="columns"
      :data="scripts"
      :loading="loading"
      :bordered="false"
    />

    <!-- Create Script Modal -->
    <n-modal
      v-model:show="showCreateModal"
      title="Create New Script"
      preset="card"
      style="width: 600px; border-radius: 16px"
    >
      <n-form>
        <n-form-item label="Script Name">
          <n-input
            v-model:value="newScript.name"
            placeholder="my_script.star"
          />
        </n-form-item>
        <n-form-item label="Source">
          <n-input
            v-model:value="newScript.source"
            type="textarea"
            :rows="10"
            placeholder="Script source code"
            style="font-family: 'Fira Code', 'Monaco', 'Consolas', monospace; font-size: 13px"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreateModal = false">Cancel</n-button>
          <n-button type="primary" @click="createScript">Create</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>
