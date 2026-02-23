<script setup>
import { NLayout, NScrollbar } from 'naive-ui'
import Sidebar from './layout/Sidebar.vue'
import Navbar from './layout/Navbar.vue'
import { useLayoutStore } from '@/store/layout.store'

const layoutStore = useLayoutStore()
</script>

<template>
  <n-layout has-sider position="absolute">
    <Sidebar />
    <n-layout :native-scrollbar="false" position="static">
      <div class="main-content flex-1 dark:bg-slate-800 dark:text-white my-1">
        <Navbar />
        <div class="relative h-full">
          <NScrollbar>
            <div class="h-full overflow-auto md:mx-auto p-4 md:p-6 md-container">
              <router-view v-slot="{ Component, route }">
                <transition name="route" mode="out-in">
                  <div :key="route.name">
                    <component :is="Component" class="relative" />
                  </div>
                </transition>
              </router-view>
            </div>
          </NScrollbar>
        </div>
      </div>
    </n-layout>
  </n-layout>
</template>

<style lang="scss">
.n-layout {
  padding: 0 4px;
  background-color: transparent !important;
}
</style>
