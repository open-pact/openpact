# YummyAdmin Theme — AI Reference Document

This document provides a comprehensive reference to the YummyAdmin theme patterns, components, and conventions. Use it when implementing or modifying the OpenPact admin UI.

**Source location:** `ai/theme/YummyAdmin/src/`

---

## 1. Project Structure

```
ai/theme/YummyAdmin/src/
├── App.vue                     # Root — theme provider, dark/light switching
├── main.ts                     # Entry — Pinia, UnoCSS, SCSS imports, router setup
├── common/
│   └── theme/
│       └── theme-overrides.ts  # Naive UI light + dark theme overrides
├── composables/
│   ├── useColors.ts            # Color manipulation utilities
│   └── useRender.ts            # Menu/table render helpers (h() functions)
├── components/
│   ├── Navbar/
│   │   ├── Navbar.vue          # Top bar — collapse toggle, breadcrumb, theme switch, user
│   │   ├── ThemeSwitch.vue     # Animated sun/moon SVG toggle
│   │   ├── BreadCrumb.vue      # Route-based breadcrumb
│   │   └── UserProfile.vue     # Avatar + dropdown (profile, logout)
│   ├── shared/
│   │   ├── Card.vue            # Themed card wrapper (flat/shadow, title sizes)
│   │   ├── Sidebar.vue         # Sidebar with menu, logo, collapse behavior
│   │   ├── SidebarMenu.vue     # NMenu wrapper with route-based active detection
│   │   └── DarkModeContainer.vue  # Circular reveal dark mode transition
│   └── CustomizeDialog/
│       └── CustomizeDialog.vue # Theme customization drawer (NOT needed for OpenPact)
├── layouts/
│   └── default.vue             # Main layout — sidebar + navbar + content area
├── store/
│   └── layout.store.ts         # Pinia store — dark mode, collapsed, theme color, mobile
├── styles/
│   ├── main.scss               # CSS variables, body styles, scrollbar, dark mode
│   └── utils/
│       ├── _animations.scss    # Route transitions, rotation, bell shake
│       ├── _override.scss      # Naive UI component style overrides
│       ├── _progress.scss      # NProgress bar styling
│       ├── _error.scss         # Error page styling
│       └── _fonts.scss         # Custom font faces (RTL — NOT needed)
└── modules/
    └── pinia.ts                # Pinia installation with persistence plugin
```

---

## 2. Theme System

### CSS Variables (defined in `styles/main.scss`)

```scss
:root {
  --primary-color: #00ad4c;      // OpenPact will use #2db8b3
  --background: #EEE;            // Light mode background
  --main-content: #FFF;          // Light mode content area
  --border-color: #e0dfdf74;     // Light mode border
  --background-dark: #283046;    // Dark mode background base
  --second-background: #FFF;
  --success-color: #00ad4c;
  --error-color: #ff4d4f;
}

.dark {
  --background: var(--background-dark);
  --main-content: #212637;
  --border-color: #20253578;
  --second-background: #1e293b;
}
```

### Naive UI Theme Overrides (`common/theme/theme-overrides.ts`)

**Light theme:**
```js
{
  common: {
    primaryColor: '#00ad4c',     // OpenPact: '#2db8b3'
    errorColor: '#FF0055',
    warningColor: '#FF8000',
    borderRadius: '5px',         // OpenPact: '8px'
    borderRadiusSmall: '3px',    // OpenPact: '6px'
    borderColor: '#e4e7ec',
  },
  Card: { borderRadius: '7px' },  // OpenPact: '12px'
  Tag: { borderRadius: '4px' },
  List: { borderRadius: '0', borderColorPopover: '#e4e7ec' },
  Notification: { padding: '15px' },
}
```

**Dark theme additions:**
```js
{
  common: {
    borderColor: '#4b556eff',
    cardColor: '#202c4633',
    popoverColor: '#0f172a',
    modalColor: '#1c202c',
  },
  Card: { color: '#1c202c' },
  Dropdown: { color: '#1c2334' },
  Drawer: { color: '#1c202c' },
  DataTable: {
    thColor: '#1c202c',
    tdColor: '#1c2334',
    hoverColor: '#1c202c',
    tdColorHover: '#1c202c',
  },
  List: { borderColorPopover: '#1c2334', colorHoverPopover: '#1c202c' },
}
```

