<template>
  <v-card :loading="loading">
    <v-tabs
    v-model="tab"
    color="primary"
    align-tabs="center"
    show-arrows
  >
    <v-tab value="t1">{{ $t('setting.interface') }}</v-tab>
    <v-tab value="t2">{{ $t('setting.sub') }}</v-tab>
    <v-tab value="t3">{{ $t('setting.jsonSub') }}</v-tab>
    <v-tab value="t4">{{ $t('setting.clashSub') }}</v-tab>
  </v-tabs>
  <v-card-text>
    <v-row align="center" justify="center" style="margin-bottom: 10px;">
      <v-col cols="auto">
        <v-btn color="primary" @click="save" :loading="loading" :disabled="!ready || !stateChange">
          {{ $t('actions.save') }}
        </v-btn>
      </v-col>
      <v-col cols="auto">
        <v-btn variant="outlined" color="warning" @click="restartApp" :loading="loading" :disabled="!ready || stateChange">
          {{ $t('actions.restartApp') }}
        </v-btn>
      </v-col>
      <v-col cols="auto" v-if="!ready && !loading">
        <v-btn variant="outlined" @click="loadData">{{ $t('actions.update') }}</v-btn>
      </v-col>
    </v-row>
    <v-window v-if="ready" v-model="tab">
      <v-window-item value="t1">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webListen" :label="$t('setting.addr')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model.number="webPort" min="1" type="number" :label="$t('setting.port')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webPath" :label="$t('setting.webPath')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webDomain" :label="$t('setting.domain')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webKeyFile" :label="$t('setting.sslKey')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webCertFile" :label="$t('setting.sslCert')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webURI" :label="$t('setting.webUri')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="sessionMaxAge"
              min="0"
              :label="$t('setting.sessionAge')"
              :suffix="$t('date.m')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="trafficAge"
              min="0"
              :label="$t('setting.trafficAge')"
              :suffix="$t('date.d')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.timeLocation" :label="$t('setting.timeLoc')" hide-details></v-text-field>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="t2">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="subEncode" :label="$t('setting.subEncode')" hide-details />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="subShowInfo" :label="$t('setting.subInfo')" hide-details />
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subListen" :label="$t('setting.addr')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="subPort"
              min="1"
              :label="$t('setting.port')"
              hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subKeyFile" :label="$t('setting.sslKey')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subCertFile" :label="$t('setting.sslCert')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subDomain" :label="$t('setting.domain')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subPath" :label="$t('setting.path')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="subUpdates"
              min="0"
              :label="$t('setting.update')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subURI" :label="$t('setting.subUri')" hide-details></v-text-field>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="t3">
        <SubJsonExtVue :settings="settings" />
      </v-window-item>

      <v-window-item value="t4">
        <SubClashExtVue :settings="settings" />
      </v-window-item>
    </v-window>
  </v-card-text>
</v-card>
</template>

