import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { useWindowSize } from '@vueuse/core'

export const useLayoutStore = defineStore('layout', () => {
  // Sidebar state
  const collapsed = ref(false)
  const forceCollapsed = ref(false)
  const mobileMenuClosed = ref(true)
  const mobileMode = ref(false)

  // Theme state
  const isDark = ref(true) // Default to dark mode
  const themeColor = ref('#2db8b3')
  const flatDesign = ref(true) // Cards use border (true) vs shadow (false)

  // Responsive breakpoints
  const { width } = useWindowSize()
  watch(width, (newValue) => {
    forceCollapsed.value = newValue <= 1024
    mobileMode.value = newValue < 600
  }, { immediate: true })

  function toggleSidebar() {
    if (mobileMode.value) {
      mobileMenuClosed.value = false
    } else {
      collapsed.value = !collapsed.value
    }
  }

  function closeSidebar() {
    mobileMenuClosed.value = true
  }

  function toggleTheme() {
    isDark.value = !isDark.value
  }

  function setThemeColor(color) {
    themeColor.value = color
  }

  return {
    collapsed,
    forceCollapsed,
    mobileMode,
    mobileMenuClosed,
    isDark,
    themeColor,
    flatDesign,
    toggleSidebar,
    closeSidebar,
    toggleTheme,
    setThemeColor,
  }
}, {
  persist: {
    omit: ['mobileMode', 'forceCollapsed'],
  },
})
