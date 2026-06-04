<template>
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
  <v-dialog v-model="leakGuardConfirm.visible" max-width="560">
    <v-card rounded="xl">
      <v-card-title>{{ $t('dns.applyLeakGuardConfirmTitle') }}</v-card-title>
      <v-divider />
      <v-card-text>
        <p class="mb-2">{{ $t('dns.applyLeakGuardConfirmDesc') }}</p>
        <v-alert color="warning" icon="mdi-alert" variant="tonal" rounded="lg">
          {{ $t('dns.applyLeakGuardConfirmWarning') }}
        </v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="outlined" color="secondary" @click="leakGuardConfirm.visible = false">{{ $t('no') }}</v-btn>
        <v-btn color="primary" variant="flat" @click="confirmApplyLeakGuardTemplate">{{ $t('yes') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
  <v-row>
    <v-col cols="12">
      <v-alert
        color="primary"
        icon="mdi-shield-check"
        variant="tonal"
        rounded="lg"
      >
        <div class="dns-leak-toolbar">
          <div class="dns-leak-copy">
            <div class="text-subtitle-1 font-weight-bold">{{ $t('dns.leakGuard') }}</div>
            <div class="text-body-2">{{ $t('dns.leakGuardDesc') }}</div>
            <div class="text-caption text-medium-emphasis mt-1">{{ leakGuardModeHelp }}</div>
            <div class="text-caption text-medium-emphasis mt-1">{{ $t('dns.leakGuardSubscriptionNote') }}</div>
          </div>
          <div class="dns-leak-actions">
            <v-select
              v-model="dnsLeakGuardMode"
              :items="dnsLeakGuardModes"
              :label="$t('dns.leakGuardMode')"
              density="compact"
              hide-details
              class="dns-mode-select"
            />
            <v-btn
              color="primary"
              variant="flat"
              prepend-icon="mdi-shield-sync"
              :disabled="dnsLeakGuardMode == 'off'"
              @click="requestApplyLeakGuardTemplate">
              {{ $t('dns.applyLeakGuard') }}
            </v-btn>
          </div>
        </div>
      </v-alert>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12">
      <v-card class="leak-check-card" rounded="xl" elevation="0">
        <div class="leak-check-header">
          <div>
            <div class="text-subtitle-1 font-weight-bold">{{ $t('dns.leakGuardCheck') }}</div>
            <div class="text-body-2 text-medium-emphasis">{{ $t('dns.leakGuardCheckDesc') }}</div>
          </div>
          <div class="leak-check-actions">
            <v-chip :color="leakGuardStatusColor" variant="flat" density="comfortable">
              {{ leakGuardStatusText }}
            </v-chip>
            <v-btn
              icon="mdi-refresh"
              variant="text"
              size="small"
              :loading="leakGuardCheckLoading"
              @click="loadLeakGuardReport"
            />
          </div>
        </div>
        <v-progress-linear v-if="leakGuardCheckLoading" indeterminate color="primary" />
        <div class="leak-check-list" v-if="leakGuardChecks.length">
          <div class="leak-check-item" v-for="check in leakGuardChecks" :key="check.key">
            <v-icon :color="check.passed ? 'success' : check.severity == 'error' ? 'error' : 'warning'"
              :icon="check.passed ? 'mdi-check-circle' : check.severity == 'error' ? 'mdi-close-circle' : 'mdi-alert-circle'" />
            <span>{{ $t(`dns.leakGuardCheckItems.${check.key}`) }}</span>
            <v-chip
              v-if="!check.passed"
              :color="check.severity == 'error' ? 'error' : 'warning'"
              variant="tonal"
              density="compact"
              size="small"
            >
              {{ check.severity == 'error' ? $t('dns.severityError') : $t('dns.severityWarning') }}
            </v-chip>
          </div>
        </div>
      </v-card>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" justify="center" align="center">
      <v-btn color="primary" @click="showDnsModal(-1)" style="margin: 0 5px;">{{ $t('dns.add') }}</v-btn>
      <v-btn color="primary" @click="showDnsRuleModal(-1)" style="margin: 0 5px;">{{ $t('dns.rule.add') }}</v-btn>
      <v-btn variant="outlined" color="warning" @click="saveConfig" :loading="loading" :disabled="stateChange">
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
              {{ Object.hasOwn(item,'tls') ? $t(item.tls?.enabled ? 'enable' : 'disable') : '-'  }}
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
      @dragstart="onDragStart(index)"
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

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, ref, onBeforeMount } from 'vue'
import DnsVue from '@/layouts/modals/Dns.vue'
import DnsRuleVue from '@/layouts/modals/DnsRule.vue'
import { Config } from '@/types/config'
import { actionDnsRuleKeys, dnsRule } from '@/types/dns'
import { FindDiff } from '@/plugins/utils'
import { push } from 'notivue'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'

const oldConfig = ref(<any>{})
const loading = ref(false)
const settings = ref(<any>{})
const dnsLeakGuardMode = ref('recommended')
const oldDnsLeakGuardMode = ref('recommended')
const leakGuardCheckLoading = ref(false)
const leakGuardReport = ref<any>(null)
const leakGuardConfirm = ref({ visible: false })

const leakGuardStatusColor = computed(() => {
  if (!leakGuardReport.value) return 'default'
  if (leakGuardReport.value.status == 'passed') return 'success'
  if (leakGuardReport.value.status == 'failed') return 'error'
  return 'warning'
})

const leakGuardStatusText = computed(() => {
  if (!leakGuardReport.value) return i18n.global.t('loading')
  if (leakGuardReport.value.status == 'passed') return i18n.global.t('dns.leakGuardPassed')
  if (leakGuardReport.value.status == 'failed') return i18n.global.t('dns.leakGuardFailed')
  return i18n.global.t('dns.leakGuardWarning')
})

const dnsLeakGuardModes = computed(() => [
  { title: i18n.global.t('dns.leakGuardModes.off'), value: 'off' },
  { title: i18n.global.t('dns.leakGuardModes.recommended'), value: 'recommended' },
  { title: i18n.global.t('dns.leakGuardModes.strict'), value: 'strict' },
])

const leakGuardModeHelp = computed(() => i18n.global.t(`dns.leakGuardModeHelp.${dnsLeakGuardMode.value}`))
const leakGuardChecks = computed(() => leakGuardReport.value?.checks ?? [])

const appConfig = computed((): Config => {
  return <Config> Data().config
})

onBeforeMount( async () => {
  // fix old configs
  if (!appConfig.value.dns) appConfig.value.dns = { servers: [], rules: [] }
  if (!appConfig.value.dns.servers) appConfig.value.dns.servers = []
  if (!appConfig.value.dns.rules) appConfig.value.dns.rules = []
  if (!appConfig.value.route) appConfig.value.route = { rules: [], rule_set: [], default_domain_resolver: '' }
  if (!appConfig.value.route.rules) appConfig.value.route.rules = []
  if (!appConfig.value.route.rule_set) appConfig.value.route.rule_set = []

  loading.value = true
  while (Data().lastLoad == 0) {
    await new Promise(resolve => setTimeout(resolve, 100))
  }
  oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  await loadSettings()
  await loadLeakGuardReport()
  loading.value = false
})

const loadSettings = async () => {
  const msg = await HttpUtils.get('api/settings')
  if (!msg.success) return
  settings.value = msg.obj ?? {}
  dnsLeakGuardMode.value = settings.value.dnsLeakGuardMode ?? 'recommended'
  oldDnsLeakGuardMode.value = dnsLeakGuardMode.value
}

const loadLeakGuardReport = async () => {
  leakGuardCheckLoading.value = true
  try {
    const msg = await HttpUtils.get('api/dnsLeakGuardCheck')
    leakGuardReport.value = msg.success ? msg.obj : null
  } finally {
    leakGuardCheckLoading.value = false
  }
}

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
    && dnsLeakGuardMode.value == oldDnsLeakGuardMode.value
})

