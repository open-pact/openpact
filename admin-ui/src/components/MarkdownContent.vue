<script setup>
import { ref, watch, onBeforeUnmount } from 'vue'
import { Marked } from 'marked'
import { markedHighlight } from 'marked-highlight'
import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/core'
import 'highlight.js/styles/github-dark.min.css'

// Register commonly used languages
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import python from 'highlight.js/lib/languages/python'
import bash from 'highlight.js/lib/languages/bash'
import json from 'highlight.js/lib/languages/json'
import yaml from 'highlight.js/lib/languages/yaml'
import go from 'highlight.js/lib/languages/go'
import css from 'highlight.js/lib/languages/css'
import xml from 'highlight.js/lib/languages/xml'
import sql from 'highlight.js/lib/languages/sql'
import markdown from 'highlight.js/lib/languages/markdown'
import diff from 'highlight.js/lib/languages/diff'
import rust from 'highlight.js/lib/languages/rust'
import java from 'highlight.js/lib/languages/java'
import csharp from 'highlight.js/lib/languages/csharp'

hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('js', javascript)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('ts', typescript)
hljs.registerLanguage('python', python)
hljs.registerLanguage('py', python)
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('sh', bash)
hljs.registerLanguage('shell', bash)
hljs.registerLanguage('json', json)
hljs.registerLanguage('yaml', yaml)
hljs.registerLanguage('yml', yaml)
hljs.registerLanguage('go', go)
hljs.registerLanguage('golang', go)
hljs.registerLanguage('css', css)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('markdown', markdown)
hljs.registerLanguage('md', markdown)
hljs.registerLanguage('diff', diff)
hljs.registerLanguage('rust', rust)
hljs.registerLanguage('rs', rust)
hljs.registerLanguage('java', java)
hljs.registerLanguage('csharp', csharp)
hljs.registerLanguage('cs', csharp)

const props = defineProps({
  content: { type: String, default: '' },
  streaming: { type: Boolean, default: false },
})

const marked = new Marked(
  markedHighlight({
    langPrefix: 'hljs language-',
    highlight(code, lang) {
      if (lang && hljs.getLanguage(lang)) {
        try {
          return hljs.highlight(code, { language: lang }).value
        } catch (_) { /* fall through */ }
      }
      return code
    },
  }),
)

marked.setOptions({
  gfm: true,
  breaks: true,
})

// Configure DOMPurify to allow markdown-safe tags
const purifyConfig = {
  ALLOWED_TAGS: [
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'p', 'br', 'hr',
    'strong', 'em', 'b', 'i', 'u', 's', 'del', 'ins', 'mark',
    'code', 'pre', 'kbd', 'samp', 'var',
    'ul', 'ol', 'li',
    'blockquote',
    'a',
    'table', 'thead', 'tbody', 'tr', 'th', 'td',
    'img',
    'div', 'span',
    'details', 'summary',
    'sup', 'sub',
  ],
  ALLOWED_ATTR: [
    'href', 'target', 'rel', 'class',
    'src', 'alt', 'title', 'width', 'height',
    'colspan', 'rowspan',
    'open',
  ],
  ALLOW_DATA_ATTR: false,
}

// Force all links to open in new tab
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

function render(text) {
  const raw = marked.parse(text)
  return DOMPurify.sanitize(raw, purifyConfig)
}

const renderedHtml = ref(render(props.content))

let debounceTimer = null

watch(
  () => props.content,
  (val) => {
    if (props.streaming) {
      if (debounceTimer) clearTimeout(debounceTimer)
      debounceTimer = setTimeout(() => {
        renderedHtml.value = render(val)
      }, 120)
    } else {
      renderedHtml.value = render(val)
    }
  },
)

// When streaming stops, do a final immediate render
watch(
  () => props.streaming,
  (streaming, wasStreaming) => {
    if (wasStreaming && !streaming) {
      if (debounceTimer) clearTimeout(debounceTimer)
      renderedHtml.value = render(props.content)
    }
  },
)

