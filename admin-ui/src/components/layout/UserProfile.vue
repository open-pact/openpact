<script setup>
import { h } from 'vue'
import { NDropdown, NIcon, NButton } from 'naive-ui'
import { LogOutOutline, PersonOutline } from '@vicons/ionicons5'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

const router = useRouter()
const auth = useAuth()

function renderIcon(icon) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

const options = [
  {
    label: 'Logout',
    key: 'logout',
    icon: renderIcon(LogOutOutline),
  },
]

async function handleSelect(key) {
  if (key === 'logout') {
    await auth.logout()
    router.push('/login')
  }
}
</script>

<template>
  <div class="flex items-center" v-bind="$attrs">
    <n-dropdown :options="options" @select="handleSelect">
      <n-button quaternary size="small">
        <template #icon>
          <n-icon><PersonOutline /></n-icon>
        </template>
        {{ auth.user.value?.username || 'User' }}
      </n-button>
    </n-dropdown>
  </div>
</template>
