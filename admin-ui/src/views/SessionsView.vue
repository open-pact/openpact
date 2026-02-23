<script setup>
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useMessage, useDialog } from 'naive-ui'
import { useApi } from '@/composables/useApi'
import { useAuth } from '@/composables/useAuth'
import {
  NLayout,
  NLayoutSider,
  NLayoutContent,
  NList,
  NListItem,
  NButton,
  NIcon,
  NEmpty,
  NInput,
  NScrollbar,
  NSpin,
} from 'naive-ui'
import {
  AddOutline,
  TrashOutline,
  MenuOutline,
  ChevronForwardOutline,
} from '@vicons/ionicons5'
import { Send28Filled as SendIcon } from '@vicons/fluent'

const message = useMessage()
const dialog = useDialog()
const api = useApi()
const auth = useAuth()

// Sessions
const sessions = ref([])
const sessionsLoading = ref(true)
const selectedSessionId = ref(null)

// Chat
const chatMessages = ref([])
const chatInput = ref('')
const chatLoading = ref(false)
const chatConnected = ref(false)
const messagesLoading = ref(false)
const messagesScrollRef = ref(null)
let ws = null

// Mobile sidebar
const sidebarCollapsed = ref(window.innerWidth < 768)
const isMobile = ref(window.innerWidth < 768)

function handleResize() {
  isMobile.value = window.innerWidth < 768
  if (!isMobile.value) sidebarCollapsed.value = false
}

const selectedSession = computed(() =>
  sessions.value.find(s => s.id === selectedSessionId.value)
)
const selectedSessionTitle = computed(() =>
  selectedSession.value?.title || '(untitled)'
)

// --- Message parsing ---
function parseMessageParts(parts) {
  if (!parts || !parts.length) return { textContent: '', blocks: [] }

  const textParts = []
  const blocks = []
  const thinkingParts = []

  for (const part of parts) {
    if (part.type === 'reasoning' || part.type === 'thinking') {
      thinkingParts.push(part.text || '')
    } else if (part.type === 'text') {
      textParts.push(part.text || '')
    } else if (part.type === 'tool') {
      blocks.push({
        kind: 'tool',
        label: `Tool: ${part.tool?.name || part.tool || 'unknown'}`,
        content: formatToolContent(part),
        expanded: false,
      })
    } else if (part.type === 'file') {
      blocks.push({
        kind: 'file',
        label: `File: ${part.source || part.url || 'attachment'}`,
        content: `URL: ${part.url || '(none)'}\nMIME: ${part.mime || 'unknown'}`,
        expanded: false,
      })
    } else if (part.type === 'snapshot') {
      blocks.push({
        kind: 'snapshot',
        label: 'Snapshot',
        content: typeof part.snapshot === 'string' ? part.snapshot : JSON.stringify(part.snapshot, null, 2),
        expanded: false,
      })
    }
  }

  if (thinkingParts.length) {
    blocks.unshift({
      kind: 'thinking',
      label: 'Thinking',
      content: thinkingParts.join('\n').trim(),
      expanded: false,
    })
  }

  return {
    textContent: textParts.join('').trim(),
    blocks,
  }
}

function formatToolContent(part) {
  const lines = []
  const tool = part.tool
  if (typeof tool === 'object' && tool) {
    if (tool.name) lines.push(`Name: ${tool.name}`)
    if (tool.input) {
      lines.push(`Input: ${typeof tool.input === 'string' ? tool.input : JSON.stringify(tool.input, null, 2)}`)
    }
  } else if (tool) {
    lines.push(`Tool: ${tool}`)
  }
  const state = part.state
  if (state) {
    if (typeof state === 'object') {
      if (state.status) lines.push(`Status: ${state.status}`)
      if (state.output) lines.push(`Output: ${typeof state.output === 'string' ? state.output : JSON.stringify(state.output, null, 2)}`)
      if (state.error) lines.push(`Error: ${state.error}`)
    } else {
      lines.push(`State: ${state}`)
    }
  }
  return lines.join('\n') || JSON.stringify(part, null, 2)
}

function thinkingBlocks(msg) {
  return msg.blocks.filter(b => b.kind === 'thinking')
}

function nonThinkingBlocks(msg) {
  return msg.blocks.filter(b => b.kind !== 'thinking')
}