### Theme Switching (in `App.vue`)

The root component manages theme state:

```vue
<script setup>
import { darkTheme, lightTheme } from 'naive-ui'
import themeOverrides, { darkThemeOverrides } from '~/common/theme/theme-overrides'
import useColors from './composables/useColors'

const layout = useLayoutStore()
const customTheme = ref({ ...themeOverrides })
const customDarkTheme = ref({ ...themeOverrides, ...darkThemeOverrides })
const { makeLighter } = useColors()

// Reactive theme switching
const activeTheme = ref(layout.isDark ? darkTheme : lightTheme)
watch(() => layout.isDark, (isDark) => {
  setTimeout(() => { activeTheme.value = isDark ? darkTheme : lightTheme }, 1)
}, { immediate: true })

const activeThemeOverrides = computed(() =>
  layout.isDark ? customDarkTheme.value : customTheme.value
)

// Dynamic primary color (updates CSS vars + Naive UI overrides)
function setThemeColor(newValue) {
  const shade1 = makeLighter(newValue, 0.8)
  const shade2 = makeLighter(newValue, 0.7)
  const shade3 = makeLighter(newValue, 0.7)

  document.documentElement.style.setProperty('--primary-color', newValue)
  document.documentElement.style.setProperty('--primary-color-shade1', shade1)
  document.documentElement.style.setProperty('--primary-color-shade2', shade2)
  document.documentElement.style.setProperty('--primary-color-shade3', shade3)

  customTheme.value.common.primaryColor = newValue
  customTheme.value.common.primaryColorHover = shade1
  customTheme.value.common.primaryColorPressed = shade2
  customTheme.value.common.primaryColorSuppl = shade3

  customDarkTheme.value.common.primaryColor = newValue
  customDarkTheme.value.common.primaryColorHover = shade1
  customDarkTheme.value.common.primaryColorPressed = shade2
  customDarkTheme.value.common.primaryColorSuppl = shade3
}
</script>

<template>
  <n-config-provider inline-theme-disabled :theme="activeTheme"
    :theme-overrides="activeThemeOverrides">
    <n-notification-provider>
      <n-message-provider>
        <n-dialog-provider>
          <DarkModeContainer class="z-1" />
          <RouterView />
        </n-dialog-provider>
      </n-message-provider>
    </n-notification-provider>
  </n-config-provider>
</template>
```

Key points:
- `inline-theme-disabled` prevents inline styles for better performance
- Theme switch has 1ms delay to allow DarkModeContainer animation
- Both light and dark overrides share the same primary color
- CSS variables are synced with Naive UI overrides

---

## 3. Layout System

### Default Layout (`layouts/default.vue`)

```vue
<template>
  <n-layout has-sider position="absolute">
    <Sidebar />
    <n-layout :native-scrollbar="false" position="static">
      <div class="main-content flex-1 dark:bg-slate-800 dark:text-white my-2">
        <Navbar />
        <div class="relative h-full">
          <NScrollbar>
            <div class="h-full overflow-auto md:mx-auto"
              :class="{ 'md-container': !effectiveFluid, 'p-3': !fullScreen }">
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
```

**Structure:** Sidebar (fixed left) + Main area (navbar on top, scrollable content below).

**Key styles:**
```scss
.main-content {
  background: var(--main-content);
  height: calc(100vh - 1.3rem);
  border-radius: 10px;
  padding-bottom: 0.2rem;
  overflow-y: hidden;
}

// Dark mode opacity
.dark .main-content {
  --un-bg-opacity: .6;
  background: rgb(30 41 59 / var(--un-bg-opacity)) !important;
}

// Light mode
.main-content {
  --un-bg-opacity: .4;
  background: #ffffffcc !important;
}

// Layout padding
.n-layout {
  padding: 0 4px;
  background-color: transparent !important;
}
```