const saveConfig = async () => {
  loading.value = true
  let success = true
  const modeChanged = dnsLeakGuardMode.value != oldDnsLeakGuardMode.value
  if (modeChanged) {
    settings.value.dnsLeakGuardMode = dnsLeakGuardMode.value
    const msg = await HttpUtils.post('api/save', { object: 'settings', action: 'set', data: JSON.stringify(settings.value) })
    success = msg.success
    if (success) oldDnsLeakGuardMode.value = dnsLeakGuardMode.value
  }
  if (success) {
    const dnsChanged = !FindDiff.deepCompare(appConfig.value.dns,oldConfig.value.dns)
    const routeChanged = !FindDiff.deepCompare(appConfig.value.route,oldConfig.value.route)
    if (dnsChanged || routeChanged || modeChanged) {
      success = await Data().save("config", "set", appConfig.value)
    }
  }
  if (success) {
    oldConfig.value = JSON.parse(JSON.stringify(Data().config))
    await loadLeakGuardReport()
  }
  loading.value = false
}

const requestApplyLeakGuardTemplate = () => {
  if (dnsLeakGuardMode.value == 'off') return
  leakGuardConfirm.value.visible = true
}

const confirmApplyLeakGuardTemplate = () => {
  leakGuardConfirm.value.visible = false
  applyLeakGuardTemplate()
}

