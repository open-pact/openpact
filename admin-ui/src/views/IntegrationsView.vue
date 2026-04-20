<script setup>
import { ref, onMounted } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import {
  NForm,
  NFormItem,
  NInput,
  NSwitch,
  NButton,
  NSpin,
  NSpace,
  NDataTable,
  NModal,
  NEmpty,
  NIcon,
} from 'naive-ui'
import { AddOutline } from '@vicons/ionicons5'
import { h } from 'vue'
import Card from '@/components/shared/Card.vue'

const api = useApi()
const message = useMessage()
const dialog = useDialog()

const loading = ref(true)
const saving = ref(false)

const settings = ref({
  calendars: [],
  vault: { path: '', git_repo: '', auto_sync: false },
})

// Add-calendar modal
const showAddCalendar = ref(false)
const newCalendar = ref({ name: '', url: '' })

async function load() {
  loading.value = true
  try {
    const response = await api.get('/api/config/integrations')
    if (response.ok) {
      const data = await response.json()
      settings.value = {
        calendars: data.calendars || [],
        vault: data.vault || { path: '', git_repo: '', auto_sync: false },
      }
    } else {
      message.error('Failed to load integrations')
    }
  } catch (e) {
    message.error('Failed to load integrations: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const response = await api.put('/api/config/integrations', settings.value)
    if (response.ok) {
      message.success('Integrations saved')
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

function addCalendar() {
  const name = (newCalendar.value.name || '').trim()
  const url = (newCalendar.value.url || '').trim()
  if (!name || !url) {
    message.warning('Name and URL are required')
    return
  }
  settings.value.calendars.push({ name, url })
  newCalendar.value = { name: '', url: '' }
  showAddCalendar.value = false
}

function removeCalendar(index) {
  dialog.error({
    title: 'Remove calendar feed?',
    content: `Remove "${settings.value.calendars[index].name}"? Changes take effect after saving.`,
    positiveText: 'Remove',
    negativeText: 'Cancel',
    onPositiveClick: () => {
      settings.value.calendars.splice(index, 1)
    },
  })
}

const calendarColumns = [
  { title: 'Name', key: 'name' },
  { title: 'URL',  key: 'url', render: row => h('code', {}, row.url) },
  {
    title: 'Actions',
    key: 'actions',
    width: 120,
    render(_row, index) {
      return h(NButton, {
        size: 'small',
        type: 'error',
        quaternary: true,
        onClick: () => removeCalendar(index),
      }, { default: () => 'Remove' })
    },
  },
]

onMounted(load)
</script>

<template>
  <div>
    <div class="page-header">
      <h2 class="page-title">Integrations</h2>
    </div>

    <n-spin v-if="loading" size="small" style="display: block; margin: 48px auto" />
    <div v-else>
      <!-- Calendar feeds -->
      <Card title="Calendar feeds">
        <div class="p-4 flex items-center justify-between">
          <div class="text-sm text-gray-500">
            iCal feeds the <code>calendar_list</code> MCP tool will read from.
          </div>
          <n-button size="small" type="primary" @click="showAddCalendar = true">
            <template #icon><n-icon :component="AddOutline" /></template>
            Add feed
          </n-button>
        </div>
        <div class="px-2 pb-4">
          <n-data-table
            v-if="settings.calendars.length"
            :columns="calendarColumns"
            :data="settings.calendars"
            :bordered="false"
          />
          <n-empty v-else description="No calendar feeds configured" style="padding: 24px 0" />
        </div>
      </Card>

      <!-- Obsidian vault -->
      <Card title="Obsidian vault" style="margin-top: 12px">
        <div class="p-6">
          <n-form>
            <n-form-item label="Vault path (local directory)">
              <n-input v-model:value="settings.vault.path" placeholder="/home/user/vault" />
            </n-form-item>
            <n-form-item label="Git repository (optional)">
              <n-input v-model:value="settings.vault.git_repo" placeholder="git@github.com:user/notes.git" />
            </n-form-item>
            <n-form-item label="Auto pull/push on vault tool calls">
              <n-switch v-model:value="settings.vault.auto_sync" />
            </n-form-item>
          </n-form>
        </div>
      </Card>

      <n-space style="margin-top: 16px" justify="end">
        <n-button type="primary" :loading="saving" @click="save">Save integrations</n-button>
      </n-space>

      <n-modal
        v-model:show="showAddCalendar"
        preset="card"
        title="Add calendar feed"
        style="width: 520px; border-radius: 12px"
      >
        <n-form>
          <n-form-item label="Display name">
            <n-input v-model:value="newCalendar.name" placeholder="e.g. Work" />
          </n-form-item>
          <n-form-item label="iCal URL">
            <n-input v-model:value="newCalendar.url" placeholder="https://calendar.example/cal.ics" />
          </n-form-item>
        </n-form>
        <template #footer>
          <n-space justify="end">
            <n-button @click="showAddCalendar = false">Cancel</n-button>
            <n-button type="primary" @click="addCalendar">Add</n-button>
          </n-space>
        </template>
      </n-modal>
    </div>
  </div>
</template>