<script lang="ts" setup>
import { i18n } from '@/locales'
import { Ref, computed, inject, onMounted, onBeforeUnmount, ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { FindDiff } from '@/plugins/utils'
import SubJsonExtVue from '@/components/SubJsonExt.vue'
import SubClashExtVue from '@/components/SubClashExt.vue'
import { push } from 'notivue'
const tab = ref("t1")
const loading:Ref = inject('loading')?? ref(false)
const oldSettings = ref<any>({})
const ready = ref(false)
let active = true
let loadRequest = 0
let restartTimer: ReturnType<typeof setTimeout> | undefined
let finishRestartWait: (() => void) | undefined
onBeforeUnmount(() => {
  active = false
  loadRequest++
  if (restartTimer !== undefined) clearTimeout(restartTimer)
  finishRestartWait?.()
  loading.value = false
})

const settings = ref({
  webListen: "",
  webDomain: "",
  webPort: "2095",
  webCertFile: "",
  webKeyFile: "",
  webPath: "/app/",
  webURI: "",
  sessionMaxAge: "0",
  trafficAge: "30",
  timeLocation: "Asia/Tehran",
  subListen: "",
  subPort: "2096",
  subPath: "/sub/",
  subDomain: "",
  subCertFile: "",
  subKeyFile: "",
  subUpdates: "12",
  subEncode: "true",
  subShowInfo: "false",
  subURI: "",
  subJsonExt: "",
  subClashExt: "",
})

onMounted(() => { void loadData() })

const copy = (data: any) => JSON.parse(JSON.stringify(data))
const loadData = async () => {
  if (!active) return
  const request = ++loadRequest
  const draft = copy(settings.value)
  loading.value = true
  try {
    const msg = await HttpUtils.get('api/settings')
    if (!active || request !== loadRequest || !msg.success) return
    if (FindDiff.deepCompare(settings.value, draft)) setData(msg.obj)
  } catch {
    if (active && request === loadRequest) push.error({ title: i18n.global.t('failed') })
  } finally {
    if (active && request === loadRequest) loading.value = false
  }
}

const setData = (data: any) => {
  if (!data || typeof data !== 'object' || Array.isArray(data) || Object.keys(data).length === 0) return
  settings.value = copy(data)
  oldSettings.value = copy(data)
  ready.value = true
}

const save = async () => {
  if (!active || !ready.value || loading.value) return
  const submitted = copy(settings.value)
  loading.value = true
  try {
    const msg = await HttpUtils.post('api/save', { object: 'settings', action: 'set', data: JSON.stringify(submitted) })
    if (active && msg.success && msg.obj?.settings) {
      push.success({
        title: i18n.global.t('success'), duration: 5000,
        message: i18n.global.t('actions.set') + ' ' + i18n.global.t('pages.settings'),
      })
      const saved = copy(msg.obj.settings)
      // A response acknowledges the submitted snapshot, not edits made afterwards.
      if (FindDiff.deepCompare(settings.value, submitted)) settings.value = copy(saved)
      oldSettings.value = saved
    }
  } catch {
    if (active) push.error({ title: i18n.global.t('failed') })
  } finally {
    if (active) loading.value = false
  }
}

const sleep = (ms: number) => new Promise<void>(resolve => {
  finishRestartWait = resolve
  restartTimer = setTimeout(() => {
    restartTimer = undefined
    finishRestartWait = undefined
    resolve()
  }, ms)
})

const restartApp = async () => {
  if (!active || !ready.value || loading.value || stateChange.value) return
  loading.value = true
  try {
    // Use saved settings and reject active-content URLs before restarting.
    const saved = copy(oldSettings.value)
    const isTLS = !!saved.webCertFile && !!saved.webKeyFile
    const target = saved.webURI || buildURL(saved.webDomain, String(saved.webPort), isTLS, saved.webPath)
    const url = new URL(target, window.location.href)
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) throw new Error('Invalid panel URL')
    const msg = await HttpUtils.post('api/restartApp', {})
    if (msg.success && active) {
      await sleep(3000)
      if (active) window.location.replace(url.href)
    }
  } catch {
    if (active) push.error({ title: i18n.global.t('failed') })
  } finally {
    if (active) loading.value = false
  }
}

const buildURL = (host: string, port: string, isTLS: boolean, path: string) => {
  host = host || window.location.hostname
  port = port || window.location.port
  // URL requires brackets around literal IPv6 hosts.
  if (host.includes(':') && !host.startsWith('[')) host = '[' + host + ']'
  const protocol = isTLS ? 'https:' : 'http:'
  const portPart = !port || (isTLS && port === '443') || (!isTLS && port === '80') ? '' : ':' + port
  const basePath = '/' + (path || '').replace(/^\/+|\/+$/g, '')
  const normalizedPath = basePath === '/' ? '/' : basePath + '/'
  return new URL(protocol + '//' + host + portPart + normalizedPath + 'settings').href
}

const subEncode = computed({
  get: () => { return settings.value.subEncode == "true" },
  set: (v:boolean) => { settings.value.subEncode = v ? "true" : "false" }
})

const subShowInfo = computed({
  get: () => { return settings.value.subShowInfo == "true" },
  set: (v:boolean) => { settings.value.subShowInfo = v ? "true" : "false" }
})

const webPort = computed({
  get: () => { return settings.value.webPort.length>0 ? parseInt(settings.value.webPort) : 2095 },
  set: (v:number) => { settings.value.webPort = v>0 ? v.toString() : "2095" }
})

const sessionMaxAge = computed({
  get: () => { return settings.value.sessionMaxAge.length>0 ? parseInt(settings.value.sessionMaxAge) : 0 },
  set: (v:number) => { settings.value.sessionMaxAge = v>0 ? v.toString() : "0" }
})

const trafficAge = computed({
  get: () => { return settings.value.trafficAge.length>0 ? parseInt(settings.value.trafficAge) : 0 },
  set: (v:number) => { settings.value.trafficAge = v>0 ? v.toString() : "0" }
})

const subPort = computed({
  get: () => { return settings.value.subPort.length>0 ? parseInt(settings.value.subPort) : 2096 },
  set: (v:number) => { settings.value.subPort = v>0 ? v.toString() : "2096" }
})

const subUpdates = computed({
  get: () => { return settings.value.subUpdates.length>0 ? parseInt(settings.value.subUpdates) : 12 },
  set: (v:number) => { if (Number.isInteger(v) && v >= 0) settings.value.subUpdates = v.toString() }
})

const stateChange = computed(() => {
  return !FindDiff.deepCompare(settings.value,oldSettings.value)
})
</script>
