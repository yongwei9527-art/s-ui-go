<template>
  <v-navigation-drawer
    v-model="showDrawer"
    :temporary="isMobile"
    :permanent="!isMobile"
    :width="isMobile ? 240 : 220"
    class="app-drawer"
    @click="isMobile ? $emit('toggleDrawer') : null"
  >
    <div class="drawer-brand">
      <div class="brand-logo">S</div>
      <span>S-UI</span>
    </div>

    <v-list density="compact" nav>
      <v-list-item
        link
        v-for="item in menu"
        :key="item.title"
        :to="item.path"
        :active="router.currentRoute.value.path == item.path"
      >
        <template v-slot:prepend>
          <v-icon :icon="item.icon" />
        </template>
        <v-list-item-title v-text="$t(item.title)" />
      </v-list-item>
    </v-list>

    <template v-slot:append>
      <div class="drawer-footer">
        <v-list-item prepend-icon="mdi-logout" :title="$t('menu.logout')" @click="Logout" />
      </div>
    </template>
  </v-navigation-drawer>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import router from '@/router'
import { logout } from '@/plugins/httputil'

defineEmits(['toggleDrawer'])
const props = defineProps(['isMobile','displayDrawer'])

const showDrawer = computed((): boolean => {
  return props.displayDrawer
})

const menu = [
  { title: 'pages.home', icon: 'mdi-home',  path: '/' },
  { title: 'pages.inbounds', icon: 'mdi-cloud-download',  path: '/inbounds' },
  { title: 'pages.clients', icon: 'mdi-account-multiple',  path: '/clients' },
  { title: 'pages.outbounds', icon: 'mdi-cloud-upload',  path: '/outbounds' },
  { title: 'pages.endpoints', icon: 'mdi-cloud-tags',  path: '/endpoints' },
  { title: 'pages.services', icon: 'mdi-server',  path: '/services' },
  { title: 'pages.tls', icon: 'mdi-certificate',  path: '/tls' },
  { title: 'pages.basics', icon: 'mdi-application-cog',  path: '/basics' },
  { title: 'pages.rules', icon: 'mdi-routes',  path: '/rules' },
  { title: 'pages.dns', icon: 'mdi-dns',  path: '/dns' },
  { title: 'pages.admins', icon: 'mdi-account-tie',  path: '/admins' },
  { title: 'pages.settings', icon: 'mdi-cog',  path: '/settings' },
]

const Logout = async () => {
  logout()
}
</script>

<style scoped>
.app-drawer {
  border-right: 1px solid rgba(var(--v-border-color), 0.08) !important;
  background: rgba(var(--v-theme-surface), 0.86) !important;
  backdrop-filter: blur(18px);
}

.drawer-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 68px;
  padding: 0 24px;
  color: rgba(var(--v-theme-on-surface), 0.9);
  font-size: 1.12rem;
  font-weight: 800;
  letter-spacing: 0.02em;
}

.brand-logo {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border-radius: 50%;
  background: linear-gradient(135deg, #5ed3ff, #2f80ed);
  color: white;
  font-weight: 900;
  box-shadow: 0 10px 24px rgba(47, 128, 237, 0.28);
}

.drawer-footer {
  border-top: 1px solid rgba(var(--v-border-color), 0.08);
  margin: 10px 16px 16px;
  padding-top: 12px;
}

:deep(.v-navigation-drawer__content) {
  display: flex;
  flex-direction: column;
  overflow-x: hidden;
  scrollbar-width: none;
}

:deep(.v-navigation-drawer__content::-webkit-scrollbar) {
  width: 0;
}

:deep(.v-list) {
  padding: 6px 14px;
}

:deep(.v-list-item) {
  margin: 6px 0;
  border-radius: 14px;
  color: rgba(var(--v-theme-on-surface), 0.68);
  min-height: 42px;
  padding-inline: 14px;
  transition: background 0.2s ease, color 0.2s ease, transform 0.2s ease;
}

:deep(.v-list-item__prepend) {
  margin-inline-end: 14px;
}

:deep(.v-list-item-title) {
  font-size: 0.88rem;
  font-weight: 650;
}

:deep(.v-list-item--active) {
  background: rgba(var(--v-theme-primary), 0.12);
  color: rgb(var(--v-theme-primary));
  box-shadow: inset 0 0 0 1px rgba(var(--v-theme-primary), 0.08), 0 8px 20px rgba(31, 45, 61, 0.06);
}

:deep(.v-list-item:hover) {
  background: rgba(var(--v-theme-primary), 0.08);
  transform: translateX(2px);
}

:deep(.v-icon) {
  opacity: 0.86;
  font-size: 1.08rem;
}

@media (max-width: 960px) {
  .drawer-brand {
    height: 68px;
  }
}
</style>
