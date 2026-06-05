<template>
  <v-app-bar class="app-bar" :elevation="0">
    <v-btn v-if="isMobile" icon="mdi-menu" variant="text" @click="$emit('toggleDrawer')" />
    <v-app-bar-title :text="$t(<string>route.name)" />
    <v-spacer />
    <div class="language-switch" aria-label="Language switcher">
      <template v-for="(lang, index) in languages" :key="lang.value">
        <v-btn
          class="language-switch__button"
          :class="{ 'language-switch__button--active': isActiveLocale(lang.value) }"
          size="small"
          variant="text"
          @click="changeLocale(lang.value)"
        >
          {{ lang.title }}
        </v-btn>
        <span v-if="index < languages.length - 1" class="language-switch__separator">/</span>
      </template>
    </div>
    <v-menu>
      <template v-slot:activator="{ props }">
        <v-btn icon v-bind="props" variant="text">
          <v-icon>mdi-theme-light-dark</v-icon>
        </v-btn>
      </template>
      <v-list>
        <v-list-item
          v-for="th in themes"
          :key="th.value"
          @click="changeTheme(th.value)"
          :prepend-icon="th.icon"
          :active="isActiveTheme(th.value)"
        >
          <v-list-item-title>{{ $t(`theme.${th.value}`) }}</v-list-item-title>
        </v-list-item>
      </v-list>
    </v-menu>
  </v-app-bar>
</template>

<script lang="ts" setup>
import { useLocale, useTheme } from 'vuetify'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { languages } from '@/locales'

defineEmits(['toggleDrawer'])
defineProps(['isMobile'])

const route = useRoute()
const { locale: i18nLocale } = useI18n()
const vuetifyLocale = useLocale()
const theme = useTheme()

const changeLocale = (l: string) => {
  if (isActiveLocale(l)) return
  i18nLocale.value = l
  vuetifyLocale.current.value = l
  localStorage.setItem('locale', l)
  window.location.reload()
}
const isActiveLocale = (l: string) => i18nLocale.value === l
const themes = [
  { value: 'light', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', icon: 'mdi-moon-waning-crescent' },
  { value: 'system', icon: 'mdi-laptop' },
]

const changeTheme = (th: string) => {
  theme.change(th)
  localStorage.setItem('theme', th)
}
const isActiveTheme = (th: string) => {
  const current = localStorage.getItem('theme') ?? 'system'
  return current == th
}
</script>

<style scoped>
.app-bar {
  border-bottom: 1px solid rgba(var(--v-border-color), 0.08);
  background: rgba(var(--v-theme-surface), 0.88) !important;
  backdrop-filter: blur(18px);
  box-shadow: 0 10px 28px rgba(31, 45, 61, 0.06) !important;
  padding-inline: 18px 22px;
}

:deep(.v-toolbar-title) {
  color: rgba(var(--v-theme-on-surface), 0.92);
  font-size: 1.06rem;
  font-weight: 780;
  letter-spacing: 0.02em;
  text-align: start;
}

:deep(.v-toolbar-title__placeholder) {
  overflow: visible;
}

:deep(.v-btn) {
  margin-inline: 3px;
}

.language-switch {
  display: inline-flex;
  align-items: center;
  margin-inline-end: 8px;
  padding: 2px 5px;
  border: 1px solid rgba(var(--v-border-color), 0.12);
  border-radius: 999px;
  background: rgba(var(--v-theme-surface-variant), 0.2);
}

.language-switch__button {
  min-width: 0;
  border-radius: 999px;
  color: rgba(var(--v-theme-on-surface), 0.7);
  font-weight: 720;
  padding-inline: 10px;
  text-transform: none;
}

.language-switch__button--active {
  color: rgb(var(--v-theme-primary));
  background: rgba(var(--v-theme-primary), 0.1);
}

.language-switch__separator {
  color: rgba(var(--v-theme-on-surface), 0.48);
  font-weight: 700;
}

@media (max-width: 600px) {
  .app-bar {
    padding-inline: 6px;
  }

  .language-switch {
    margin-inline-end: 2px;
    padding: 1px 3px;
  }

  .language-switch__button {
    padding-inline: 6px;
  }
}
</style>
