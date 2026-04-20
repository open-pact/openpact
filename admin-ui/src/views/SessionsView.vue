<script setup>
// SessionsView — admin chat over the stackllm web.ManagedHandler SSE
// endpoints. Visual structure mirrors the YummyAdmin theme's Chat/*
// components (ChatApp, ChatMessages, ChatList, MessageItem) verbatim —
// every height/calc value is a copy from the theme source, per the
// CLAUDE.md rule. Only the transport changed: we POST to /api/engine/chat
// and parse Server-Sent Events instead of driving a WebSocket.

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
import MarkdownContent from '@/components/MarkdownContent.vue'

const message = useMessage()
const dialog = useDialog()
const api = useApi()
const auth = useAuth()

const sessions = ref([])
const sessionsLoading = ref(true)
const sessionsLoadingMore = ref(false)
const sessionsTotal = ref(0)
const sessionsOffset = ref(0)
const SESSIONS_PAGE_SIZE = 50
const selectedSessionId = ref(null)
const messagesLoading = ref(false)
const messagesScrollRef = ref(null)

const chatMessages = ref([])
const chatInput = ref('')
const chatLoading = ref(false)

// The active SSE stream. We abort it on unmount or when the user sends a
// second message while one is in flight.
let streamAbort = null

const sidebarCollapsed = ref(window.innerWidth < 768)
const isMobile = ref(window.innerWidth < 768)

function handleResize() {
  isMobile.value = window.innerWidth < 768
  if (!isMobile.value) sidebarCollapsed.value = false
}

const selectedSessionTitle = computed(() => {
  if (!selectedSessionId.value) return ''
  const row = sessions.value.find(s => s.id === selectedSessionId.value)
  if (row && row.name) return row.name
  return selectedSessionId.value.substring(0, 16)
})

const canLoadMore = computed(
  () => sessions.value.length < sessionsTotal.value,
)

// ---- Session list (server-backed, paginated) ----
// We load in pages of SESSIONS_PAGE_SIZE from GET /api/engine/sessions.
// Includes bot sessions (Discord/Slack/Telegram/scheduler) alongside the
// admin-chat sessions — the orchestrator names bot sessions on creation
// so the list is browsable.
async function loadSessionsPage(offset) {
  const url = `/api/engine/sessions?limit=${SESSIONS_PAGE_SIZE}&offset=${offset}`
  const response = await api.get(url)
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.message || `HTTP ${response.status}`)
  }
  return response.json()
}

async function loadInitialSessions() {
  sessionsLoading.value = true
  try {
    const page = await loadSessionsPage(0)
    sessions.value = page.sessions || []
    sessionsTotal.value = page.total || 0
    sessionsOffset.value = sessions.value.length
  } catch (e) {
    message.error('Failed to load sessions: ' + e.message)
    sessions.value = []
    sessionsTotal.value = 0
    sessionsOffset.value = 0
  } finally {
    sessionsLoading.value = false
  }
}

async function loadMoreSessions() {
  if (sessionsLoadingMore.value || !canLoadMore.value) return
  sessionsLoadingMore.value = true
  try {
    const page = await loadSessionsPage(sessionsOffset.value)
    const fresh = (page.sessions || []).filter(
      s => !sessions.value.some(existing => existing.id === s.id),
    )
    sessions.value = [...sessions.value, ...fresh]
    sessionsTotal.value = page.total || sessionsTotal.value
    sessionsOffset.value = sessions.value.length
  } catch (e) {
    message.error('Failed to load more: ' + e.message)
  } finally {
    sessionsLoadingMore.value = false
  }
}

// refreshSession fetches metadata for a single id and moves it to the top
// of the list, or inserts it if not already present. Called after a chat
// reply lands (a session may have been created server-side or its
// updated_at bumped).
async function refreshSession(id) {
  if (!id) return
  try {
    const response = await api.get(`/api/engine/sessions/${id}`)
    if (!response.ok) return
    const data = await response.json()
    const summary = {
      id: data.id,
      name: data.name || '',
      model: data.model || '',
      updated: data.updated,
      created: data.created,
    }
    const without = sessions.value.filter(s => s.id !== id)
    sessions.value = [summary, ...without]
    // If this id wasn't in the current page, total gained a row.
    if (!sessions.value.slice(1).some(s => s.id === id)) {
      sessionsTotal.value = Math.max(sessionsTotal.value, sessions.value.length)
    }
    sessionsOffset.value = sessions.value.length
  } catch {
    // Non-fatal — list stays usable.
  }
}

function dropSession(id) {
  sessions.value = sessions.value.filter(s => s.id !== id)
  sessionsTotal.value = Math.max(0, sessionsTotal.value - 1)
  sessionsOffset.value = sessions.value.length
}

