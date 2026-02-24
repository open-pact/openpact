<script setup>
import { ref, computed, watch, onMounted, h } from 'vue'
import { useRoute } from 'vue-router'
import { NMenu, NIcon, NText } from 'naive-ui'
import { RouterLink } from 'vue-router'
import {
  HomeOutline,
  ChatbubblesOutline,
  RadioOutline,
  CodeSlashOutline,
  LockClosedOutline,
  KeyOutline,
  SettingsOutline,
} from '@vicons/ionicons5'

const route = useRoute()
const selectedMenuKey = ref('dashboard')

const menuOptions = [
  { label: 'Dashboard', key: 'dashboard', route: '/', icon: HomeOutline },
  { label: 'Sessions', key: 'sessions', route: '/sessions', icon: ChatbubblesOutline },
  { label: 'Providers', key: 'providers', route: '/providers', icon: RadioOutline },
  { label: 'Scripts', key: 'scripts', route: '/scripts', icon: CodeSlashOutline },
  { label: 'Secrets', key: 'secrets', route: '/secrets', icon: LockClosedOutline },
  { label: 'Engine Auth', key: 'engine-auth', route: '/engine-auth', icon: KeyOutline },
  { label: 'Settings', key: 'settings', route: '/settings', icon: SettingsOutline },
]

function renderIcon(icon) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

function renderLabel(label, path) {
  return () => h(RouterLink, { to: path }, { default: () => h(NText, { class: 'mx-2' }, { default: () => label }) })
}

const items = computed(() =>
  menuOptions.map(o => ({
    label: renderLabel(o.label, o.route),
    icon: renderIcon(o.icon),
    key: o.key,
  }))
)

function activateCurrentRoute() {
  const path = route.path
  if (path === '/') selectedMenuKey.value = 'dashboard'
  else if (path === '/sessions') selectedMenuKey.value = 'sessions'
  else if (path === '/providers') selectedMenuKey.value = 'providers'
  else if (path.startsWith('/scripts')) selectedMenuKey.value = 'scripts'
  else if (path === '/secrets') selectedMenuKey.value = 'secrets'
  else if (path === '/engine-auth') selectedMenuKey.value = 'engine-auth'
  else if (path === '/settings') selectedMenuKey.value = 'settings'
  else selectedMenuKey.value = 'dashboard'
}

onMounted(activateCurrentRoute)
watch(() => route.path, activateCurrentRoute)
</script>

<template>
  <n-menu
    accordion
    v-model:value="selectedMenuKey"
    :options="items"
    style="padding: 0 8px"
  />
</template>
