<script setup>
import { ref, onMounted } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import {
  NDataTable,
  NSpace,
  NButton,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NIcon,
  NText,
  NEmpty,
  NTag,
  NAlert,
} from 'naive-ui'
import { AddOutline, EyeOutline, EyeOffOutline, CheckmarkCircle } from '@vicons/ionicons5'
import { h } from 'vue'
import Card from '@/components/shared/Card.vue'

const message = useMessage()
const dialog = useDialog()
const api = useApi()

const secrets = ref([])
const loading = ref(true)
const providers = ref([])
const providersLoading = ref(true)

// Add modal
const showAddModal = ref(false)
const newSecret = ref({ name: '', value: '' })
const showNewValue = ref(false)
const creating = ref(false)

// Edit modal
const showEditModal = ref(false)
const editSecret = ref({ name: '', value: '' })
const showEditValue = ref(false)
const updating = ref(false)

const columns = [
  {
    title: 'Name',
    key: 'name',
    render(row) {
      return h(NText, { code: true }, { default: () => row.name })
    },
  },
  {
    title: 'Created',
    key: 'created_at',
    width: 180,
    render(row) {
      return new Date(row.created_at).toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    },
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 200,
    render(row) {
      return h(NSpace, { size: 8 }, {
        default: () => [
          h(NButton, {
            size: 'small',
            secondary: true,
            onClick: () => openEditModal(row.name),
          }, { default: () => 'Edit Value' }),
          h(NButton, {
            size: 'small',
            type: 'error',
            quaternary: true,
            onClick: () => confirmDelete(row.name),
          }, { default: () => 'Delete' }),
        ],
      })
    },
  },
]

async function loadSecrets() {
  loading.value = true
  try {
    const response = await api.get('/api/secrets')
    if (response.ok) {
      const data = await response.json()
      secrets.value = data.secrets || []
    }
  } catch (e) {
    message.error('Failed to load secrets')
  } finally {
    loading.value = false
  }
}

async function createSecret() {
  const name = newSecret.value.name.trim().toUpperCase()
  const value = newSecret.value.value

  if (!name) {
    message.warning('Secret name is required')
    return
  }
  if (!/^[A-Z][A-Z0-9_]*$/.test(name)) {
    message.warning('Name must be uppercase letters, numbers, and underscores (starting with a letter)')
    return
  }
  if (!value) {
    message.warning('Secret value is required')
    return
  }

  creating.value = true
  try {
    const response = await api.post('/api/secrets', { name, value })
    if (response.ok) {
      message.success(`Secret ${name} created`)
      showAddModal.value = false
      newSecret.value = { name: '', value: '' }
      showNewValue.value = false
      await loadSecrets()
    } else {
      const data = await response.json()
      message.error(data.message || 'Failed to create secret')
    }
  } catch (e) {
    message.error('Failed to create secret')
  } finally {
    creating.value = false
  }
}

function openEditModal(name) {
  editSecret.value = { name, value: '' }
  showEditValue.value = false
  showEditModal.value = true
}

async function updateSecret() {
  if (!editSecret.value.value) {
    message.warning('Please enter a new value')
    return
  }

  updating.value = true
  try {
    const response = await api.put(
      `/api/secrets/${editSecret.value.name}`,
      { value: editSecret.value.value }
    )
    if (response.ok) {
      message.success(`Secret ${editSecret.value.name} updated`)
      showEditModal.value = false
      await loadSecrets()
    } else {
      const data = await response.json()
      message.error(data.message || 'Failed to update secret')
    }
  } catch (e) {
    message.error('Failed to update secret')
  } finally {
    updating.value = false
  }
}

function confirmDelete(name) {
  dialog.error({
    title: 'Delete Secret',
    content: `Delete secret ${name}? Scripts using this secret will fail.`,
    positiveText: 'Delete',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        const response = await api.del(`/api/secrets/${name}`)
        if (response.ok || response.status === 204) {
          message.success(`Secret ${name} deleted`)
          await loadSecrets()
        } else {
          message.error('Failed to delete secret')
        }
      } catch (e) {
        message.error('Failed to delete secret')
      }
    },
  })
}

