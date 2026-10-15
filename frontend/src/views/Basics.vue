<template>
  <v-row class="basics-save-row">
    <v-col cols="12" justify="center" align="center">
      <v-btn class="basics-save-action" variant="outlined" color="warning" @click="saveConfig" :loading="loading" :disabled="!ready || stateChange">
        {{ $t('actions.save') }}
      </v-btn>
    </v-col>
  </v-row>
  <v-expansion-panels v-if="ready" class="basics-panels">
    <v-expansion-panel>
      <v-expansion-panel-title>
        <v-icon class="basics-panel-icon" icon="mdi-text-box-outline" />
        {{ $t('basic.log.title') }}
      </v-expansion-panel-title>
      <v-expansion-panel-text>
        <v-row>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-switch v-model="appConfig.log.disabled" color="primary" :label="$t('disable')" hide-details></v-switch>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-select
              hide-details
              :label="$t('basic.log.level')"
              :items="levels"
              clearable
              @click:clear="delete appConfig.log.level"
              v-model="appConfig.log.level">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-text-field
              v-model="appConfig.log.output"
              hide-details
              :label="$t('basic.log.output')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-switch v-model="appConfig.log.timestamp" color="primary" :label="$t('basic.log.timestamp')" hide-details></v-switch>
          </v-col>
        </v-row>
      </v-expansion-panel-text>
    </v-expansion-panel>
    <v-expansion-panel>
      <v-expansion-panel-title>
        <v-icon class="basics-panel-icon" icon="mdi-clock-outline" />
        NTP
      </v-expansion-panel-title>
      <v-expansion-panel-text>
        <v-row>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-switch v-model="enableNtp" color="primary" :label="$t('enable')" hide-details></v-switch>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2" v-if="appConfig.ntp?.enabled">
            <v-text-field
              v-model="appConfig.ntp.server"
              hide-details
              :label="$t('out.addr')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2" v-if="appConfig.ntp?.enabled">
            <v-text-field
              v-model="appConfig.ntp.server_port"
              hide-details
              type="number"
              clearable
              @click:clear="delete appConfig.ntp?.server_port"
              :label="$t('out.port')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2" v-if="appConfig.ntp?.enabled">
            <v-text-field
              v-model="ntpInterval"
              hide-details
              :suffix="$t('date.m')"
              min="0"
              type="number"
              :label="$t('ruleset.interval')"
            ></v-text-field>
          </v-col>
        </v-row>
        <Dial :dial="appConfig.ntp" v-if="appConfig.ntp?.enabled" />
      </v-expansion-panel-text>
    </v-expansion-panel>
    <v-expansion-panel>
      <v-expansion-panel-title>
        <v-icon class="basics-panel-icon" icon="mdi-flask-outline" />
        Experimental
      </v-expansion-panel-title>
      <v-expansion-panel-text>
        <v-row>
          <v-col class="v-card-subtitle">Cache File</v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-switch v-model="enableCacheFile" color="primary" :label="$t('enable')" hide-details></v-switch>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2" v-if="appConfig.experimental.cache_file">
            <v-text-field
              v-model="appConfig.experimental.cache_file.path"
              hide-details
              :label="$t('transport.path')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2" v-if="appConfig.experimental.cache_file">
            <v-text-field
              v-model="appConfig.experimental.cache_file.cache_id"
              hide-details
              label="Cache ID"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2" v-if="appConfig.experimental.cache_file">
            <v-switch v-model="appConfig.experimental.cache_file.store_fakeip"
              color="primary"
              :label="$t('basic.exp.storeFakeIp')"
              hide-details></v-switch>
          </v-col>
        </v-row>
        <v-row>
          <v-col class="v-card-subtitle">Clash API</v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-switch v-model="enableClashApi" color="primary" :label="$t('enable')" hide-details></v-switch>
          </v-col>
          <template v-if="appConfig.experimental.clash_api">
            <v-col cols="12" sm="6" md="3" lg="2">
              <v-text-field
                v-model="appConfig.experimental.clash_api.external_controller"
                hide-details
                :label="$t('basic.exp.extController')"
              ></v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="3" lg="2">
              <v-text-field
                v-model="appConfig.experimental.clash_api.secret"
                hide-details
                :label="$t('basic.exp.secret')"
              ></v-text-field>
            </v-col>
          </template>
        </v-row>
        <v-row v-if="appConfig.experimental.clash_api">
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-text-field
              v-model="appConfig.experimental.clash_api.external_ui"
              hide-details
              :label="$t('basic.exp.extUi')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="8" md="4">
            <v-text-field
              v-model="appConfig.experimental.clash_api.external_ui_download_url"
              hide-details
              :label="$t('basic.exp.extUiDownloadUrl')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-select
              v-model="appConfig.experimental.clash_api.external_ui_download_detour"
              hide-details
              :items="outboundTags"
              clearable
              @click:clear="delete appConfig.experimental.clash_api.external_ui_download_detour"
              :label="$t('basic.exp.extUiDownloadDetour')"
            ></v-select>
          </v-col>
        </v-row>
        <v-row v-if="appConfig.experimental.clash_api">
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-text-field
              v-model="appConfig.experimental.clash_api.default_mode"
              hide-details
              :label="$t('basic.exp.defaultMode')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="8" md="4">
            <v-text-field
              v-model="origin"
              hide-details
              :label="$t('basic.exp.allowOrigin') + ' ' + $t('commaSeparated')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-switch v-model="appConfig.experimental.clash_api.access_control_allow_private_network" color="primary" :label="$t('basic.exp.allowPrivate')" hide-details></v-switch>
          </v-col>
        </v-row>
        <v-row>
          <v-col class="v-card-subtitle">V2Ray API</v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-switch v-model="enableV2rayApi" color="primary" :label="$t('enable')" hide-details></v-switch>
          </v-col>
          <template v-if="appConfig.experimental.v2ray_api">
            <v-col cols="12" sm="6" md="3" lg="2">
              <v-text-field
                v-model="appConfig.experimental.v2ray_api.listen"
                hide-details
                :label="$t('objects.listen')"
              ></v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="3" lg="2">
              <v-switch v-model="appConfig.experimental.v2ray_api.stats.enabled"
                color="primary"
                :label="$t('stats.enable')"
                hide-details></v-switch>
            </v-col>
          </template>
        </v-row>
        <v-row v-if="appConfig.experimental.v2ray_api?.stats?.enabled">
          <v-col cols="12" sm="6">
            <v-select
              hide-details
              :label="$t('pages.inbounds')"
              multiple chips closable-chips
              :items="inboundTags"
              v-model="appConfig.experimental.v2ray_api.stats.inbounds">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6">
            <v-select
              hide-details
              :label="$t('pages.outbounds')"
              multiple chips closable-chips
              :items="outboundTags"
              v-model="appConfig.experimental.v2ray_api.stats.outbounds">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6">
            <v-select
              hide-details
              :label="$t('pages.clients')"
              multiple chips closable-chips
              :items="clientNames"
              v-model="appConfig.experimental.v2ray_api.stats.users">
            </v-select>
          </v-col>
        </v-row>
      </v-expansion-panel-text>
    </v-expansion-panel>
  </v-expansion-panels>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import Dial from '@/components/Dial.vue'
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { Config, Ntp } from '@/types/config'
import { FindDiff } from '@/plugins/utils'
import { i18n } from '@/locales'
import { push } from 'notivue'