### Route Transitions (`styles/utils/_animations.scss`)

```scss
.route-enter-from {
  opacity: 0;
  transform: translateY(100px);
}
.route-enter-active {
  transition: all 0.3s ease-out;
}
.route-leave-to {
  opacity: 0;
  transform: translateY(-100px);
}
.route-leave-active {
  transition: all 0.3s ease-in;
}
```

---

## 4. Shared Components

### Card (`components/shared/Card.vue`)

Themed card wrapper with flat/shadow modes and title sizes.

```vue
<script setup>
import { storeToRefs } from 'pinia'

const props = withDefaults(defineProps({
  title: String,
  titleSize: { type: String, default: 'normal' },  // 'small' | 'normal' | 'large'
  stretchHeight: Boolean,
}), { titleSize: 'normal', stretch: false })

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
      <div class="card-content dark:bg-slate-900 rounded-md border-solid border-color-default p-4 relative z-10"
        :class="{
          'shadow-lg': !flatDesign,
          'drop-shadow-md': !flatDesign,
          'border-1': flatDesign,
          'h-full': stretchHeight
        }">
        <div v-if="slots.title" class="mix-blend-difference">
          <slot name="title" />
        </div>
        <div v-else-if="title">
          <h3 class="title pb-2 text-dark-400 dark:text-light-800 mix-blend-difference"
            :class="`title-${titleSize}`">
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
```

**Slots:** `header`, `title`, `subtitle`, `default`

**Props:**
- `title` — plain text title
- `titleSize` — `'small'` (0.9rem), `'normal'` (1.1rem), `'large'` (1.4rem bold)
- `stretchHeight` — fills parent height

**Style notes:**
- Uses `dark:bg-slate-900` (UnoCSS) for dark mode background
- `border-color-default` uses the theme's border color
- `flatDesign` store value controls shadow vs border style
- Card content has `--un-bg-opacity: .7` for slight transparency

### Sidebar (`components/shared/Sidebar.vue`)

```vue
<template>
  <n-layout-sider :native-scrollbar="false" collapse-mode="width"
    :collapsed-width="mobileMode ? 0 : 64"
    :collapsed="effectiveCollapsed"
    :class="{
      'collapsed': effectiveCollapsed,
      'mobile-mode': mobileMode,
    }">
    <div class="logo-container mb-4">
      <div flex w-full justify-between items-center>
        <div flex w-full justify-start items-center>
          <div class="logo-bg">
            <img src="@/assets/images/logo.png" alt="logo" class="logo">
          </div>
          <h1 class="main-title">{{ t('title') }}</h1>
        </div>
        <n-button v-if="mobileMode" mx-2 size="small" tertiary circle
          @click="layoutStore.closeSidebar">
          <template #icon>
            <NIcon size="1.2rem"><CloseIcon /></NIcon>
          </template>
        </n-button>
      </div>
    </div>
    <SidebarMenu :collapsed-width="mobileMode ? 0 : 64"
      :collapsed-icon-size="mobileMode ? 30 : 20"
      :options="menuOptions" />
  </n-layout-sider>
</template>
```

**Key behavior:**
- `effectiveCollapsed` = mobile ? `mobileMenuClosed` : `collapsed || forceCollapsed`
- Collapsed width: 64px (desktop), 0px (mobile)
- Logo hides title text when collapsed
- Mobile mode: full-width overlay with close button
- Route changes auto-close mobile sidebar

### SidebarMenu (`components/shared/SidebarMenu.vue`)

