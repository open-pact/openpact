<script setup>
import { useSlots } from 'vue'
import { storeToRefs } from 'pinia'
import { useLayoutStore } from '@/store/layout.store'

const props = defineProps({
  title: String,
  titleSize: { type: String, default: 'normal' },
  stretchHeight: Boolean,
})

const slots = useSlots()
const layout = useLayoutStore()
const { flatDesign } = storeToRefs(layout)
</script>

<template>
  <div :class="{ 'h-full': stretchHeight }">
    <div v-if="slots.header" class="py-3">
      <slot name="header" />
    </div>
    <div class="card-container" :class="{ 'h-full': stretchHeight }">
      <div
        class="card-content dark:bg-slate-900 rounded-md border-solid border-color-default p-4 relative z-10"
        :class="{
          'shadow-lg': !flatDesign,
          'drop-shadow-md': !flatDesign,
          'border-1': flatDesign,
          'h-full': stretchHeight,
        }"
      >
        <div v-if="slots.title" class="mix-blend-difference">
          <slot name="title" />
        </div>
        <div v-else-if="title">
          <h3 class="title pb-2 text-dark-400 dark:text-light-800 mix-blend-difference" :class="`title-${titleSize}`">
            {{ title }}
          </h3>
        </div>
        <div v-if="slots.subtitle">
          <slot name="subtitle" />
        </div>
        <slot />
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.card-container {
  position: relative;

  .card-content {
    --un-bg-opacity: 0.7;
  }

  .title {
    font-size: 1.1rem;
    font-weight: 500;
  }

  .title-small {
    font-size: 0.9rem;
    padding-bottom: 0.2rem;
  }

  .title-large {
    font-size: 1.4rem;
    font-weight: 700;
  }
}
</style>
