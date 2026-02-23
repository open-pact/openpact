<script setup>
import { storeToRefs } from 'pinia'
import { NPageHeader, NButton, NIcon } from 'naive-ui'
import {
  PanelLeftContract24Regular as CollapseIcon,
  PanelLeftExpand20Regular as ExpandIcon,
  Navigation20Regular as MenuIcon,
} from '@vicons/fluent'
import { useLayoutStore } from '@/store/layout.store'
import ThemeSwitch from './ThemeSwitch.vue'
import UserProfile from './UserProfile.vue'
import BreadCrumb from './BreadCrumb.vue'

const layoutStore = useLayoutStore()
const { collapsed, mobileMode } = storeToRefs(layoutStore)
</script>

<template>
  <n-page-header class="px-2 py-3 navbar relative z-100">
    <template #title>
      <div class="flex items-center">
        <div class="flex w-full justify-start items-center">
          <img v-if="mobileMode" width="30" src="/assets/logo-smallest.svg" alt="OpenPact" class="logo" style="border-radius: 6px">
          <n-button class="mx-2" size="small" quaternary circle @click="layoutStore.toggleSidebar">
            <template #icon>
              <NIcon size="1.2rem">
                <MenuIcon v-if="mobileMode" />
                <ExpandIcon v-else-if="collapsed" />
                <CollapseIcon v-else />
              </NIcon>
            </template>
          </n-button>
        </div>
        <BreadCrumb />
      </div>
    </template>
    <template #extra>
      <div class="flex items-center">
        <ThemeSwitch class="mx-1" />
        <UserProfile class="mx-1" />
      </div>
    </template>
  </n-page-header>
</template>

<style lang="scss">
.navbar {
  border-bottom: solid 1px var(--border-color);
  padding-bottom: 0.4rem;
}
</style>
