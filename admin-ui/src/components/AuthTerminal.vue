<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { useAuth } from '../composables/useAuth'
import { useApi } from '../composables/useApi'

const props = defineProps({
  engine: { type: String, required: true },
})

const emit = defineEmits(['success', 'error', 'close'])

const api = useApi()
const terminalRef = ref(null)
let terminal = null
let fitAddon = null
let ws = null

onMounted(async () => {
  await nextTick()

  terminal = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: '"Fira Code", "Cascadia Code", Menlo, Monaco, "Courier New", monospace',
    theme: {
      background: '#1e1e2e',
      foreground: '#cdd6f4',
      cursor: '#f5e0dc',
      selectionBackground: '#585b70',
    },
    rows: 20,
    cols: 80,
  })

  fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  terminal.loadAddon(new WebLinksAddon())

  terminal.open(terminalRef.value)
  fitAddon.fit()

  // Connect WebSocket
  const auth = useAuth()
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/engine/auth/terminal?token=${auth.accessToken.value}`

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    // Send start message
    ws.send(JSON.stringify({
      type: 'start',
      engine: props.engine,
    }))

    // Send initial resize
    ws.send(JSON.stringify({
      type: 'resize',
      rows: terminal.rows,
      cols: terminal.cols,
    }))
  }

  ws.onmessage = async (event) => {
    if (!terminal) return
    const msg = JSON.parse(event.data)

    switch (msg.type) {
      case 'output':
        terminal.write(msg.data)
        break
      case 'status':
        // Status update
        break
      case 'exit':
        // Don't trust exit code — opencode may exit non-zero on success.
        // Check actual auth status from the API instead.
        try {
          const resp = await api.get('/api/engine/auth')
          if (resp.ok) {
            const status = await resp.json()
            if (status.authenticated) {
              terminal.write('\r\n\x1b[32mAuthentication successful!\x1b[0m\r\n')
              setTimeout(() => emit('success'), 1500)
              break
            }
          }
        } catch (_) { /* fall through to failure */ }
        terminal.write('\r\n\x1b[31mAuthentication failed.\x1b[0m\r\n')
        emit('error', 'Authentication process exited with an error')
        break
      case 'error':
        terminal.write(`\r\n\x1b[31mError: ${msg.data}\x1b[0m\r\n`)
        emit('error', msg.data)
        break
    }
  }

  ws.onerror = () => {
    if (!terminal) return
    terminal.write('\r\n\x1b[31mWebSocket connection error.\x1b[0m\r\n')
    emit('error', 'Connection error')
  }

  ws.onclose = () => {
    // WebSocket closed
  }

  // Forward terminal input to WebSocket
  terminal.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data }))
    }
  })

  // Handle terminal resize
  terminal.onResize(({ rows, cols }) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', rows, cols }))
    }
  })

  // Handle window resize
  window.addEventListener('resize', handleResize)
})

function handleResize() {
  if (fitAddon) {
    fitAddon.fit()
  }
}

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (ws) {
    ws.close()
    ws = null
  }
  if (terminal) {
    terminal.dispose()
    terminal = null
  }
})
</script>

<template>
  <div ref="terminalRef" style="width: 100%; height: 400px; border-radius: 8px; overflow: hidden;"></div>
</template>
