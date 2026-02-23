<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { NLayout, NScrollbar } from 'naive-ui'
import Sidebar from './layout/Sidebar.vue'
import Navbar from './layout/Navbar.vue'
import { useLayoutStore } from '@/store/layout.store'

const layoutStore = useLayoutStore()
const route = useRoute()

// Matches YummyAdmin default.vue: fullScreen removes padding and max-width
const fullScreen = computed(() => !!route.meta?.fullScreen)
</script>

<template>
  <n-layout has-sider position="absolute">
    <Sidebar />
    <n-layout :native-scrollbar="false" position="static">
      <div class="main-content flex-1 dark:bg-slate-800 dark:text-white my-2">
        <Navbar />
        <div class="relative h-full">
          <NScrollbar>
            <div class="h-full overflow-auto md:mx-auto"
              :class="{ 'md-container': !fullScreen, 'md:pb-18': !fullScreen, 'p-3': !fullScreen }">
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

.dark {
  .main-content {
    --un-bg-opacity: .6;
    background: rgb(30 41 59 / var(--un-bg-opacity)) !important;
  }
}

.main-content {
  --un-bg-opacity: .4;
  background: #ffffffcc !important;
}
</style>