onBeforeUnmount(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>

<template>
  <div class="markdown-content" v-html="renderedHtml"></div>
</template>

<style>
/* Unscoped — all styles are scoped by the .markdown-content class */
.markdown-content {
  line-height: 1.6;
  word-break: break-word;
  min-width: 0;
}

.markdown-content > *:first-child {
  margin-top: 0;
}

.markdown-content > *:last-child {
  margin-bottom: 0;
}

/* Headings */
.markdown-content h1 {
  font-size: 1.3em;
  font-weight: 600;
  margin: 0.8em 0 0.4em;
}

.markdown-content h2 {
  font-size: 1.2em;
  font-weight: 600;
  margin: 0.7em 0 0.3em;
}

.markdown-content h3 {
  font-size: 1.1em;
  font-weight: 600;
  margin: 0.6em 0 0.3em;
}

.markdown-content h4,
.markdown-content h5,
.markdown-content h6 {
  font-size: 1em;
  font-weight: 600;
  margin: 0.5em 0 0.2em;
}

/* Paragraphs */
.markdown-content p {
  margin: 0.4em 0;
}

/* Inline code */
.markdown-content code {
  background: rgba(128, 128, 128, 0.15);
  border-radius: 3px;
  padding: 0.15em 0.35em;
  font-size: 0.9em;
  font-family: 'SF Mono', 'Fira Code', 'Fira Mono', 'Roboto Mono', 'Consolas', monospace;
}

/* Code blocks */
.markdown-content pre {
  background: #1e1e2e;
  color: #cdd6f4;
  border-radius: 6px;
  padding: 0.8em 1em;
  margin: 0.5em 0;
  overflow-x: auto;
  font-size: 0.85em;
  line-height: 1.5;
}

.markdown-content pre code {
  background: none;
  padding: 0;
  border-radius: 0;
  font-size: inherit;
  color: inherit;
}

/* Blockquotes */
.markdown-content blockquote {
  border-left: 3px solid var(--primary-color, #6366f1);
  margin: 0.5em 0;
  padding: 0.3em 0.8em;
  color: inherit;
  opacity: 0.85;
}

.markdown-content blockquote p {
  margin: 0.2em 0;
}

/* Lists */
.markdown-content ul,
.markdown-content ol {
  margin: 0.4em 0;
  padding-left: 1.5em;
}

.markdown-content li {
  margin: 0.15em 0;
}

.markdown-content li > ul,
.markdown-content li > ol {
  margin: 0.1em 0;
}

/* Tables */
.markdown-content table {
  border-collapse: collapse;
  margin: 0.5em 0;
  width: 100%;
  font-size: 0.9em;
}

.markdown-content th,
.markdown-content td {
  border: 1px solid var(--border-color, rgba(128, 128, 128, 0.3));
  padding: 0.4em 0.6em;
  text-align: left;
}

.markdown-content th {
  font-weight: 600;
  background: rgba(128, 128, 128, 0.1);
}

/* Links */
.markdown-content a {
  color: var(--primary-color, #6366f1);
  text-decoration: none;
}

.markdown-content a:hover {
  text-decoration: underline;
}

/* Horizontal rules */
.markdown-content hr {
  border: none;
  border-top: 1px solid var(--border-color, rgba(128, 128, 128, 0.3));
  margin: 0.6em 0;
}

/* Images */
.markdown-content img {
  max-width: 100%;
  border-radius: 4px;
}

/* Strong / emphasis */
.markdown-content strong {
  font-weight: 600;
}

/* Keyboard */
.markdown-content kbd {
  background: rgba(128, 128, 128, 0.15);
  border: 1px solid rgba(128, 128, 128, 0.3);
  border-radius: 3px;
  padding: 0.1em 0.35em;
  font-size: 0.85em;
  font-family: inherit;
}
</style>
