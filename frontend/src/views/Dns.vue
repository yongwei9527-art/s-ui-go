<template>
  <template v-if="ready">
  <DnsVue
    v-model="dnsModal.visible"
    :visible="dnsModal.visible"
    :index="dnsModal.index"
    :data="dnsModal.data"
    :tsTags="tsTags"
    :rslvdTags="rslvdTags"
    @close="closeDnsModal"
    @save="saveDnsModal"
  />
  <DnsRuleVue
    v-model="dnsRuleModal.visible"
    :visible="dnsRuleModal.visible"
    :index="dnsRuleModal.index"
    :data="dnsRuleModal.data"
    :clients="clients"
    :inTags="inboundTags"
    :serverTags="dnsServerTags"
    :ruleSets="ruleSets"
    @close="closeDnsRuleModal"
    @save="saveDnsRuleModal"
  />
  <v-row>
    <v-col cols="12" justify="center" align="center">
      <v-btn color="primary" @click="showDnsModal(-1)" style="margin: 0 5px;">{{ $t('dns.add') }}</v-btn>
      <v-btn color="primary" @click="showDnsRuleModal(-1)" style="margin: 0 5px;">{{ $t('dns.rule.add') }}</v-btn>
      <v-btn variant="outlined" color="warning" @click="saveConfig" :loading="loading" :disabled="!ready || stateChange">
        {{ $t('actions.save') }}
      </v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col class="v-card-subtitle" cols="12">{{ $t('pages.basics') }}</v-col>
    <v-col cols="12">
      <v-row>
        <v-col cols="12" sm="6" md="3" lg="2">
          <v-select
            hide-details
            :label="$t('dns.final')"
            :items="[ {title: $t('dns.firstServer'), value: ''}, ...dnsServerTags]"
            v-model="finalDns">
          </v-select>
        </v-col>
        <v-col cols="12" sm="6" md="3" lg="2">
          <v-select
            hide-details
            :label="$t('dns.domainStrategy')"
            clearable
            @click:clear="delete dns.strategy"
            :items="['prefer_ipv4','prefer_ipv6','ipv4_only','ipv6_only']"
            v-model="dns.strategy">
          </v-select>
        </v-col>
        <v-col cols="12" sm="6" md="3" lg="2">
          <v-text-field
            v-model="dns.client_subnet" hide-details
            clearable @click:clear="delete dns.client_subnet"
            :label="$t('dns.rule.action.clientSubnet')"></v-text-field>
        </v-col>
        <v-col cols="auto">
          <v-text-field
            v-model.number="dns.cache_capacity"
            type="number" min="1024" hide-details
            clearable @click:clear="delete dns.cache_capacity"
            :label="$t('dns.cacheCapacity')"></v-text-field>
        </v-col>
        <v-col cols="auto">
          <v-checkbox v-model="dns.disable_cache" hide-details :label="$t('dns.disableCache')" />
        </v-col>
        <v-col cols="auto">
          <v-checkbox v-model="dns.disable_expire" hide-details :label="$t('dns.disableExpire')" />
        </v-col>
        <v-col cols="auto">
          <v-checkbox v-model="dns.independent_cache" hide-details :label="$t('dns.independentCache')" />
        </v-col>
        <v-col cols="auto">
          <v-checkbox v-model="dns.reverse_mapping" hide-details :label="$t('dns.reverseMapping')" />
        </v-col>
      </v-row>
    </v-col>
  </v-row>
  <v-row class="dns-server-card-row" align="stretch">
    <v-col class="v-card-subtitle dns-section-title" cols="12">{{ $t('dns.title') }}</v-col>
    <v-col class="dns-server-card-col" cols="12" sm="6" md="4" lg="3" xl="2" v-for="(item, index) in <any[]>dns.servers" :key="item.tag ?? index">
      <v-card class="dns-server-card" rounded="xl" elevation="5" :title="item.tag">
        <v-card-subtitle style="margin-top: -15px;">
          <v-row>
            <v-col>{{ item.type }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('dns.server') }}</v-col>
            <v-col>
              {{ item.server?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('in.port') }}</v-col>
            <v-col>
              {{ item.server_port?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('objects.tls') }}</v-col>
            <v-col>
              {{ dnsTlsStatus(item) }}
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-file-edit" @click="showDnsModal(index)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove" style="margin-inline-start:0;" color="warning" @click="delDnsOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="delDnsOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="delDns(index)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delDnsOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
  <v-row class="dns-server-card-row" align="stretch">
    <v-col class="v-card-subtitle dns-section-title" cols="12">{{ $t('dns.rule.title') }}</v-col>
    <v-col class="dns-server-card-col" cols="12" sm="6" md="4" lg="3" xl="2" v-for="(item, index) in <any[]>dnsRules"
      :key="`${index}-${item.server ?? item.action ?? 'rule'}`"
      :draggable="true"
      @dragstart="onDragStart(index)" @dragend="onDragEnd"
      @dragover.prevent
      @drop="onDrop(index)"
      >
      <v-card class="dns-server-card" rounded="xl" elevation="5" :title="String(index + 1)">
        <v-card-subtitle style="margin-top: -15px;">
          <v-row>
            <v-col>{{ item.type != undefined ? $t('rule.logical') + ' (' + item.mode + ')' : $t('rule.simple') }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('admin.action') }}</v-col>
            <v-col>
              {{ item.action }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('dns.server') }}</v-col>
            <v-col>
              {{ item.server?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('pages.rules') }}</v-col>
            <v-col>
              {{ item.rules ? item.rules.length : Object.keys(item).filter(r => !actionDnsRuleKeys.includes(r)).length }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('rule.invert') }}</v-col>
            <v-col>
              {{ $t( (item.invert?? false)? 'yes' : 'no') }}
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-file-edit" @click="showDnsRuleModal(index)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove" style="margin-inline-start:0;" color="warning" @click="delDnsRuleOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="delDnsRuleOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="delDnsRule(index)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delDnsRuleOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
  </template>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import DnsVue from '@/layouts/modals/Dns.vue'