const oldConfig = ref<any>({})
const loading = ref(false)

const appConfig = ref<Config>({} as Config)
const ready = ref(false)
let active = true
onBeforeUnmount(() => { active = false })

const copyConfig = (config: any): Config => {
  const copy = JSON.parse(JSON.stringify(config))
  copy.log ??= {}
  copy.experimental ??= {}
  return copy
}

// Take one local draft; background store refreshes must not erase unsaved edits.
watch(() => [Data().lastLoad, Data().config], () => {
  if (ready.value || !Data().lastLoad || !Object.keys(Data().config).length) return
  appConfig.value = copyConfig(Data().config)
  oldConfig.value = copyConfig(appConfig.value)
  ready.value = true
}, { immediate: true })

const stateChange = computed(() => {
  return FindDiff.deepCompare(appConfig.value,oldConfig.value)
})

const editableSections = ['log', 'ntp', 'experimental'] as const
const saveConfig = async () => {
  if (!active || !ready.value || loading.value) return
  const draft = copyConfig(appConfig.value)
  const latest = copyConfig(Data().config)
  const changed = editableSections.filter(key => !FindDiff.deepCompare(draft[key], oldConfig.value[key]))
  if (changed.length === 0) return
  // Do not overwrite a concurrently changed section with an older local draft.
  if (changed.some(key => !FindDiff.deepCompare(latest[key], oldConfig.value[key]))) {
    push.error({ title: i18n.global.t('failed'), message: 'Configuration changed elsewhere; reload before saving this section.' })
    return
  }
  const submitted: any = latest
  for (const key of changed) submitted[key] = draft[key]
  loading.value = true
  try {
    const success = await Data().save('config', 'set', submitted)
    if (success && active) {
      const saved = copyConfig(Data().config)
      // Rebase untouched sections even if another section was edited in flight.
      // Otherwise the next save could mistake stale data for a new local edit.
      const rebased: any = copyConfig(saved)
      const baseline: any = copyConfig(saved)
      for (const key of editableSections) {
        if (!FindDiff.deepCompare(appConfig.value[key], draft[key])) {
          rebased[key] = appConfig.value[key]
          // A section edited only after submission may contain an unseen external
          // update. Keep its original baseline so retry detects that conflict.
          if (!FindDiff.deepCompare(draft[key], submitted[key])) baseline[key] = draft[key]
        }
      }
      appConfig.value = rebased
      oldConfig.value = baseline
    }
  } catch {
    if (active) push.error({ title: i18n.global.t('failed') })
  } finally {
    loading.value = false
  }
}

