<script setup>
import { ref, computed, watch } from 'vue'
import { NConfigProvider, NMessageProvider, NDialogProvider, darkTheme, lightTheme } from 'naive-ui'
import themeOverrides, { darkThemeOverrides } from './common/theme-overrides'
import useColors from './composables/useColors'
import { useLayoutStore } from './store/layout.store'
import DarkModeContainer from './components/layout/DarkModeContainer.vue'

const layout = useLayoutStore()
const { makeLighter } = useColors()

// Theme objects (mutable copies for dynamic color updates)
const customTheme = ref({ ...themeOverrides })
const customDarkTheme = ref({ ...themeOverrides, ...darkThemeOverrides })

// Reactive theme switching
const activeTheme = ref(layout.isDark ? darkTheme : lightTheme)
watch(() => layout.isDark, (isDark) => {
  setTimeout(() => {
    activeTheme.value = isDark ? darkTheme : lightTheme
  }, 1)
}, { immediate: true })

const activeThemeOverrides = computed(() =>
  layout.isDark ? customDarkTheme.value : customTheme.value
)

// Dynamic primary color (syncs CSS vars + Naive UI overrides)
watch(() => layout.themeColor, (newValue) => {
  if (!newValue) return

  const shade1 = makeLighter(newValue, 0.8)
  const shade2 = makeLighter(newValue, 0.7)
  const shade3 = makeLighter(newValue, 0.7)

  document.documentElement.style.setProperty('--primary-color', newValue)

  if (customTheme.value.common) {
    customTheme.value.common.primaryColor = newValue
    customTheme.value.common.primaryColorHover = shade1
    customTheme.value.common.primaryColorPressed = shade2
    customTheme.value.common.primaryColorSuppl = shade3
  }

  if (customDarkTheme.value.common) {
    customDarkTheme.value.common.primaryColor = newValue
    customDarkTheme.value.common.primaryColorHover = shade1
    customDarkTheme.value.common.primaryColorPressed = shade2
    customDarkTheme.value.common.primaryColorSuppl = shade3
  }
}, { immediate: true })
</script>

<template>
  <n-config-provider inline-theme-disabled :theme="activeTheme" :theme-overrides="activeThemeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <DarkModeContainer class="z-1" />
        <router-view />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
