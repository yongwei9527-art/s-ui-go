<template>
  <div class="login-page">
    <div class="login-bg login-bg-one"></div>
    <div class="login-bg login-bg-two"></div>
    <div class="login-background-brand" aria-hidden="true">S-UI</div>

    <v-container class="login-container">
      <v-row justify="center" align="center" class="login-row">
        <v-col cols="12" sm="8" md="5" lg="4" xl="3">
          <v-card class="login-card" elevation="0">
            <v-card-text class="login-card-content">
              <div class="login-heading">
                <h1 v-text="$t('login.title')"></h1>
              </div>

              <v-form @submit.prevent="login" ref="form" class="login-form">
                <v-text-field
                  v-model="username"
                  :label="$t('login.username')"
                  :rules="usernameRules"
                  required
                  class="soft-field"
                  prepend-inner-icon="mdi-account-outline"
                ></v-text-field>
                <v-text-field
                  v-model="password"
                  :label="$t('login.password')"
                  :rules="passwordRules"
                  type="password"
                  required
                  class="soft-field"
                  prepend-inner-icon="mdi-lock-outline"
                ></v-text-field>
                <v-btn
                  :loading="loading"
                  type="submit"
                  color="primary"
                  block
                  size="large"
                  class="login-submit"
                  v-text="$t('actions.submit')"
                ></v-btn>
              </v-form>

              <div class="login-actions">
                <v-select
                  density="comfortable"
                  hide-details
                  variant="solo"
                  :items="languages"
                  v-model="$i18n.locale"
                  @update:modelValue="changeLocale"
                  class="language-select"
                ></v-select>

                <v-menu>
                  <template v-slot:activator="{ props }">
                    <v-btn icon v-bind="props" class="theme-button" variant="flat">
                      <v-icon>mdi-theme-light-dark</v-icon>
                    </v-btn>
                  </template>
                  <v-list class="theme-list" density="comfortable">
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
              </div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>
    </v-container>
  </div>
</template>

<script lang="ts" setup>
import { ref } from "vue"
import { useLocale,useTheme } from 'vuetify'
import { i18n, languages } from '@/locales'
import { useRouter } from 'vue-router'
import HttpUtil from '@/plugins/httputil'


const theme = useTheme()
const locale = useLocale()

const themes = [
  { value: 'light', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', icon: 'mdi-moon-waning-crescent' },
  { value: 'system', icon: 'mdi-laptop' },
]

const username = ref('')
const usernameRules = [
  (value: string) => {
    if (value?.length > 0) return true
    return i18n.global.t('login.unRules')
  },
]

const password = ref('')
const passwordRules = [
  (value: string) => {
    if (value?.length > 0) return true
    return i18n.global.t('login.pwRules')
  },
]

const loading = ref(false)
const router = useRouter()

const login = async () => {
  if (username.value == '' || password.value == '') return
  loading.value=true
  const response = await HttpUtil.post('api/login',{user: username.value, pass: password.value})
  if(response.success){
    localStorage.setItem('s-ui-authenticated', 'true')
    setTimeout(() => {
      loading.value=false
      router.push('/')
    }, 500)
  } else {
    localStorage.removeItem('s-ui-authenticated')
    loading.value=false
  }
}
const changeLocale = (l: any) => {
  const nextLocale = l ?? 'en'
  i18n.global.locale.value = nextLocale
  locale.current.value = nextLocale
  localStorage.setItem('locale', nextLocale)
}
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
.login-page {
  position: relative;
  display: block;
  min-height: 100vh;
  min-height: 100dvh;
  overflow: hidden;
  font-family: Inter, "Segoe UI", "HarmonyOS Sans SC", "Microsoft YaHei UI", "Microsoft YaHei", system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
  background:
    radial-gradient(circle at 18% 16%, rgba(95, 134, 255, 0.14), transparent 30%),
    radial-gradient(circle at 80% 80%, rgba(72, 187, 163, 0.1), transparent 36%),
    linear-gradient(135deg, rgb(var(--v-theme-background)) 0%, rgba(var(--v-theme-surface), 0.9) 100%);
}

.login-bg {
  position: absolute;
  border-radius: 999px;
  filter: blur(4px);
  opacity: 0.38;
  pointer-events: none;
}

.login-bg-one {
  width: 360px;
  height: 360px;
  top: -110px;
  right: -95px;
  background: rgba(var(--v-theme-primary), 0.13);
}

.login-bg-two {
  width: 300px;
  height: 300px;
  bottom: -120px;
  left: -80px;
  background: rgba(120, 180, 160, 0.12);
}

.login-background-brand {
  position: absolute;
  top: clamp(28px, 8vh, 92px);
  left: clamp(32px, 7vw, 120px);
  color: rgba(var(--v-theme-primary), 0.12);
  font-size: clamp(4.5rem, 12vw, 13rem);
  font-weight: 780;
  letter-spacing: 0.06em;
  line-height: 1;
  pointer-events: none;
  user-select: none;
  z-index: 0;
}

.login-container,
.login-row {
  min-height: 100vh;
}

.login-container {
  position: relative;
  z-index: 1;
}

.login-card {
  border: 1px solid rgba(var(--v-border-color), 0.1);
  border-radius: 28px;
  background: rgba(var(--v-theme-surface), 0.76);
  box-shadow: 0 18px 54px rgba(28, 42, 70, 0.1);
  backdrop-filter: blur(18px);
}

.login-card-content {
  padding: 34px;
}

.login-heading {
  margin-bottom: 28px;
  text-align: center;
}

.login-heading h1 {
  margin: 0;
  color: rgb(var(--v-theme-on-surface));
  font-size: clamp(2rem, 4vw, 2.65rem);
  font-weight: 620;
  letter-spacing: -0.035em;
  line-height: 1.15;
}

.login-form {
  display: grid;
  gap: 16px;
}

.login-submit {
  min-height: 52px;
  margin-top: 10px;
  border-radius: 16px;
  box-shadow: 0 12px 28px rgba(var(--v-theme-primary), 0.22);
  font-weight: 620;
  letter-spacing: 0.01em;
}

.login-actions {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 14px;
  align-items: center;
  margin-top: 16px;
}

:deep(.v-field__input),
:deep(.v-label),
:deep(.v-btn),
:deep(.v-list-item-title) {
  font-weight: 450;
  letter-spacing: -0.01em;
}

.theme-button {
  width: 54px;
  height: 54px;
  border-radius: 18px;
  background: rgba(var(--v-theme-surface-variant), 0.28);
  box-shadow: none;
}

.theme-list {
  border-radius: 18px;
}

:deep(.soft-field .v-field),
:deep(.language-select .v-field) {
  border-radius: 16px;
  background: rgba(var(--v-theme-surface-variant), 0.28);
  box-shadow: none;
}

:deep(.soft-field .v-field--focused) {
  box-shadow: 0 0 0 3px rgba(var(--v-theme-primary), 0.13);
}

:deep(.v-field__outline),
:deep(.v-field__overlay) {
  opacity: 0;
}

@media (max-width: 600px) {
  .login-background-brand {
    top: 24px;
    left: 24px;
    font-size: 4rem;
    opacity: 0.55;
  }

  .login-card-content {
    padding: 26px;
  }

  .login-actions {
    grid-template-columns: 1fr;
  }

  .theme-button {
    width: 100%;
  }
}
</style>