function newSession() {
  // A fresh session is created lazily by the server on the first /chat
  // call when session_id is empty. We just clear the pane.
  selectedSessionId.value = null
  chatMessages.value = []
  chatInput.value = ''
  if (isMobile.value) sidebarCollapsed.value = true
  nextTick(() => scrollToBottom())
}

async function confirmDelete(id, e) {
  if (e) e.stopPropagation()
  dialog.error({
    title: 'Delete session',
    content: 'Delete this session and its messages? This cannot be undone.',
    positiveText: 'Delete',
    negativeText: 'Cancel',
    onPositiveClick: async () => {
      try {
        const response = await api.del(`/api/engine/sessions/${id}`)
        if (response.ok || response.status === 204) {
          dropSession(id)
          if (selectedSessionId.value === id) {
            selectedSessionId.value = null
            chatMessages.value = []
          }
          message.success('Session deleted')
        } else if (response.status === 404) {
          // Server doesn't know this id — drop it from the sidebar anyway
          // so the list stays honest.
          dropSession(id)
        } else {
          const data = await response.json().catch(() => ({}))
          message.error('Delete failed: ' + (data.message || data.error || `HTTP ${response.status}`))
        }
      } catch (e) {
        message.error('Delete failed: ' + e.message)
      }
    },
  })
}

// ---- Load messages when selecting a session ----
async function selectSession(id) {
  if (selectedSessionId.value === id) return
  abortStream()
  selectedSessionId.value = id
  chatMessages.value = []
  chatInput.value = ''
  if (isMobile.value) sidebarCollapsed.value = true

  messagesLoading.value = true
  try {
    const response = await api.get(`/api/engine/sessions/${id}`)
    if (response.ok) {
      const data = await response.json()
      chatMessages.value = (data.messages || [])
        .filter(m => m.role !== 'system')
        .map(m => ({
          role: m.role,
          orderedParts: blocksToParts(m.blocks || []),
        }))
        .filter(m => m.orderedParts.length > 0)
    }
  } catch (e) {
    // Non-fatal — empty state is OK.
  } finally {
    messagesLoading.value = false
    await nextTick()
    scrollToBottom()
  }
}

function blocksToParts(blocks) {
  const out = []
  for (const b of blocks) {
    switch (b.type) {
      case 'text':
        out.push({ kind: 'text', content: b.text || '' })
        break
      case 'thinking':
        out.push({ kind: 'thinking', label: 'Thinking', content: b.text || '', expanded: false })
        break
      case 'tool_use':
        out.push({
          kind: 'tool',
          label: `Tool: ${b.tool_name || 'unknown'}`,
          content: `Input: ${b.tool_args_json || '{}'}`,
          expanded: false,
        })
        break
      case 'tool_result':
        out.push({
          kind: 'tool',
          label: 'Tool result',
          content: b.text || '',
          expanded: false,
        })
        break
    }
  }
  return out
}

function togglePart(p) { p.expanded = !p.expanded }

function isLastBubblePart(msg, i) {
  for (let j = msg.orderedParts.length - 1; j >= 0; j--) {
    const k = msg.orderedParts[j].kind
    if (k === 'text') return j === i
  }
  return false
}

function isLastInGroup(i) {
  const msgs = chatMessages.value
  if (i === msgs.length - 1) return true
  return msgs[i].role !== msgs[i + 1].role
}

// ---- SSE chat ----
async function sendMessage() {
  const content = chatInput.value.trim()
  if (!content || chatLoading.value) return
  chatInput.value = ''
  chatLoading.value = true

  chatMessages.value.push({ role: 'user', orderedParts: [{ kind: 'text', content }] })
  const assistant = {
    role: 'assistant',
    orderedParts: [],
    streaming: true,
    _textIndex: -1,
    _thinkingIndex: -1,
  }
  chatMessages.value.push(assistant)
  await nextTick()
  scrollToBottom()

  try {
    await streamChat(content, assistant)
  } catch (e) {
    message.error('Chat failed: ' + e.message)
  } finally {
    assistant.streaming = false
    chatLoading.value = false
  }
}

async function streamChat(content, assistant) {
  abortStream()
  streamAbort = new AbortController()

  const body = {
    session_id: selectedSessionId.value || '',
    message: {
      role: 'user',
      blocks: [{ type: 'text', text: content }],
    },
  }

  const resp = await fetch('/api/engine/chat', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...auth.getAuthHeader(),
    },
    credentials: 'include',
    body: JSON.stringify(body),
    signal: streamAbort.signal,
  })

  if (!resp.ok || !resp.body) {
    const data = await resp.json().catch(() => ({}))
    throw new Error(data.error || `HTTP ${resp.status}`)
  }

  const reader = resp.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    // Split buffer into complete SSE events (double-newline terminated).
    let idx
    while ((idx = buffer.indexOf('\n\n')) !== -1) {
      const raw = buffer.slice(0, idx)
      buffer = buffer.slice(idx + 2)
      const ev = parseSSEEvent(raw)
      if (ev) handleEvent(ev, assistant)
    }
  }
}