const applyLeakGuardTemplate = () => {
  if (dnsLeakGuardMode.value == 'off') return
  dns.value.servers = [
    {
      tag: 'remote-dns',
      type: 'tls',
      server: '1.1.1.1',
      server_port: 853,
      detour: 'direct',
      tls: {},
    },
    {
      tag: 'local-dns',
      type: 'udp',
      server: '223.5.5.5',
      server_port: 53,
      detour: 'direct',
    },
  ]
  dns.value.rules = [
    {
      domain_suffix: ['.lan', '.local'],
      action: 'route',
      server: 'local-dns',
    },
  ]
  dns.value.final = 'remote-dns'
  dns.value.strategy = 'prefer_ipv4'
  dns.value.disable_cache = false
  dns.value.disable_expire = false
  dns.value.independent_cache = true
  dns.value.reverse_mapping = true
  if (!appConfig.value.route) appConfig.value.route = { rules: [], rule_set: [], default_domain_resolver: '' }
  if (!appConfig.value.route.rules) appConfig.value.route.rules = []
  appConfig.value.route.auto_detect_interface = true
  const rules = appConfig.value.route.rules as any[]
  appConfig.value.route.rules = insertDNSHijackRules(rules)
  if (dnsLeakGuardMode.value == 'strict') {
    appConfig.value.route.default_domain_resolver = 'remote-dns'
  } else if (!appConfig.value.route.default_domain_resolver) {
    appConfig.value.route.default_domain_resolver = 'remote-dns'
  }
  push.success(i18n.global.t('dns.leakGuardApplied'))
}

const hasDNSProtocolHijack = (rules: any[]) => {
  return rules.some((rule: any) => rule.action == 'hijack-dns' && (
    rule.protocol == 'dns' || rule.protocol?.includes?.('dns')
  ))
}

const hasPort53Hijack = (rules: any[]) => {
  let tcpCovered = false
  let udpCovered = false
  for (const rule of rules) {
    if (rule.action != 'hijack-dns' || !(rule.port == 53 || rule.port?.includes?.(53))) continue
    if (!rule.network) return true
    if (hasRuleValue(rule.network, 'tcp')) tcpCovered = true
    if (hasRuleValue(rule.network, 'udp')) udpCovered = true
    if (tcpCovered && udpCovered) return true
  }
  return false
}

const hasRuleValue = (value: any, expected: string) => {
  if (typeof value == 'string') return value == expected
  return Array.isArray(value) && value.includes(expected)
}

const insertDNSHijackRules = (rules: any[]) => {
  const additions = []
  if (!hasDNSProtocolHijack(rules)) additions.push({ protocol: ['dns'], action: 'hijack-dns' })
  if (!hasPort53Hijack(rules)) additions.push({ port: 53, network: ['tcp', 'udp'], action: 'hijack-dns' })
  if (additions.length == 0) return rules
  if (rules[0]?.action == 'sniff') {
    return [rules[0], ...additions, ...rules.slice(1)]
  }
  return [...additions, ...rules]
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

const dnsServerTags = computed((): string[] => {
  return dns.value?.servers?.filter((s:any) => s.tag && s.tag != "")?.map((s:any) => s.tag) ?? []
})

const finalDns = computed({
  get() { return dns.value?.final?? '' },
  set(v:string | null | undefined) { dns.value.final = v && v.length > 0 ? v : undefined }
})


const dnsRules = computed((): dnsRule[] => {
  return <dnsRule[]>dns.value.rules
})

const ruleSets = computed((): string[] => {
  return appConfig.value?.route?.rule_set?.map((r:any) => r.tag) ?? []
})

let delDnsOverlay = ref(new Array<boolean>)
let delDnsRuleOverlay = ref(new Array<boolean>)

const dnsModal = ref({
  visible: false,
  index: -1,
  data: "",
})

const showDnsModal = (index: number) => {
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
    dns.value.servers[dnsModal.value.index] = data
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
    dnsRules.value[dnsRuleModal.value.index] = data
  }
  dnsRuleModal.value.visible = false
}

const delDnsRule = (index: number) => {
  dnsRules.value.splice(index,1)
  delDnsRuleOverlay.value[index] = false
}

const draggedItemIndex = ref(null)

const onDragStart = (index: any) => {
  draggedItemIndex.value = index
}

const onDrop = (index: any) => {
  if (draggedItemIndex.value !== null) {
    // Swap the dragged item with the dropped one
    const draggedItem = dnsRules.value[draggedItemIndex.value]
    dnsRules.value.splice(draggedItemIndex.value, 1)
    dnsRules.value.splice(index, 0, draggedItem)
    draggedItemIndex.value = null
  }
}
</script>

<style scoped>
.dns-leak-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.dns-leak-copy {
  min-width: 0;
  flex: 1 1 auto;
}

.dns-leak-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 0 0 auto;
}

.dns-mode-select {
  width: 240px;
}

.leak-check-card {
  border: 1px solid rgba(var(--v-border-color), 0.1);
  background: rgba(var(--v-theme-surface), 0.72);
  padding: 16px;
}

.leak-check-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.leak-check-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.leak-check-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
  margin-top: 14px;
}

.leak-check-item {
  display: flex;
  align-items: center;
  gap: 8px;
  border-radius: 12px;
  background: rgba(var(--v-theme-background), 0.52);
  padding: 9px 10px;
  font-size: 0.9rem;
}

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
  .dns-leak-toolbar,
  .dns-leak-actions,
  .leak-check-header,
  .leak-check-actions {
    align-items: stretch;
    flex-direction: column;
    width: 100%;
  }

  .dns-mode-select {
    width: 100%;
  }

  .dns-server-card-row {
    row-gap: 18px;
  }
}
</style>