import DnsRuleVue from '@/layouts/modals/DnsRule.vue'
import { Config } from '@/types/config'
import { actionDnsRuleKeys, dnsRule } from '@/types/dns'
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
  copy.dns ??= {}
  copy.dns.servers ??= []
  copy.dns.rules ??= []
  return copy
}

// Take one local draft; background store refreshes must not erase unsaved edits.
watch(() => [Data().lastLoad, Data().config], () => {
  if (ready.value || !Data().lastLoad || !Object.keys(Data().config).length) return
  appConfig.value = copyConfig(Data().config)
  oldConfig.value = copyConfig(appConfig.value)
  ready.value = true
}, { immediate: true })

const tsTags = computed((): string[] => {
  return Data().endpoints?.filter((e:any) => e.type == "tailscale").map((e:any) => e.tag)
})

const rslvdTags = computed((): string[] => {
  return Data().services?.filter((e:any) => e.type == "resolved").map((e:any) => e.tag)
})

const clients = computed((): string[] => {
  return Data().clients.map((c:any) => c.name)
})

const stateChange = computed(() => {
  return FindDiff.deepCompare(appConfig.value.dns,oldConfig.value.dns)
    && FindDiff.deepCompare(appConfig.value.route,oldConfig.value.route)
})

const editableSections = ['dns', 'route'] as const
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
  return [
    ...(Data().inbounds?.map((o:any) => o.tag) ?? []),
    ...(Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag) ?? []),
  ]
})

const dns = computed((): any => {
  return appConfig.value.dns
})

const dnsTlsStatus = (server: any) => {
  if (['tls', 'https', 'quic', 'h3'].includes(server.type)) return i18n.global.t('enable')
  return Object.hasOwn(server, 'tls') ? i18n.global.t(server.tls?.enabled ? 'enable' : 'disable') : '-'
}

const dnsServerTags = computed((): string[] => {
  return dns.value?.servers?.filter((s:any) => s.tag && s.tag != "")?.map((s:any) => s.tag) ?? []
})

const finalDns = computed({
  get() { return dns.value?.final?? '' },
  set(v:string | null | undefined) { dns.value.final = v && v.length > 0 ? v : undefined }
})