function toggleBlock(block) {
  block.expanded = !block.expanded
}

// --- Sessions ---
async function loadSessions() {
  sessionsLoading.value = true
  try {
    const response = await api.get('/api/sessions')
    if (response.ok) {
      sessions.value = await response.json()
    }
  } catch (e) {
    message.error('Failed to load sessions')
  } finally {
    sessionsLoading.value = false
  }
}

async function createSession() {
  try {
    const response = await api.post('/api/sessions', {})
    if (response.ok) {
      const newSession = await response.json()
      await loadSessions()
      selectSession(newSession.id)
    } else {
      const data = await response.json()
      message.error(data.error || 'Failed to create session')
    }
  } catch (e) {
    message.error('Failed to create session')
  }
}

function confirmDelete(id, e) {
  if (e) e.stopPropagation()
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
          if (selectedSessionId.value === id) {
            selectedSessionId.value = null
            chatMessages.value = []
            disconnectChat()
          }
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

// --- Chat ---
async function selectSession(sessionId) {
  if (selectedSessionId.value === sessionId) return
  selectedSessionId.value = sessionId
  chatMessages.value = []
  chatInput.value = ''
  chatConnected.value = false
  chatLoading.value = false

  if (isMobile.value) sidebarCollapsed.value = true

  disconnectChat()

  messagesLoading.value = true
  try {
    const response = await api.get(`/api/sessions/${sessionId}/messages?limit=50`)
    if (response.ok) {
      const msgs = await response.json()
      if (msgs && msgs.length) {
        chatMessages.value = msgs.map(m => {
          const parsed = parseMessageParts(m.parts)
          return { role: m.role, textContent: parsed.textContent, blocks: parsed.blocks }
        }).filter(m => m.textContent || m.blocks.length)
      }
    }
  } catch (e) {
    // Non-critical
  } finally {
    messagesLoading.value = false
    await nextTick()
    scrollToBottom()
  }

  connectChat(sessionId)
}

function connectChat(sessionId) {
  const token = auth.accessToken.value
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/sessions/${sessionId}/chat?token=${token}`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => { chatConnected.value = true }

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    switch (msg.type) {
      case 'connected':
        chatConnected.value = true
        break
      case 'text': {
        const last = chatMessages.value[chatMessages.value.length - 1]
        if (last && last.role === 'assistant' && last.streaming) {
          last.textContent += msg.content
        } else {
          chatMessages.value.push({
            role: 'assistant',
            textContent: msg.content,
            blocks: [],
            streaming: true,
          })
        }
        nextTick(() => scrollToBottom())
        break
      }
      case 'done':
        chatLoading.value = false
        {
          const lastMsg = chatMessages.value[chatMessages.value.length - 1]
          if (lastMsg) lastMsg.streaming = false
        }
        loadSessions()
        break
      case 'error':
        message.error(msg.content || 'Chat error')
        chatLoading.value = false
        break
    }
  }

  ws.onclose = () => { chatConnected.value = false }
  ws.onerror = () => { chatConnected.value = false }
}

function disconnectChat() {
  if (ws) { ws.close(); ws = null }
}

function sendMessage() {
  if (!chatInput.value.trim() || !ws || ws.readyState !== WebSocket.OPEN) return
  const content = chatInput.value.trim()
  chatMessages.value.push({ role: 'user', textContent: content, blocks: [] })
  chatInput.value = ''
  chatLoading.value = true
  ws.send(JSON.stringify({ type: 'message', content }))
  nextTick(() => scrollToBottom())
}

function handleInputKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    sendMessage()
  }
}

function scrollToBottom() {
  const el = messagesScrollRef.value
  if (el && el.scrollTo) el.scrollTo({ top: 999999, behavior: 'smooth' })
}

function formatTime(session) {
  if (!session.time?.updated) return ''
  const d = new Date(session.time.updated)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

function isLastInGroup(index) {
  const msgs = chatMessages.value
  if (index === msgs.length - 1) return true
  return msgs[index].role !== msgs[index + 1].role
}

onMounted(() => {
  loadSessions()
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  disconnectChat()
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <!-- Matches YummyAdmin ChatApp.vue structure -->
  <NLayout has-sider sider-placement="left" class="chat-layout">
    <NLayoutSider
      bordered
      collapse-mode="width"
      :collapsed-width="0"
      :width="300"
      :collapsed="sidebarCollapsed"
      @collapse="sidebarCollapsed = true"
      @expand="sidebarCollapsed = false"
    >
      <!-- Sidebar header with New button -->
      <div class="p-3 flex items-center justify-between">
        <span class="text-base font-semibold">Sessions</span>
        <n-button size="small" type="primary" @click="createSession">
          <template #icon><n-icon><AddOutline /></n-icon></template>
          New
        </n-button>
      </div>
      <!-- Sidebar list — matches theme .chat-sidebar -->
      <div class="chat-sidebar">
        <NScrollbar>
          <n-spin v-if="sessionsLoading" size="small" style="display: block; margin: 24px auto" />
          <!-- Matches ChatList.vue: NList hoverable clickable -->
          <NList v-else-if="sessions.length" hoverable clickable class="pe-1">
            <NListItem
              v-for="session in sessions"
              :key="session.id"
              :class="{ selected: session.id === selectedSessionId }"
              @click="selectSession(session.id)"
            >
              <div class="flex items-center justify-between w-full">
                <div class="flex flex-col min-w-0 flex-1">
                  <span class="text-sm dark:text-white overflow-hidden text-ellipsis whitespace-nowrap">
                    {{ session.title || '(untitled)' }}
                  </span>
                  <span class="text-xs text-gray-500 overflow-hidden text-ellipsis whitespace-nowrap">
                    {{ session.id.substring(0, 8) }}
                    <template v-if="formatTime(session)"> &middot; {{ formatTime(session) }}</template>
                  </span>
                </div>
                <n-button
                  quaternary
                  size="tiny"
                  class="session-delete-btn"
                  @click="confirmDelete(session.id, $event)"
                >
                  <template #icon><n-icon size="14"><TrashOutline /></n-icon></template>
                </n-button>
              </div>
            </NListItem>
          </NList>
          <n-empty v-else description="No sessions" style="padding: 24px 0" />
        </NScrollbar>
      </div>
    </NLayoutSider>

    <NLayoutContent>
      <!-- Empty state when nothing selected -->
      <div v-if="!selectedSessionId" class="flex flex-col items-center justify-center h-full">
        <n-empty description="Select a session or create a new one">
          <template #extra>
            <n-button type="primary" @click="createSession">
              <template #icon><n-icon><AddOutline /></n-icon></template>
              New Session
            </n-button>
          </template>
        </n-empty>
        <n-button v-if="isMobile" quaternary style="margin-top: 12px" @click="sidebarCollapsed = !sidebarCollapsed">
          Show Sessions
        </n-button>
      </div>

      <!-- Messages box — matches ChatMessages.vue structure -->
      <div v-else class="messages-box flex flex-col items-stretch justify-stretch">
        <!-- Header — bg-gray-100 dark:bg-gray-700 like theme -->
        <header class="send-message p-3 bg-gray-100 dark:bg-gray-700 flex justify-between">
          <div class="flex items-center">
            <n-button
              v-if="isMobile"
              quaternary
              circle
              size="small"
              class="me-2"
              @click="sidebarCollapsed = !sidebarCollapsed"
            >
              <template #icon><n-icon><MenuOutline /></n-icon></template>
            </n-button>
            <div class="flex flex-col">
              <span class="text-gray-800 dark:text-gray-200">{{ selectedSessionTitle }}</span>
              <span class="text-xs text-gray-500 dark:text-gray-400 font-mono">{{ selectedSessionId.substring(0, 16) }}</span>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <span class="status-dot" :class="{ connected: chatConnected }"></span>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ chatConnected ? 'Connected' : 'Disconnected' }}</span>
          </div>
        </header>

        <!-- Chat content area — flex-1 with inner scrollbar -->
        <section class="flex flex-col flex-1 min-h-0">
          <div class="flex-1 items-end flex-col justify-end min-h-0">
            <n-scrollbar ref="messagesScrollRef">
              <div class="flex flex-col justify-end items-start gap-2 py-4 px-7 flex-1">
                <n-spin v-if="messagesLoading" size="small" style="display: block; margin: 24px auto" />
                <template v-else>
                  <template v-for="(msg, i) in chatMessages" :key="i">
                    <!-- Thinking blocks — full width, outside bubbles -->
                    <div
                      v-for="(block, bi) in thinkingBlocks(msg)"
                      :key="`${i}-t-${bi}`"
                      class="thinking-row"
                    >
                      <div
                        class="detail-block thinking"
                        :class="{ expanded: block.expanded }"
                        @click="toggleBlock(block)"
                      >
                        <div class="detail-header">
                          <n-icon size="14" class="detail-chevron"><ChevronForwardOutline /></n-icon>
                          <span class="detail-label">{{ block.label }}</span>
                          <span v-if="!block.expanded" class="detail-preview">
                            {{ block.content.substring(0, 80) }}{{ block.content.length > 80 ? '...' : '' }}
                          </span>
                        </div>
                        <div v-if="block.expanded" class="detail-body">{{ block.content }}</div>
                      </div>
                    </div>

                    <!-- Message bubble — matches MessageItem.vue -->
                    <div
                      v-if="msg.textContent || nonThinkingBlocks(msg).length"
                      class="chat-message flex flex-col gap-2 p-3 bg-gray-100 dark:bg-gray-700"
                      :class="{
                        'self-message': msg.role === 'user',
                        'last': isLastInGroup(i),
                      }"
                    >
                      <!-- Tool/file/snapshot blocks inside bubble -->
                      <div
                        v-for="(block, bi) in nonThinkingBlocks(msg)"
                        :key="bi"
                        class="detail-block"
                        :class="[block.kind, { expanded: block.expanded }]"
                        @click.stop="toggleBlock(block)"
                      >
                        <div class="detail-header">
                          <n-icon size="14" class="detail-chevron"><ChevronForwardOutline /></n-icon>
                          <span class="detail-label">{{ block.label }}</span>
                          <span v-if="!block.expanded" class="detail-preview">
                            {{ block.content.substring(0, 80) }}{{ block.content.length > 80 ? '...' : '' }}
                          </span>
                        </div>
                        <div v-if="block.expanded" class="detail-body">{{ block.content }}</div>
                      </div>
                      <!-- Text -->
                      <span v-if="msg.textContent" style="white-space: pre-wrap; word-break: break-word;">{{ msg.textContent }}</span>
                    </div>
                  </template>

                  <!-- Typing indicator -->
                  <div
                    v-if="chatLoading && (!chatMessages.length || !chatMessages[chatMessages.length - 1]?.streaming)"
                    class="chat-message flex flex-col gap-2 p-3 bg-gray-100 dark:bg-gray-700"
                  >
                    <div class="typing-indicator"><span></span><span></span><span></span></div>
                  </div>
                </template>
              </div>
            </n-scrollbar>
          </div>

          <!-- Input bar — matches theme send-message section -->
          <section class="send-message p-4 bg-gray-100 dark:bg-gray-700 flex items-center">
            <input
              v-model="chatInput"
              placeholder="Write Message"
              class="message-input flex-1"
              :disabled="!chatConnected || chatLoading"
              @keydown="handleInputKeydown"
            >
            <n-button
              :disabled="!chatInput.trim() || !chatConnected || chatLoading"
              text
              type="primary"
              @click="sendMessage"
            >
              <template #icon>
                <n-icon size="1.4rem"><SendIcon /></n-icon>
              </template>
            </n-button>
          </section>
        </section>
      </div>
    </NLayoutContent>
  </NLayout>
</template>

<style lang="scss" scoped>
// =============================================
// Layout — matches ChatApp.vue from YummyAdmin theme
// =============================================
// The route uses meta.fullScreen which removes padding/max-width
// from AppLayout (same as theme's layout: wide).

.n-layout {
  padding: 0;
}

// Exact values from theme ChatApp.vue
.chat-layout {
  height: calc(100vh - 30px);
}

.chat-sidebar {
  height: calc(100vh - 150px);
}

// Sidebar list item styling — matches ChatList.vue
.session-delete-btn {
  opacity: 0;
  transition: opacity 0.15s;
}

:deep(.n-list-item:hover) .session-delete-btn {
  opacity: 1;
}

// Match ChatList.vue .selected
.selected {
  font-weight: bold;
  background: var(--n-merged-color-hover);
  position: relative;

  &::before {
    content: '';
    z-index: 999;
    position: absolute;
    left: -10px;
    top: 2px;
    height: 18px;
    width: 3px;
    border-radius: 3px;
    background: var(--primary-color);
  }
}

// =============================================
// Messages box — matches ChatMessages.vue exactly
// =============================================
// Exact value from theme ChatMessages.vue
.messages-box {
  height: calc(100% - 51px);

  .message-input {
    background: transparent;
    border: none;

    &:focus {
      outline: none;
    }
  }
}

// Status dot
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #aaa;
  transition: background 0.2s;

  &.connected {
    background: #4ade80;
  }
}

// =============================================
// Message bubbles — matches MessageItem.vue exactly
// =============================================
// NOTE: No max-width on .chat-message — the theme doesn't set one.
.dark {
  .chat-message {
    --current-color: #374151;
    --self-background: #424e64;
  }
}

.chat-message {
  --current-color: #f3f4f6;
  --self-background: #e0f7fa;

  span a {
    color: rgb(0, 183, 255) !important;
    text-decoration: underline;
  }
  border-radius: 1rem;
  position: relative;

  &.last {
    border-bottom-left-radius: 0;

    &::before {
      content: "";
      position: absolute;
      bottom: 0;
      left: -9px;
      width: 20px;
      height: 20px;
      display: block;
      background-color: var(--current-color);
    }

    &::after {
      content: "";
      position: absolute;
      bottom: 1px;
      left: -29px;
      width: 29px;
      height: 28px;
      display: block;
      border-radius: 50%;
      background-color: var(--second-background);
    }
  }
}

.self-message {
  background-color: var(--self-background);
  align-self: flex-end;

  &.last {
    border-bottom-right-radius: 0;
    border-bottom-left-radius: 0.7rem;

    &::before {
      content: "";
      position: absolute;
      bottom: 0;
      right: -9px;
      left: auto;
      width: 20px;
      height: 20px;
      display: block;
      background-color: var(--self-background);
    }

    &::after {
      content: "";
      position: absolute;
      bottom: 1px;
      right: -29px;
      left: auto;
      width: 29px;
      height: 28px;
      display: block;
      border-radius: 50%;
      background-color: var(--second-background);
    }
  }
}

// ---- Thinking row — full width, between bubbles ----
.thinking-row {
  width: 100%;
  align-self: stretch;
}

// ---- Collapsible detail blocks ----
.detail-block {
  cursor: pointer;
  background: var(--chat-thinking-bg);
  border: 1px solid var(--chat-thinking-border);
  border-radius: 8px;
  padding: 6px 10px;
  font-size: 13px;
  transition: background 0.15s;
  border-left: 3px solid var(--chat-thinking-border);

  &:hover {
    filter: brightness(0.97);
  }

  &.thinking { border-left-color: #a78bfa; }
  &.tool { border-left-color: #f59e0b; }
  &.file { border-left-color: #3b82f6; }
  &.snapshot { border-left-color: #10b981; }
}

.detail-header {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.detail-chevron {
  transition: transform 0.2s;
  flex-shrink: 0;

  .detail-block.expanded & {
    transform: rotate(90deg);
  }
}

.detail-label {
  font-weight: 500;
  color: #6b7280;
  flex-shrink: 0;

  :global(.dark) & {
    color: #d1d5db;
  }
}

.detail-preview {
  color: #9ca3af;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-left: 4px;

  :global(.dark) & {
    color: #9ca3af;
  }
}

.detail-body {
  margin-top: 8px;
  color: #6b7280;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 300px;
  overflow-y: auto;
  font-family: monospace;
  font-size: 12px;

  :global(.dark) & {
    color: #d1d5db;
  }
}

.detail-block.thinking .detail-body {
  font-family: inherit;
  font-size: 13px;
}

// Inside-bubble detail blocks need margin
.chat-message .detail-block {
  margin-bottom: 4px;
}

// ---- Typing indicator ----
.typing-indicator {
  display: flex;
  gap: 4px;
  padding: 4px 0;

  span {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    opacity: 0.4;
    animation: typing 1.4s infinite;

    &:nth-child(2) { animation-delay: 0.2s; }
    &:nth-child(3) { animation-delay: 0.4s; }
  }
}

@keyframes typing {
  0%, 60%, 100% {
    opacity: 0.4;
    transform: translateY(0);
  }
  30% {
    opacity: 1;
    transform: translateY(-4px);
  }
}
</style>
