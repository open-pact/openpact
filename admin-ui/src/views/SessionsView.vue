<script setup>
import { ref, onMounted, computed } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import { useAuth } from '@/composables/useAuth'
import {
  NDataTable,
  NSpace,
  NButton,
  NModal,
  NIcon,
  NText,
  NEmpty,
  NCard,
  NInput,
  NScrollbar,
  NSpin,
} from 'naive-ui'
import { AddOutline, ChatbubbleOutline, SendOutline } from '@vicons/ionicons5'
import { h } from 'vue'

const message = useMessage()
const dialog = useDialog()
const api = useApi()
const auth = useAuth()

const sessions = ref([])
const loading = ref(true)

// Chat modal
const showChatModal = ref(false)
const chatSessionId = ref('')
const chatSessionTitle = ref('')
const chatMessages = ref([])
const chatInput = ref('')
const chatLoading = ref(false)
const chatConnected = ref(false)
const messagesLoading = ref(false)
let ws = null

// Messages modal
const showMessagesModal = ref(false)
const messagesSessionId = ref('')
const messagesSessionTitle = ref('')
const messageHistory = ref([])

const columns = [
  {
    title: 'Session',
    key: 'id',
    render(row) {
      return h(NText, { code: true, style: 'font-size: 12px' }, { default: () => row.id.substring(0, 16) + '...' })
    },
  },
  {
    title: 'Title',
    key: 'title',
    render(row) {
      return row.title || h(NText, { depth: 3 }, { default: () => '(untitled)' })
    },
  },
  {
    title: 'Updated',
    key: 'time',
    width: 180,
    render(row) {
      if (!row.time?.updated) return '-'
      return new Date(row.time.updated).toLocaleDateString('en-US', {
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
    width: 320,
    render(row) {
      return h(NSpace, { size: 8 }, {
        default: () => [
          h(NButton, {
            size: 'small',
            secondary: true,
            onClick: () => openChat(row.id, row.title),
          }, {
            icon: () => h(NIcon, null, { default: () => h(ChatbubbleOutline) }),
            default: () => 'Chat',
          }),
          h(NButton, {
            size: 'small',
            type: 'error',
            quaternary: true,
            onClick: () => confirmDelete(row.id),
          }, { default: () => 'Delete' }),
        ],
      })
    },
  },
]

async function loadSessions() {
  loading.value = true
  try {
    const response = await api.get('/api/sessions')
    if (response.ok) {
      sessions.value = await response.json()
    }
  } catch (e) {
    message.error('Failed to load sessions')
  } finally {
    loading.value = false
  }
}

async function createSession() {
  try {
    const response = await api.post('/api/sessions', {})
    if (response.ok) {
      message.success('New session created')
      await loadSessions()
    } else {
      const data = await response.json()
      message.error(data.error || 'Failed to create session')
    }
  } catch (e) {
    message.error('Failed to create session')
  }
}

function confirmDelete(id) {
  dialog.error({
    title: 'Delete Session',
    content: 'Delete this session and all its messages? This cannot be undone.',
    positiveText: 'Delete',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        const response = await api.del(`/api/sessions/${id}`)
        if (response.ok) {
          message.success('Session deleted')
          await loadSessions()
        } else {
          message.error('Failed to delete session')
        }
      } catch (e) {
        message.error('Failed to delete session')
      }
    },
  })
}

async function openChat(sessionId, title) {
  chatSessionId.value = sessionId
  chatSessionTitle.value = title || '(untitled)'
  chatMessages.value = []
  chatInput.value = ''
  chatConnected.value = false
  chatLoading.value = false
  showChatModal.value = true

  // Load existing messages
  messagesLoading.value = true
  try {
    const response = await api.get(`/api/sessions/${sessionId}/messages?limit=50`)
    if (response.ok) {
      const msgs = await response.json()
      if (msgs && msgs.length) {
        chatMessages.value = msgs.map(m => ({
          role: m.role,
          content: m.parts?.map(p => p.text || '').join('') || '',
        })).filter(m => m.content)
      }
    }
  } catch (e) {
    // Non-critical, just start fresh
  } finally {
    messagesLoading.value = false
  }

  // Connect WebSocket
  connectChat(sessionId)
}

function connectChat(sessionId) {
  const token = auth.accessToken.value
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/sessions/${sessionId}/chat?token=${token}`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    chatConnected.value = true
  }

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    switch (msg.type) {
      case 'connected':
        chatConnected.value = true
        break
      case 'text':
        // Append to last assistant message or create new
        const last = chatMessages.value[chatMessages.value.length - 1]
        if (last && last.role === 'assistant' && last.streaming) {
          last.content += msg.content
        } else {
          chatMessages.value.push({ role: 'assistant', content: msg.content, streaming: true })
        }
        break
      case 'done':
        chatLoading.value = false
        // Mark last message as not streaming
        const lastMsg = chatMessages.value[chatMessages.value.length - 1]
        if (lastMsg) lastMsg.streaming = false
        break
      case 'error':
        message.error(msg.content || 'Chat error')
        chatLoading.value = false
        break
    }
  }

  ws.onclose = () => {
    chatConnected.value = false
  }

  ws.onerror = () => {
    chatConnected.value = false
    message.error('WebSocket connection failed')
  }
}

function sendChatMessage() {
  if (!chatInput.value.trim() || !ws || ws.readyState !== WebSocket.OPEN) return

  const content = chatInput.value.trim()
  chatMessages.value.push({ role: 'user', content })
  chatInput.value = ''
  chatLoading.value = true

  ws.send(JSON.stringify({ type: 'message', content }))
}

function closeChatModal() {
  showChatModal.value = false
  if (ws) {
    ws.close()
    ws = null
  }
}

onMounted(loadSessions)
</script>

<template>
  <div class="sessions-page" style="max-width: 1000px; margin: 0 auto">
    <div class="page-header">
      <h2 class="page-title">Sessions</h2>
      <n-button type="primary" @click="createSession">
        <template #icon>
          <n-icon><AddOutline /></n-icon>
        </template>
        New Session
      </n-button>
    </div>

    <n-data-table
      v-if="sessions.length > 0 || loading"
      :columns="columns"
      :data="sessions"
      :loading="loading"
      :bordered="false"
    />
    <n-empty
      v-else
      description="No sessions yet. Create a new session or send a message via Discord to start one."
      style="padding: 40px 0"
    />

    <!-- Chat Modal -->
    <n-modal
      :show="showChatModal"
      :title="'Chat — ' + chatSessionTitle"
      preset="card"
      style="width: 700px; max-height: 80vh; border-radius: 16px"
      @update:show="(v) => { if (!v) closeChatModal() }"
    >
      <div style="display: flex; flex-direction: column; height: 55vh">
        <n-scrollbar style="flex: 1; padding: 0 8px">
          <n-spin v-if="messagesLoading" size="small" style="display: block; margin: 20px auto" />
          <div v-for="(msg, i) in chatMessages" :key="i" style="margin-bottom: 12px">
            <n-text :depth="msg.role === 'user' ? 1 : 2" style="font-size: 12px; font-weight: 600; display: block; margin-bottom: 4px">
              {{ msg.role === 'user' ? 'You' : 'Assistant' }}
            </n-text>
            <div
              :style="{
                padding: '8px 12px',
                borderRadius: '8px',
                background: msg.role === 'user' ? 'var(--n-color-target)' : 'var(--card-color)',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
                fontSize: '14px',
              }"
            >{{ msg.content }}</div>
          </div>
          <div v-if="chatLoading && (!chatMessages.length || !chatMessages[chatMessages.length-1]?.streaming)" style="padding: 8px; color: var(--n-text-color-3)">
            Thinking...
          </div>
        </n-scrollbar>

        <div style="display: flex; gap: 8px; margin-top: 12px">
          <n-input
            v-model:value="chatInput"
            placeholder="Type a message..."
            :disabled="!chatConnected || chatLoading"
            @keyup.enter="sendChatMessage"
          />
          <n-button
            type="primary"
            :disabled="!chatInput.trim() || !chatConnected || chatLoading"
            @click="sendChatMessage"
          >
            <template #icon>
              <n-icon><SendOutline /></n-icon>
            </template>
          </n-button>
        </div>
      </div>
    </n-modal>
  </div>
</template>