```vue
<script setup>
export interface SidebarMenuOption {
  type?: string
  label?: string
  key?: string
  icon?: any
  activeIcon?: any
  isNew?: boolean
  showBadge?: boolean
  route?: string
  children?: SidebarMenuOption[]
}

const props = defineProps({ options: Array })
const route = useRoute()
const selectedMenuKey = ref('dashboard')
const menuRef = ref(null)
const { renderIcon, renderLabel } = useRender()

onMounted(() => activateCurrentRoute())

// Flattens nested menu to find current route's key
function activateCurrentRoute() {
  setTimeout(() => {
    const keys = props.options.flatMap(m =>
      m.children ? [m, ...m.children.flatMap(child => child.children || child)] : m
    )
    selectedMenuKey.value = keys.find(
      s => s.key?.toLowerCase() === route.name.toLowerCase()
    )?.key ?? 'dashboard-ecommerce'
    menuRef.value?.showOption(selectedMenuKey.value)
  }, 20)
}

watch(() => route.name, () => {
  setTimeout(() => activateCurrentRoute(), 200)
})

// Convert custom options to Naive UI MenuOption format
function convertToMenuOption(item) {
  return {
    type: item.type,
    label: item.route
      ? () => renderLabel(item.label, item.route, item.isNew ?? false)
      : () => item.label,
    icon: renderIcon(
      isActiveRoute(item) && item.activeIcon ? item.activeIcon : item.icon,
      item.showBadge
    ),
    key: item.key,
    children: item.children?.map(i => convertToMenuOption(i)),
  }
}
</script>

<template>
  <n-menu ref="menuRef" v-bind="$attrs" accordion
    v-model:value="selectedMenuKey" :options="items" />
</template>
```

### ThemeSwitch (`components/Navbar/ThemeSwitch.vue`)

Animated SVG toggle with sun→moon transition:

```vue
<template>
  <div v-bind="$attrs">
    <n-tooltip placement="top" trigger="hover">
      <template #trigger>
        <n-button quaternary circle @click="layout.toggleTheme()"
          class="theme-toggle" id="theme-toggle"
          :class="{ 'theme-toggle--toggled': layout.isDark }">
          <template #icon>
            <NIcon size="1.4rem" :color="layout.isDark ? '#FFF' : '#444'">
              <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true"
                width="1em" height="1em" fill="currentColor"
                class="theme-toggle__expand" viewBox="0 0 32 32">
                <clipPath id="theme-toggle__expand__cutout">
                  <path d="M0-11h25a1 1 0 0017 13v30H0Z" />
                </clipPath>
                <g clip-path="url(#theme-toggle__expand__cutout)">
                  <circle cx="16" cy="16" r="8.4" />
                  <path d="M18.3 3.2c0 1.3-1 2.3-2.3 2.3s-2.3-1-2.3-2.3S14.7.9 16 .9s2.3 1 2.3 2.3zm-4.6 25.6c0-1.3 1-2.3 2.3-2.3s2.3 1 2.3 2.3-1 2.3-2.3 2.3-2.3-1-2.3-2.3zm15.1-10.5c-1.3 0-2.3-1-2.3-2.3s1-2.3 2.3-2.3 2.3 1 2.3 2.3-1 2.3-2.3 2.3zM3.2 13.7c1.3 0 2.3 1 2.3 2.3s-1 2.3-2.3 2.3S.9 17.3.9 16s1-2.3 2.3-2.3zm5.8-7C9 7.9 7.9 9 6.7 9S4.4 8 4.4 6.7s1-2.3 2.3-2.3S9 5.4 9 6.7zm16.3 21c-1.3 0-2.3-1-2.3-2.3s1-2.3 2.3-2.3 2.3 1 2.3 2.3-1 2.3-2.3 2.3zm2.4-21c0 1.3-1 2.3-2.3 2.3S23 7.9 23 6.7s1-2.3 2.3-2.3 2.4 1 2.4 2.3zM6.7 23C8 23 9 24 9 25.3s-1 2.3-2.3 2.3-2.3-1-2.3-2.3 1-2.3 2.3-2.3z" />
                </g>
              </svg>
            </NIcon>
          </template>
        </n-button>
      </template>
      <span>Toggle dark mode</span>
    </n-tooltip>
  </div>
</template>
```

**CSS animation:** 500ms transition — in dark mode the sun rays shrink and circle expands (moon), clipPath shifts to reveal crescent. Uses `d: path(...)` for smooth SVG path morphing.

**Important:** The button must have `id="theme-toggle"` — `DarkModeContainer` uses this to position its circular reveal animation.

### DarkModeContainer (`components/shared/DarkModeContainer.vue`)