async function loadProviders() {
  providersLoading.value = true
  try {
    const response = await api.get('/api/engine/providers')
    if (response.ok) {
      const data = await response.json()
      providers.value = data.providers || []
    }
  } catch (e) {
    // Non-fatal — Secrets view still works without provider info.
  } finally {
    providersLoading.value = false
  }
}

function providerLabel(name) {
  switch (name) {
    case 'openai':  return 'OpenAI'
    case 'gemini':  return 'Google Gemini'
    case 'copilot': return 'GitHub Copilot'
    case 'ollama':  return 'Ollama'
    default:        return name
  }
}

onMounted(() => {
  loadSecrets()
  loadProviders()
})
</script>

<template>
  <div class="secrets-page">
    <div class="page-header">
      <h2 class="page-title">Secrets</h2>
      <n-button type="primary" @click="showAddModal = true">
        <template #icon>
          <n-icon><AddOutline /></n-icon>
        </template>
        Add Secret
      </n-button>
    </div>

    <!-- LLM providers (read-only, sourced from stackllm auth store) -->
    <Card title="Authenticated LLM providers" style="margin-bottom: 12px">
      <n-alert type="info" style="margin: 8px 12px 4px">
        Provider credentials (OpenAI / Gemini / Copilot / Ollama) are managed on the
        <a href="/engine">Engine</a> page. They don't appear in the Starlark secret store below.
      </n-alert>
      <div v-if="providersLoading" class="p-4">Loading…</div>
      <div v-else class="p-4">
        <n-space>
          <n-tag
            v-for="p in providers"
            :key="p.name"
            :type="p.authenticated ? 'success' : 'default'"
          >
            <template v-if="p.authenticated" #icon>
              <n-icon :component="CheckmarkCircle" />
            </template>
            {{ providerLabel(p.name) }} — {{ p.authenticated ? 'authenticated' : 'not signed in' }}
          </n-tag>
        </n-space>
      </div>
    </Card>

    <n-data-table
      v-if="secrets.length > 0 || loading"
      :columns="columns"
      :data="secrets"
      :loading="loading"
      :bordered="false"
    />
    <n-empty
      v-else
      description="No Starlark secrets configured. Add secrets to make them available to scripts via secrets.get(&quot;NAME&quot;)."
      style="padding: 40px 0"
    />

    <!-- Add Secret Modal -->
    <n-modal
      v-model:show="showAddModal"
      title="Add Secret"
      preset="card"
      style="width: 500px; border-radius: 16px"
    >
      <n-form>
        <n-form-item label="Name">
          <n-input
            v-model:value="newSecret.name"
            placeholder="MY_API_KEY"
            @input="newSecret.name = $event.toUpperCase()"
          />
        </n-form-item>
        <n-form-item label="Value">
          <n-input
            v-model:value="newSecret.value"
            :type="showNewValue ? 'text' : 'password'"
            placeholder="Enter secret value"
          >
            <template #suffix>
              <n-icon
                :component="showNewValue ? EyeOffOutline : EyeOutline"
                style="cursor: pointer"
                @click="showNewValue = !showNewValue"
              />
            </template>
          </n-input>
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showAddModal = false">Cancel</n-button>
          <n-button type="primary" :loading="creating" @click="createSecret">Save</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- Edit Secret Modal -->
    <n-modal
      v-model:show="showEditModal"
      title="Update Secret"
      preset="card"
      style="width: 500px; border-radius: 16px"
    >
      <n-form>
        <n-form-item label="Name">
          <n-input :value="editSecret.name" disabled />
        </n-form-item>
        <n-form-item label="New Value">
          <n-input
            v-model:value="editSecret.value"
            :type="showEditValue ? 'text' : 'password'"
            placeholder="Enter new value (current value is hidden)"
          >
            <template #suffix>
              <n-icon
                :component="showEditValue ? EyeOffOutline : EyeOutline"
                style="cursor: pointer"
                @click="showEditValue = !showEditValue"
              />
            </template>
          </n-input>
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showEditModal = false">Cancel</n-button>
          <n-button type="primary" :loading="updating" @click="updateSecret">Save</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>
