<script setup>
import { ref, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useLayoutStore } from '@/store/layout.store'

const layout = useLayoutStore()
const toggleButtonPosition = ref({ left: '0px', top: '0px' })
const transitionDone = ref(true)

onMounted(() => {
  setTimeout(() => getToggleElementPosition(), 100)
})

watch(() => layout.isDark, (newValue) => {
  if (newValue) {
    transitionDone.value = false
    setTimeout(() => {
      document.documentElement.classList.add('dark')
      transitionDone.value = true
    }, 800)
  } else {
    transitionDone.value = false
    document.documentElement.classList.remove('dark')
    setTimeout(() => {
      transitionDone.value = true
    }, 1000)
  }
}, { immediate: true })

function getToggleElementPosition() {
  const element = document.querySelector('#theme-toggle')
  if (!element) return
  const rect = element.getBoundingClientRect()
  toggleButtonPosition.value = {
    left: `${rect.left + window.scrollX}px`,
    top: `${rect.top + window.scrollY}px`,
  }
}

const router = useRouter()
router.afterEach(() => {
  setTimeout(() => getToggleElementPosition(), 1000)
})
</script>

<template>
  <div
    class="dark-mode-container"
    :style="{ left: toggleButtonPosition.left, top: toggleButtonPosition.top }"
    :class="{ done: transitionDone }"
  >
    <div class="dark-mode" :class="{ active: layout.isDark }"></div>
  </div>
</template>

<style lang="scss" scoped>
.dark-mode-container {
  position: absolute;
  display: flex;
  justify-content: center;
  align-items: center;
  width: 20px;
  height: 20px;
  z-index: 9999;
  pointer-events: none;

  &.done {
    z-index: -1;
  }

  .dark-mode {
    position: relative;
    transform: scale(0);
    left: 0;
    right: 0;
    top: 0;
    bottom: 0;
    width: 250vw;
    height: 250vw;
    border-radius: 50%;
    background: var(--background-dark);
    transition: 1000ms ease-in-out;
    display: flex;
    flex: 0 0 auto;

    &.active {
      transform: scale(1);
      transition: 1000ms ease-in-out;
    }
  }
}
</style>