Circular reveal animation centered on the theme toggle button:

```vue
<script setup>
const layout = useLayoutStore()
const toggleButtonPosition = ref({ left: 0, top: 0 })
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
    setTimeout(() => { transitionDone.value = true }, 1000)
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

// Re-position after route changes
const router = useRouter()
router.afterEach(() => {
  setTimeout(() => getToggleElementPosition(), 1000)
})
</script>

<template>
  <div class="dark-mode-container"
    :style="{ left: toggleButtonPosition.left, top: toggleButtonPosition.top }"
    :class="{ done: transitionDone }">
    <div class="dark-mode" :class="{ active: layout.isDark }"></div>
  </div>
</template>
```

**CSS:**
```scss
.dark-mode-container {
  position: absolute;
  width: 20px; height: 20px;
  z-index: 0;

  .dark-mode {
    position: relative;
    transform: scale(0);
    width: 250vw; height: 250vw;
    border-radius: 50%;
    background: var(--background-dark);
    transition: 1000ms ease-in-out;

    &.active { transform: scale(1); }
  }
}
```

**How it works:**
1. When `isDark` becomes true → dark circle expands from toggle button position
2. After 800ms → `document.documentElement.classList.add('dark')` (applies CSS variable overrides)
3. When `isDark` becomes false → `dark` class removed immediately, circle shrinks over 1000ms

### Navbar (`components/Navbar/Navbar.vue`)

```vue
<template>
  <n-page-header class="px-2 py-3 navbar relative z-100">
    <template #title>
      <div class="flex items-center">
        <div flex w-full justify-start items-center>
          <img v-if="mobileMode" width="35" src="@/assets/images/logo.png" alt="logo">
          <n-button mx-2 size="small" quaternary circle @click="layoutStore.toggleSidebar">
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
```

**Icons from `@vicons/fluent`:**
- `PanelLeftContract24Regular` — collapse icon
- `PanelLeftExpand20Regular` — expand icon
- `Navigation20Regular` — mobile menu icon

### BreadCrumb (`components/Navbar/BreadCrumb.vue`)

```vue
<template>
  <n-breadcrumb class="hidden md:block">
    <n-breadcrumb-item>
      <RouterLink to="/">Home</RouterLink>
    </n-breadcrumb-item>
    <n-breadcrumb-item v-for="item in route.meta.breadcrumb" :key="item">
      {{ item }}
    </n-breadcrumb-item>
  </n-breadcrumb>
</template>
```

Hidden on mobile (`hidden md:block`). Reads `route.meta.breadcrumb` array.

### UserProfile (`components/Navbar/UserProfile.vue`)

```vue
<template>
  <div class="flex items-center" v-bind="$attrs">
    <n-dropdown :options="items">
      <NImage class="avatar" preview-disabled
        :src="userProfile.avatar" alt="avatar"
        fallbackSrc="/assets/images/avatar.png" />
    </n-dropdown>
  </div>
</template>
```

Avatar with dropdown menu (profile link, logout). Avatar is 33x33px circle.

---

## 5. Pinia Layout Store (`store/layout.store.ts`)

```js
export const useLayoutStore = defineStore('layout', () => {
  // Sidebar State
  const collapsed = ref(false)
  const forceCollapsed = ref(false)    // Force collapse on tablet (≤1024px)
  const mobileMenuClosed = ref(true)
  const mobileMode = ref(false)        // <600px

  // Theme State
  const isDark = ref(false)            // Dark mode toggle
  const themeColor = ref('#00ad4c')    // Primary color (OpenPact: '#2db8b3')
  const flatDesign = ref(true)         // Border style (true) vs shadow style

  // Responsive watchers
  watch(() => useWindowSize().width.value, (newValue) => {
    forceCollapsed.value = newValue <= 1024
    mobileMode.value = newValue < 600
  }, { immediate: true })

  // Methods
  function toggleSidebar() {
    if (mobileMode.value) mobileMenuClosed.value = false
    else collapsed.value = !collapsed.value
  }

  function closeSidebar() { mobileMenuClosed.value = true }

  function toggleTheme() { isDark.value = !isDark.value }

  function setThemeColor(color) { themeColor.value = color }

  return {
    collapsed, forceCollapsed, mobileMode, mobileMenuClosed,
    isDark, themeColor, flatDesign,
    toggleSidebar, closeSidebar, toggleTheme, setThemeColor,
  }
}, {
  persist: {
    omit: ['mobileMode', 'forceCollapsed'],  // Don't persist responsive state
  },
})
```

