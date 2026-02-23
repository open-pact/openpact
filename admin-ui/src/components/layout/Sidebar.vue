<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { NLayoutSider, NButton, NIcon } from 'naive-ui'
import {
  PanelLeftContract24Regular as CollapseIcon,
  Dismiss24Filled as CloseIcon,
} from '@vicons/fluent'
import { useLayoutStore } from '@/store/layout.store'
import SidebarMenu from './SidebarMenu.vue'

const layoutStore = useLayoutStore()
const { collapsed, forceCollapsed, mobileMode, mobileMenuClosed } = storeToRefs(layoutStore)

const effectiveCollapsed = computed(() => {
  if (mobileMode.value) return mobileMenuClosed.value
  return collapsed.value || forceCollapsed.value
})

// Close mobile sidebar on route change
const router = useRouter()
router.beforeEach(() => {
  layoutStore.closeSidebar()
})
</script>

<template>
  <n-layout-sider
    :native-scrollbar="false"
    collapse-mode="width"
    :collapsed-width="mobileMode ? 0 : 64"
    :collapsed="effectiveCollapsed"
    :class="{
      collapsed: effectiveCollapsed,
      'mobile-mode': mobileMode,
    }"
  >
    <div class="logo-container mb-4">
      <div class="flex w-full justify-between items-center">
        <div class="flex w-full justify-start items-center">
          <div class="logo-bg">
            <img src="/assets/logo-smallest.svg" alt="OpenPact" class="logo" />
          </div>
          <h1 class="main-title">OpenPact</h1>
        </div>
        <n-button v-if="mobileMode" class="mx-2" size="small" tertiary circle @click="layoutStore.closeSidebar">
          <template #icon>
            <NIcon size="1.2rem"><CloseIcon /></NIcon>
          </template>
        </n-button>
      </div>
    </div>
    <SidebarMenu />
  </n-layout-sider>
</template>

<style lang="scss">
.logo-container {
  display: flex;
  align-items: center;
  padding: 1.5rem 0.8rem 0.5rem 1.1rem;
  transition: all 100ms;
  line-height: 1;

  .main-title {
    font-size: 1.2rem;
    font-weight: 600;
    letter-spacing: -0.3px;
    background: linear-gradient(135deg, #2dd7b7, #2387a7);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    user-select: none;
  }

  .logo-bg {
    width: 34px;
    height: 34px;
    display: flex;
    margin: 0 0.4rem;
    justify-content: center;
    align-items: center;

    .logo {
      width: 30px;
      height: 30px;
      border-radius: 6px;
      object-fit: cover;
    }
  }
}

.mobile-mode {
  max-width: 100% !important;
  width: 100% !important;
}

.mobile-mode.collapsed {
  max-width: 0 !important;
}

.collapsed {
  .logo-container {
    padding: 1.5rem 0.5rem 0.5rem 0.5rem;
  }
  .main-title {
    display: none;
  }
}

.n-layout-sider {
  background-color: transparent;
}

.n-menu .n-menu-item-content:not(.n-menu-item-content--disabled):hover::before {
  background-color: rgba(189, 189, 189, 0.15);
}

.n-menu-item {
  user-select: none;
}
</style>
