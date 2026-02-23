import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import naive from 'naive-ui'
import App from './App.vue'
import AppLayout from './components/AppLayout.vue'
import { useAuth } from './composables/useAuth'

import '@unocss/reset/tailwind-compat.css'
import 'uno.css'
import './styles/main.scss'

// Views
import SetupView from './views/SetupView.vue'
import LoginView from './views/LoginView.vue'
import DashboardView from './views/DashboardView.vue'
import ScriptsView from './views/ScriptsView.vue'
import ScriptEditorView from './views/ScriptEditorView.vue'
import EngineAuthView from './views/EngineAuthView.vue'
import SecretsView from './views/SecretsView.vue'
import SessionsView from './views/SessionsView.vue'
import ProvidersView from './views/ProvidersView.vue'

const routes = [
  { path: '/setup', name: 'setup', component: SetupView, meta: { requiresAuth: false } },
  { path: '/login', name: 'login', component: LoginView, meta: { requiresAuth: false } },
  {
    path: '/',
    component: AppLayout,
    children: [
      { path: '', name: 'dashboard', component: DashboardView, meta: { requiresAuth: true, title: 'Dashboard' } },
      { path: 'scripts', name: 'scripts', component: ScriptsView, meta: { requiresAuth: true, title: 'Scripts' } },
      { path: 'scripts/:name', name: 'script-editor', component: ScriptEditorView, meta: { requiresAuth: true, title: 'Script Editor' } },
      { path: 'sessions', name: 'sessions', component: SessionsView, meta: { requiresAuth: true, title: 'Sessions', fullScreen: true } },
      { path: 'providers', name: 'providers', component: ProvidersView, meta: { requiresAuth: true, title: 'Providers' } },
      { path: 'secrets', name: 'secrets', component: SecretsView, meta: { requiresAuth: true, title: 'Secrets' } },
      { path: 'engine-auth', name: 'engine-auth', component: EngineAuthView, meta: { requiresAuth: true, title: 'Engine Auth' } },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Navigation guard (unchanged logic)
router.beforeEach(async (to, from, next) => {
  const auth = useAuth()

  try {
    const response = await fetch('/api/setup/status')
    const data = await response.json()

    if (data.setup_required && to.name !== 'setup') {
      return next({ name: 'setup' })
    }

    if (!data.setup_required && to.name === 'setup') {
      return next({ name: 'login' })
    }
  } catch (e) {
    console.error('Failed to check setup status:', e)
  }

  if (to.meta.requiresAuth && !auth.isAuthenticated.value) {
    const refreshed = await auth.refreshToken()
    if (!refreshed) {
      return next({ name: 'login', query: { redirect: to.fullPath } })
    }
  }

  next()
})

const pinia = createPinia()
pinia.use(piniaPluginPersistedstate)

const app = createApp(App)
app.use(pinia)
app.use(router)
app.use(naive)
app.mount('#app')