function parseSSEEvent(raw) {
  const lines = raw.split('\n')
  let event = 'message'
  let data = ''
  for (const line of lines) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) data += line.slice(5).trim()
  }
  if (!data) return null
  try {
    return { event, data: JSON.parse(data) }
  } catch {
    return null
  }
}

function handleEvent(ev, assistant) {
  switch (ev.event) {
    case 'block_start':
      // Pre-create a slot for the incoming block so deltas land in order.
      if (ev.data.block_type === 'text') {
        assistant._textIndex = assistant.orderedParts.length
        assistant.orderedParts.push({ kind: 'text', content: '' })
      } else if (ev.data.block_type === 'thinking') {
        assistant._thinkingIndex = assistant.orderedParts.length
        assistant.orderedParts.push({ kind: 'thinking', label: 'Thinking', content: '', expanded: false })
      }
      break
    case 'block_delta':
      if (ev.data.block_type === 'text' && assistant._textIndex >= 0) {
        assistant.orderedParts[assistant._textIndex].content += (ev.data.delta || '')
      } else if (ev.data.block_type === 'thinking' && assistant._thinkingIndex >= 0) {
        assistant.orderedParts[assistant._thinkingIndex].content += (ev.data.delta || '')
      }
      nextTick(() => scrollToBottom())
      break
    case 'block_end':
      if (ev.data.block && ev.data.block_type === 'tool_use') {
        assistant.orderedParts.push({
          kind: 'tool',
          label: `Tool: ${ev.data.block.tool_name || 'unknown'}`,
          content: `Args: ${ev.data.block.tool_args || '{}'}`,
          expanded: false,
        })
      }
      // Reset the streaming slot indexes so the next block_start picks a
      // fresh one (some turns emit multiple text blocks).
      if (ev.data.block_type === 'text') assistant._textIndex = -1
      if (ev.data.block_type === 'thinking') assistant._thinkingIndex = -1
      break
    case 'done':
      if (ev.data.session_id) {
        selectedSessionId.value = ev.data.session_id
        // Pull the fresh metadata row so the sidebar shows the new session
        // (or bumps the existing one to the top of the list).
        refreshSession(ev.data.session_id)
      }
      break
    case 'error':
      message.error(ev.data.message || 'Stream error')
      break
  }
}

