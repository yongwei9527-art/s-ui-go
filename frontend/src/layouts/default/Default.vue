<template>
  <v-app class="default-shell">
    <drawer :isMobile="isMobile" :displayDrawer="displayDrawer" @toggleDrawer="toggleDrawer" />
    <default-bar :isMobile="isMobile" @toggleDrawer="toggleDrawer" />
    <default-view />
  </v-app>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue'
import DefaultBar from './AppBar.vue'
import Drawer from './Drawer.vue'
import DefaultView from './View.vue'
import { useDisplay } from 'vuetify'

const { smAndDown } = useDisplay()
const displayDrawer = ref(false)

const toggleDrawer = () => {
  displayDrawer.value = !displayDrawer.value
}

const isMobile = computed( ():boolean =>{
  displayDrawer.value = !smAndDown.value
  return smAndDown.value
})
</script>

<style>
.default-shell {
  overflow: auto;
  font-family: Inter, "Segoe UI", "HarmonyOS Sans SC", "Microsoft YaHei UI", "Microsoft YaHei", system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
  background:
    linear-gradient(rgba(var(--v-theme-on-surface), 0.028) 1px, transparent 1px),
    linear-gradient(90deg, rgba(var(--v-theme-on-surface), 0.028) 1px, transparent 1px),
    radial-gradient(circle at 18% 8%, rgba(var(--v-theme-primary), 0.13), transparent 30%),
    radial-gradient(circle at 88% 16%, rgba(88, 181, 246, 0.12), transparent 28%),
    radial-gradient(circle at 70% 92%, rgba(55, 190, 142, 0.1), transparent 32%),
    linear-gradient(135deg, rgba(var(--v-theme-background), 1), rgba(var(--v-theme-surface), 0.88)) !important;
  background-position: -1px -1px, -1px -1px, center, center, center, center;
  background-size: 44px 44px, 44px 44px, auto, auto, auto, auto;
}

.v-card-subtitle {
  text-align: center;
  border-bottom: 1px solid gray;
  min-height: 20px;
}
.v-switch.v-input {
  padding-inline-start: .6rem;
}
</style>