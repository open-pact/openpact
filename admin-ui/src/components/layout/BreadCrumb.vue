<script setup>
import { useRoute } from 'vue-router'
import { NBreadcrumb, NBreadcrumbItem } from 'naive-ui'
import { RouterLink } from 'vue-router'

const route = useRoute()
</script>

<template>
  <n-breadcrumb class="hidden md:block">
    <n-breadcrumb-item>
      <RouterLink to="/">Home</RouterLink>
    </n-breadcrumb-item>
    <n-breadcrumb-item v-if="route.meta.title">
      {{ route.meta.title }}
    </n-breadcrumb-item>
  </n-breadcrumb>
</template>
