<template>
  <v-main class="default-main">
    <router-view />
  </v-main>
</template>

<script lang="ts" setup>
</script>

<style>
.default-main {
  position: relative;
  margin: 0;
  margin-inline-start: 220px;
  padding: calc(var(--v-layout-top, 64px) + 24px) 28px 24px;
  overflow: hidden;
  isolation: isolate;
  background:
    linear-gradient(180deg, rgba(var(--v-theme-surface), 0.16), transparent 240px),
    radial-gradient(circle at 34% 0%, rgba(var(--v-theme-primary), 0.08), transparent 34%),
    transparent;
}

.default-main::before,
.default-main::after {
  position: fixed;
  z-index: 0;
  border-radius: 999px;
  pointer-events: none;
  content: "";
}

.default-main > * {
  position: relative;
  z-index: 1;
}

.default-main::before {
  width: 360px;
  height: 360px;
  top: 92px;
  right: -120px;
  background: rgba(var(--v-theme-primary), 0.08);
  filter: blur(8px);
}

.default-main::after {
  width: 280px;
  height: 280px;
  bottom: -110px;
  left: 260px;
  background: rgba(68, 190, 155, 0.08);
  filter: blur(10px);
}

@media (max-width: 960px) {
  .default-main {
    margin-inline-start: 0;
  }
}

@media (max-width: 600px) {
  .default-main {
    padding: calc(var(--v-layout-top, 56px) + 16px) 16px 16px;
  }
}
</style>