function abortStream() {
  if (streamAbort) {
    streamAbort.abort()
    streamAbort = null
  }
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

function shortId(id) { return id ? id.substring(0, 8) : '' }

onMounted(() => {
  loadInitialSessions()
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  abortStream()
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
      <div class="p-3 flex items-center justify-between">
        <span class="text-base font-semibold">
          Sessions
          <span v-if="sessionsTotal" class="text-xs text-gray-500 font-normal ms-1">
            {{ sessions.length }} / {{ sessionsTotal }}
          </span>
        </span>
        <n-button size="small" type="primary" @click="newSession">
          <template #icon><n-icon><AddOutline /></n-icon></template>
          New
        </n-button>
      </div>
      <div class="chat-sidebar">
        <NScrollbar>
          <n-spin v-if="sessionsLoading" size="small" style="display: block; margin: 24px auto" />
          <template v-else-if="sessions.length">
            <NList hoverable clickable class="pe-1">
              <NListItem
                v-for="s in sessions"
                :key="s.id"
                :class="{ selected: s.id === selectedSessionId }"
                @click="selectSession(s.id)"
              >
                <div class="flex items-center justify-between w-full">
                  <div class="flex flex-col min-w-0 flex-1">
                    <span class="text-sm dark:text-white overflow-hidden text-ellipsis whitespace-nowrap">
                      {{ s.name || shortId(s.id) }}
                    </span>
                    <span v-if="s.name" class="text-xs text-gray-400 font-mono overflow-hidden text-ellipsis whitespace-nowrap">
                      {{ shortId(s.id) }}
                    </span>
                  </div>
                  <n-button
                    quaternary
                    size="tiny"
                    class="session-delete-btn"
                    @click="confirmDelete(s.id, $event)"
                  >
                    <template #icon><n-icon size="14"><TrashOutline /></n-icon></template>
                  </n-button>
                </div>
              </NListItem>
            </NList>
            <div v-if="canLoadMore" class="p-3 flex justify-center">
              <n-button
                size="small"
                quaternary
                :loading="sessionsLoadingMore"
                @click="loadMoreSessions"
              >
                Load more ({{ sessionsTotal - sessions.length }} remaining)
              </n-button>
            </div>
          </template>
          <n-empty v-else description="No sessions" style="padding: 24px 0" />
        </NScrollbar>
      </div>
    </NLayoutSider>

    <NLayoutContent>
      <div v-if="!selectedSessionId && !chatMessages.length" class="flex flex-col items-center justify-center h-full">
        <n-empty description="Start a new chat to begin">
          <template #extra>
            <n-button type="primary" @click="newSession">
              <template #icon><n-icon><AddOutline /></n-icon></template>
              New Session
            </n-button>
          </template>
        </n-empty>
        <n-button v-if="isMobile" quaternary style="margin-top: 12px" @click="sidebarCollapsed = !sidebarCollapsed">
          Show Sessions
        </n-button>
      </div>

      <div v-else class="messages-box flex flex-col items-stretch justify-stretch">
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
              <span class="text-gray-800 dark:text-gray-200">
                {{ selectedSessionId ? 'Chat' : 'New chat' }}
              </span>
              <span class="text-xs text-gray-500 dark:text-gray-400 font-mono">
                {{ selectedSessionTitle || '—' }}
              </span>
            </div>
          </div>
        </header>

        <section class="flex flex-col flex-1 min-h-0">
          <div class="flex-1 items-end flex-col justify-end min-h-0">
            <n-scrollbar ref="messagesScrollRef">
              <div class="flex flex-col justify-end items-start gap-2 py-4 px-7 flex-1">
                <n-spin v-if="messagesLoading" size="small" style="display: block; margin: 24px auto" />
                <template v-else>
                  <template v-for="(msg, i) in chatMessages" :key="i">
                    <template v-for="(part, pi) in msg.orderedParts" :key="`${i}-${pi}`">
                      <div v-if="part.kind === 'thinking'" class="thinking-row">
                        <div
                          class="detail-block thinking"
                          :class="{ expanded: part.expanded }"
                          @click="togglePart(part)"
                        >
                          <div class="detail-header">
                            <n-icon size="14" class="detail-chevron"><ChevronForwardOutline /></n-icon>
                            <span class="detail-label">{{ part.label }}</span>
                            <span v-if="!part.expanded" class="detail-preview">
                              {{ part.content.substring(0, 80) }}{{ part.content.length > 80 ? '...' : '' }}
                            </span>
                          </div>
                          <div v-if="part.expanded" class="detail-body">
                            <MarkdownContent :content="part.content" :streaming="!!msg.streaming" />
                          </div>
                        </div>
                      </div>

                      <div v-else-if="part.kind === 'tool'" class="thinking-row">
                        <div
                          class="detail-block tool"
                          :class="{ expanded: part.expanded }"
                          @click="togglePart(part)"
                        >
                          <div class="detail-header">
                            <n-icon size="14" class="detail-chevron"><ChevronForwardOutline /></n-icon>
                            <span class="detail-label">{{ part.label }}</span>
                            <span v-if="!part.expanded" class="detail-preview">
                              {{ part.content.substring(0, 80) }}{{ part.content.length > 80 ? '...' : '' }}
                            </span>
                          </div>
                          <div v-if="part.expanded" class="detail-body">{{ part.content }}</div>
                        </div>
                      </div>

                      <div
                        v-else
                        class="chat-message flex flex-col gap-2 p-3 bg-gray-100 dark:bg-gray-700"
                        :class="{
                          'self-message': msg.role === 'user',
                          'last': isLastInGroup(i) && isLastBubblePart(msg, pi),
                        }"
                      >
                        <MarkdownContent v-if="msg.role === 'assistant'" :content="part.content" :streaming="!!msg.streaming" />
                        <span v-else style="white-space: pre-wrap; word-break: break-word;">{{ part.content }}</span>
                      </div>
                    </template>
                  </template>

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

          <section class="send-message p-4 bg-gray-100 dark:bg-gray-700 flex items-center">
            <input
              v-model="chatInput"
              placeholder="Write Message"
              class="message-input flex-1"
              :disabled="chatLoading"
              @keydown="handleInputKeydown"
            >
            <n-button
              :disabled="!chatInput.trim() || chatLoading"
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
.n-layout {
  padding: 0;
}

.chat-layout {
  height: calc(100vh - 30px);
}

.chat-sidebar {
  height: calc(100vh - 150px);
}

.session-delete-btn {
  opacity: 0;
  transition: opacity 0.15s;
}

:deep(.n-list-item:hover) .session-delete-btn {
  opacity: 1;
}

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

// =============================================
// Message bubbles — matches MessageItem.vue exactly
// =============================================
.dark {
  .chat-message {
    --current-color: #374151;
    --self-background: #424e64;
  }
}

.chat-message {
  --current-color: #f3f4f6;
  --self-background: #e0f7fa;
  max-width: 100%;
  min-width: 0;

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

.thinking-row {
  width: 100%;
  align-self: stretch;
}

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
  font-size: 13px;
  white-space: normal;
  max-height: none;
}

.chat-message .detail-block {
  margin-bottom: 4px;
}

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