**Persistence:** Uses `pinia-plugin-persistedstate`. Stores to `localStorage` automatically. Theme preference (dark/light) survives page reload.

---

## 6. UnoCSS Configuration (`uno.config.ts`)

```js
import {
  defineConfig,
  presetAttributify,
  presetIcons,
  presetTypography,
  presetUno,
  presetWebFonts,
  presetWind,
  transformerDirectives,
  transformerVariantGroup,
} from 'unocss'

export default defineConfig({
  shortcuts: [
    ['btn', 'px-4 py-1 rounded inline-block bg-teal-700 text-white cursor-pointer !outline-none hover:bg-teal-800 disabled:cursor-default disabled:bg-gray-600 disabled:opacity-50'],
    ['icon-btn', 'inline-block cursor-pointer select-none opacity-75 transition duration-200 ease-in-out hover:opacity-100 hover:text-teal-600'],
    ['box-row', 'line-row flex flex-col justify-stretch items-stretch lg:flex-row margin-outside w-full'],
  ],
  presets: [
    presetUno(),
    presetAttributify(),
    presetIcons({ scale: 1.2 }),
    presetTypography(),
    presetWind(),
    presetWebFonts({
      fonts: { Inter: 'Inter', Quicksand: 'Quicksand' },
    }),
  ],
  transformers: [transformerDirectives(), transformerVariantGroup()],
  safelist: 'prose m-auto text-left'.split(' '),
})
```

**Commonly used UnoCSS patterns in components:**
- `dark:bg-slate-900`, `dark:bg-slate-800`, `dark:text-white`
- `flex`, `items-center`, `justify-between`, `gap-*`
- `p-3`, `p-4`, `px-2`, `py-3`, `mx-1`, `mx-2`, `mb-3`, `mb-4`
- `w-full`, `h-full`
- `rounded-md`, `border-solid`, `border-1`, `border-color-default`
- `shadow-lg`, `drop-shadow-md`
- `relative`, `absolute`, `z-10`, `z-100`
- `hidden md:block` (responsive visibility)
- `text-dark-400 dark:text-light-800` (theme-aware text)
- Attributify mode: `flex w-full justify-between items-center` (on elements directly)

---

## 7. Color Utilities (`composables/useColors.ts`)

```js
export default function useColors() {
  function makeLighter(color, ratio) {
    // Multiplies each RGB channel by ratio (< 1 = darker, > 1 = lighter)
    const r = Math.floor(parseInt(color.slice(1, 3), 16) * ratio)
    const g = Math.floor(parseInt(color.slice(3, 5), 16) * ratio)
    const b = Math.floor(parseInt(color.slice(5, 7), 16) * ratio)
    return `#${toHex(r)}${toHex(g)}${toHex(b)}`
  }

  function makeDarker(color, ratio) {
    // Divides each RGB channel by ratio
    const r = Math.floor(parseInt(color.slice(1, 3), 16) / ratio)
    const g = Math.floor(parseInt(color.slice(3, 5), 16) / ratio)
    const b = Math.floor(parseInt(color.slice(5, 7), 16) / ratio)
    return `#${toHex(r)}${toHex(g)}${toHex(b)}`
  }

  function buildThemeColorSeries(count) {
    const color = useLayoutStore().themeColor
    const series = []
    for (let i = 0; i < count; i++)
      series.push(makeDarker(color, 1 + i * 0.2))
    return series
  }

  function toHex(num) {
    const hex = num.toString(16)
    return hex.length === 1 ? `0${hex}` : hex
  }

  return { buildThemeColorSeries, makeLighter, makeDarker }
}
```

**Usage in App.vue for theme color shades:**
- `shade1 = makeLighter(color, 0.8)` → hover color
- `shade2 = makeLighter(color, 0.7)` → pressed color
- `shade3 = makeLighter(color, 0.7)` → supplementary color

---

## 8. Responsive Breakpoints

| Width | Behavior |
|-------|----------|
| > 1024px | Full sidebar, all navbar elements visible |
| ≤ 1024px | Sidebar force-collapsed (64px icons only) |
| < 600px | Mobile mode: sidebar hidden, hamburger menu, logo in navbar |

Breakpoints are reactive via `@vueuse/core`'s `useWindowSize()` in the layout store.

---

## 9. Styling Conventions

### Body & HTML
```scss
html, body, #app {
  height: 100%;
  margin: 0;
  padding: 0;
  font-family: 'Inter var', 'ui-sans-serif', 'system-ui', 'sans-serif';
}