const dnsRules = computed((): dnsRule[] => {
  return <dnsRule[]>(dns.value?.rules ?? [])
})

const ruleSets = computed((): string[] => {
  return appConfig.value?.route?.rule_set?.map((r:any) => r.tag) ?? []
})

let delDnsOverlay = ref(new Array<boolean>)
let delDnsRuleOverlay = ref(new Array<boolean>)
// Confirmations refer to positions; invalidate them before those positions move.
watch(() => dns.value?.servers?.slice(), () => { delDnsOverlay.value = [] }, { flush: 'sync' })
watch(() => dnsRules.value.slice(), () => { delDnsRuleOverlay.value = [] }, { flush: 'sync' })
let dnsEditTarget: any = null
let dnsRuleEditTarget: dnsRule | null = null

const dnsModal = ref({
  visible: false,
  index: -1,
  data: "",
})

const showDnsModal = (index: number) => {
  if (!Number.isInteger(index) || index < -1 || (index >= 0 && !dns.value.servers[index])) return
  dnsEditTarget = index === -1 ? null : dns.value.servers[index]
  dnsModal.value.index = index
  dnsModal.value.data = index == -1 ? '' : JSON.stringify(dns.value.servers[index])
  dnsModal.value.visible = true
}

const closeDnsModal = () => {
  dnsModal.value.visible = false
}

const saveDnsModal = (data:any) => {
  // New or Edit
  if (dnsModal.value.index == -1) {
    dns.value.servers.push(data)
  } else {
    const index = dns.value.servers.indexOf(dnsEditTarget)
    if (index < 0) { closeDnsModal(); return }
    dns.value.servers[index] = data
  }
  dnsModal.value.visible = false
}

const delDns = (index: number) => {
  dns.value.servers.splice(index,1)
  delDnsOverlay.value[index] = false
}

const dnsRuleModal = ref({
  visible: false,
  index: -1,
  data: "",
})

const showDnsRuleModal = (index: number) => {
  if (!Number.isInteger(index) || index < -1 || (index >= 0 && !dnsRules.value[index])) return
  dnsRuleEditTarget = index === -1 ? null : dnsRules.value[index]
  dnsRuleModal.value.index = index
  dnsRuleModal.value.data = index == -1 ? '' : JSON.stringify(dnsRules.value[index])
  dnsRuleModal.value.visible = true
}

const closeDnsRuleModal = () => {
  dnsRuleModal.value.visible = false
}

const saveDnsRuleModal = (data:dnsRule) => {
  // New or Edit
  if (dnsRuleModal.value.index == -1) {
    dnsRules.value.push(data)
  } else {
    const index = dnsRuleEditTarget === null ? -1 : dnsRules.value.indexOf(dnsRuleEditTarget)
    if (index < 0) { closeDnsRuleModal(); return }
    dnsRules.value[index] = data
  }
  dnsRuleModal.value.visible = false
}

const delDnsRule = (index: number) => {
  dnsRules.value.splice(index,1)
  delDnsRuleOverlay.value[index] = false
}

let draggedRule: any = null
const onDragStart = (index: number) => {
  draggedRule = Number.isInteger(index) ? dnsRules.value[index] ?? null : null
}
const onDragEnd = () => { draggedRule = null }
const onDrop = (index: number) => {
  const source = dnsRules.value.indexOf(draggedRule)
  onDragEnd()
  if (source < 0 || !Number.isInteger(index) || index < 0 || index >= dnsRules.value.length) return
  const [rule] = dnsRules.value.splice(source, 1)
  dnsRules.value.splice(index, 0, rule)
}

</script>

<style scoped>
.dns-server-card-row {
  row-gap: 16px;
}

.dns-section-title {
  margin-bottom: 8px;
}

.dns-server-card-col {
  display: flex;
  min-width: 0;
}

.dns-server-card {
  height: 100%;
  width: 100%;
  min-width: 0;
  overflow: hidden;
}

.dns-server-card :deep(.v-card-text .v-col) {
  min-width: 0;
  overflow-wrap: anywhere;
}

.dns-server-card :deep(.v-card-title) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .dns-server-card-row {
    row-gap: 18px;
  }
}
</style>