const inboundTags = computed((): string[] => {
  return [...Data().inbounds?.map((i:any) => i.tag), ...Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag)]
})

const clientNames = computed((): string[] => {
  const clients = <any[]>Data().clients
  return clients?.map(c => c.name)
})

const outboundTags = computed((): string[] => {
  return [...Data().outbounds?.map((o:any) => o.tag), ...Data().endpoints?.map((e:any) => e.tag)]
})

const levels = ["trace", "debug", "info", "warn", "error", "fatal", "panic"]

const enableNtp = computed({
  get() { return appConfig.value.ntp?.enabled?? false },
  set(v:boolean) {
    if (v){
      appConfig.value.ntp = <Ntp>{ server: 'time.apple.com', server_port: 123, interval: '30m', ...appConfig.value.ntp, enabled: true }
    } else { appConfig.value.ntp = <Ntp>{}  }
  }
})

const ntpInterval = computed({
  get(): number | null {
    const value = appConfig.value.ntp?.interval
    if (!value) return null
    const text = value.replace(/^\+/, '')
    if (text === '0') return 0
    const parts = [...text.matchAll(/(\d+(?:\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h|d)/g)]
    if (parts.map(part => part[0]).join('') !== text) return null
    const units: Record<string, number> = { ns: 1e-9, us: 1e-6, 'µs': 1e-6, 'μs': 1e-6, ms: 1e-3, s: 1, m: 60, h: 3600, d: 86400 }
    const seconds = parts.reduce((sum, part) => sum + Number(part[1]) * units[part[2]], 0)
    return Number.isFinite(seconds) && seconds <= 9_223_372_036 ? seconds / 60 : null
  },
  set(value: number | string | null) {
    if (!appConfig.value.ntp) return
    const seconds = Number(value) * 60
    if (Number.isFinite(seconds) && seconds >= 1e-9 && seconds <= 9_223_372_036) {
      appConfig.value.ntp.interval = seconds.toFixed(9).replace(/\.?0+$/, '') + 's'
    } else delete appConfig.value.ntp.interval
  },
})

const enableCacheFile = computed({
  get() { return appConfig.value.experimental.cache_file?.enabled?? false },
  set(v:boolean) {
    if (v){
      appConfig.value.experimental.cache_file = { ...appConfig.value.experimental.cache_file, enabled: true }
    } else { delete appConfig.value.experimental.cache_file  }
  }
})

const enableClashApi = computed({
  get() { return appConfig.value.experimental.clash_api != undefined },
  set(v:boolean) {
    if (v) appConfig.value.experimental.clash_api ??= { external_controller: '127.0.0.1:9090' }
    else delete appConfig.value.experimental.clash_api
  }
})

const enableV2rayApi = computed({
  get() { return appConfig.value.experimental.v2ray_api != undefined },
  set(v:boolean) {
    if (v) appConfig.value.experimental.v2ray_api ??= { listen: '127.0.0.1:8080', stats: { enabled: false, inbounds: [], outbounds: [], users: [] }}
    else delete appConfig.value.experimental.v2ray_api
  }
})

const origin = computed({
  get() { return appConfig.value.experimental.clash_api?.access_control_allow_origin &&
    appConfig.value.experimental.clash_api.access_control_allow_origin.length>0 ? appConfig.value.experimental.clash_api.access_control_allow_origin.join(',') : '' },
  set(v:string) {
    if (appConfig.value.experimental.clash_api)
      appConfig.value.experimental.clash_api.access_control_allow_origin = v?.trim() ? v.split(',').map(origin => origin.trim()).filter(Boolean) : undefined
    }
})
</script>

<style scoped>
.basics-save-row {
  margin-bottom: 14px;
}

.basics-save-action {
  min-width: 112px;
}

.basics-panels {
  overflow: hidden;
  border: 1px solid rgba(var(--v-border-color), 0.1);
  border-radius: 20px;
  background: rgba(var(--v-theme-surface), 0.76);
  box-shadow: 0 14px 30px rgba(31, 45, 61, 0.08);
  backdrop-filter: blur(14px);
}

.basics-panels :deep(.v-expansion-panel) {
  background: transparent;
}

.basics-panels :deep(.v-expansion-panel + .v-expansion-panel) {
  border-top: 1px solid rgba(var(--v-border-color), 0.1);
}

.basics-panels :deep(.v-expansion-panel-title) {
  min-height: 62px;
  padding-inline: 20px;
  color: rgba(var(--v-theme-on-surface), 0.9);
  font-size: var(--sui-font-size-title);
  font-weight: 720;
}

.basics-panels :deep(.v-expansion-panel-title__overlay) {
  opacity: 0.04;
}

.basics-panels :deep(.v-expansion-panel-text__wrapper) {
  padding: 8px 20px 20px;
}

.basics-panel-icon {
  margin-inline-end: 12px;
  color: rgb(var(--v-theme-primary));
}
</style>