body {
  background-color: var(--background);
  max-height: 100vh;
  max-width: 100vw;
  overflow: hidden;
}
```

### Scrollbar
The theme relies on Naive UI's `NScrollbar` component for content scrolling. Custom webkit scrollbar styling is not used — the body overflow is `hidden` and scrolling happens inside `NScrollbar`.

### Border Colors
- Light: `var(--border-color)` = `#e0dfdf74`
- Dark: `var(--border-color)` = `#20253578`
- Naive UI border: `#e4e7ec` (light), `#4b556eff` (dark)

### Selection Color
```scss
body::selection {
  background: color-mix(in srgb, var(--primary-color), var(--background) 85%);
}
```

---

## 10. What NOT to Use from YummyAdmin

These features are YummyAdmin-specific and should NOT be brought into OpenPact:

- **MSW (Mock Service Worker)** — YummyAdmin uses `msw` for mock API data
- **i18n / translations** — YummyAdmin has 6 languages; OpenPact is English-only
- **RTL support** — Right-to-left CSS, Shabnam font, directional adjustments
- **File-based routing** — `vite-plugin-pages`, `vite-plugin-vue-layouts`
- **Auto-import plugins** — `unplugin-auto-import`, `unplugin-vue-components`
- **ApexCharts** — Dashboard charts
- **Vue Quill** — Rich text editor
- **CustomizeDialog** — Theme customization drawer
- **SupportProject widget** — External project promotion
- **Umami analytics** — Usage tracking
- **TypeScript** — OpenPact admin uses JavaScript
- **Notification store** — Toast notification management (OpenPact uses Naive UI's built-in `useMessage`)
- **Profile store** — User profile with avatar (OpenPact has simpler auth)
- **useRender composable** — Complex render helpers for tables (OpenPact already has its own h() patterns)

---

## 11. Common Patterns — Code Examples

### Wrapping page content in a Card
```vue
<Card title="My Section">
  <n-data-table :columns="columns" :data="data" :bordered="false" />
</Card>
```

### Theme-aware page title
```vue
<h2 class="text-dark-400 dark:text-light-800" style="font-size: 1.1rem; font-weight: 500">
  Page Title
</h2>
```

### Using theme switch in navbar
```vue
<ThemeSwitch class="mx-1" />
```

### Sidebar menu option (flat, no children)
```js
const menuOptions = [
  { label: 'Dashboard', key: 'dashboard', icon: DashboardIcon, route: '/' },
  { label: 'Scripts', key: 'scripts', icon: ScriptsIcon, route: '/scripts' },
]
```

### Accessing dark mode state
```js
const layout = useLayoutStore()
// layout.isDark — boolean
// layout.toggleTheme() — toggle
// layout.collapsed — sidebar state
// layout.toggleSidebar() — toggle sidebar
```

### Dark-mode-aware inline styles
```vue
<!-- Use UnoCSS dark: variant -->
<div class="bg-white dark:bg-slate-900 text-gray-800 dark:text-gray-200">
  Content
</div>

<!-- Or use CSS variables -->
<div :style="{ background: 'var(--main-content)' }">
  Content
</div>
```
