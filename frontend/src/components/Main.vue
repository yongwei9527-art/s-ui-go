<template>
  <LogVue v-model="logModal.visible" :control="logModal" :visible="logModal.visible" />
  <Backup v-model="backupModal.visible" :control="backupModal" :visible="backupModal.visible" />
  <UsageStats v-model:visible="usageStatsModal.visible" />

  <v-container fluid class="dashboard-page">
    <section class="dashboard-hero">
      <div>
        <v-chip class="hero-status" color="success" variant="flat" size="small" v-if="isSingboxRunning">
            <v-icon icon="mdi-check-circle" start />{{ $t('main.dashboard.runningOk') }}
        </v-chip>
        <v-chip class="hero-status" color="error" variant="flat" size="small" v-else>
            <v-icon icon="mdi-alert-circle" start />{{ $t('main.dashboard.serviceStopped') }}
        </v-chip>
          <h1>{{ $t('main.dashboard.welcome') }}</h1>
          <p>{{ $t('main.dashboard.subtitle') }}</p>
        <div class="hero-meta">
          <span>{{ serverSystemVersion }}</span>
          <span>S-UI v{{ appVersion }}</span>
          <span>{{ $t('main.dashboard.serverIp') }} {{ serverIp }}</span>
        </div>
      </div>
      <div class="hero-mark" aria-hidden="true">S-UI</div>
    </section>

    <v-row class="stats-row" density="compact">
      <v-col cols="12" sm="6" lg="3" v-for="card in statCards" :key="card.title">
        <v-card class="stat-card" elevation="0">
          <div class="stat-icon" :class="card.color">
            <v-icon :icon="card.icon" />
          </div>
          <div class="stat-content">
            <span>{{ card.title }}</span>
            <strong>{{ card.value }}</strong>
            <small>{{ card.caption }}</small>
          </div>
          <v-icon class="stat-spark" :icon="card.spark" />
        </v-card>
      </v-col>
    </v-row>

    <v-row class="dashboard-grid" density="compact">
      <v-col cols="12" lg="6">
        <v-row class="shortcut-row" density="compact">
          <v-col cols="12" sm="6" xl="4">
            <v-card class="shortcut-card" elevation="0" @click="backupModal.visible = true">
              <div class="shortcut-icon green"><v-icon icon="mdi-backup-restore" /></div>
              <div class="shortcut-copy">
                <strong>{{ $t('main.backup.title') }}</strong>
                  <span>{{ $t('main.dashboard.backupHint') }}</span>
              </div>
              <v-icon class="shortcut-arrow" icon="mdi-arrow-right" />
            </v-card>
          </v-col>
          <v-col cols="12" sm="6" xl="4">
            <v-card class="shortcut-card" elevation="0" @click="logModal.visible = true">
              <div class="shortcut-icon purple"><v-icon icon="mdi-list-box-outline" /></div>
              <div class="shortcut-copy">
                <strong>{{ $t('basic.log.title') }}</strong>
                  <span>{{ $t('main.dashboard.logsHint') }}</span>
              </div>
              <v-icon class="shortcut-arrow" icon="mdi-arrow-right" />
            </v-card>
          </v-col>
          <v-col cols="12" sm="6" xl="4">
            <v-card class="shortcut-card" elevation="0" @click="usageStatsModal.visible = true">
              <div class="shortcut-icon orange"><v-icon icon="mdi-chart-box-outline" /></div>
              <div class="shortcut-copy">
                <strong>{{ $t('main.stats.title') }}</strong>
                  <span>{{ $t('main.dashboard.statsHint') }}</span>
              </div>
              <v-icon class="shortcut-arrow" icon="mdi-arrow-right" />
            </v-card>
          </v-col>
        </v-row>

        <v-card class="panel-card recent-card" elevation="0">
          <div class="panel-title">
            <div>
                <h2>{{ $t('main.dashboard.recentLogs') }}</h2>
                <p>{{ $t('main.dashboard.recentLogsHint') }}</p>
            </div>
            <div>
              <v-btn icon="mdi-refresh" size="small" variant="text" :loading="recentLogsLoading" @click="loadRecentLogs" />
                <v-btn size="small" variant="text" color="primary" @click="logModal.visible = true">{{ $t('main.dashboard.viewAll') }}</v-btn>
            </div>
          </div>
          <div class="log-list" v-if="recentLogs.length">
            <div class="log-item" v-for="(log, index) in recentLogs" :key="index">
              <div class="log-dot"><v-icon icon="mdi-file-document-outline" /></div>
              <span>{{ log }}</span>
                <v-chip size="x-small" variant="flat" color="primary">{{ $t('main.dashboard.info') }}</v-chip>
            </div>
          </div>
          <div class="empty-state" v-else>{{ $t('noData') }}</div>
        </v-card>
      </v-col>

      <v-col cols="12" lg="6">
        <v-card class="panel-card status-card" elevation="0">
          <div class="panel-title">
            <div>
                <h2>{{ $t('main.dashboard.systemStatus') }}</h2>
                <p>{{ $t('main.dashboard.systemStatusHint') }}</p>
            </div>
            <v-btn
              icon="mdi-refresh"
              size="small"
              variant="text"
              :loading="loading"
              @click="reloadData"
            />
          </div>

          <div class="status-list">
            <div class="status-row">
                <span><v-icon icon="mdi-server" />{{ $t('main.dashboard.serviceStatus') }}</span>
              <v-chip density="comfortable" variant="flat" :color="isSingboxRunning ? 'success' : 'error'">
                  {{ isSingboxRunning ? $t('main.dashboard.runningOk') : $t('main.dashboard.stopped') }}
              </v-chip>
            </div>
            <div class="status-row">
                <span><v-icon icon="mdi-clock-outline" />{{ $t('main.info.uptime') }}</span>
              <strong>{{ singboxUptime }}</strong>
            </div>
            <div class="status-row">
                <span><v-icon icon="mdi-tag-outline" />{{ $t('version') }}</span>
              <strong>v{{ appVersion }}</strong>
            </div>
            <div class="status-row">
                <span><v-icon icon="mdi-server-network" />{{ $t('main.dashboard.systemVersion') }}</span>
              <strong>{{ serverSystemVersion }}</strong>
            </div>
          </div>

          <div class="resource-list">
            <div class="resource-row" v-for="item in resourceRows" :key="item.title">
              <div>
                <span>{{ item.title }}</span>
                <strong>{{ item.value }}</strong>
              </div>
              <v-progress-linear :model-value="item.percent" :color="item.color" height="9" rounded />
            </div>
          </div>

          <div class="status-actions">
            <v-btn
              color="warning"
              variant="tonal"
              :loading="loading"
              :disabled="!isSingboxRunning"
              @click="restartSingbox"
            >
              <v-icon icon="mdi-restart" start />{{ $t('actions.restartSb') }}
            </v-btn>
          </div>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script lang="ts" setup>
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import Data from '@/store/modules/data'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { i18n } from '@/locales'
import LogVue from '@/layouts/modals/Logs.vue'
import Backup from '@/layouts/modals/Backup.vue'
import UsageStats from '@/layouts/modals/UsageStats.vue'

const dataStore = Data()
const loading = ref(false)
const tilesData = ref(<any>{})
const recentLogs = ref<string[]>([])
const recentLogsLoading = ref(false)
const logModal = ref({ visible: false })
const backupModal = ref({ visible: false })
const usageStatsModal = ref({ visible: false })

const percent = (current?: number, total?: number) => {
  if (!current || !total) return 0
  return Math.min(100, Math.round((current / total) * 100))
}

const formatPercent = (value?: number) => `${Math.round(Number(value) || 0)}%`

const isSingboxRunning = computed(() => Boolean(tilesData.value.sbd?.running))
const onlineUsersCount = computed(() => dataStore.onlines.user?.length ?? 0)
const onlineInboundCount = computed(() => dataStore.onlines.inbound?.length ?? 0)
const onlineOutboundCount = computed(() => dataStore.onlines.outbound?.length ?? 0)
const inboundCount = computed(() => tilesData.value.db?.inbounds ?? dataStore.inbounds.length)
const outboundCount = computed(() => tilesData.value.db?.outbounds ?? dataStore.outbounds.length)
const clientCount = computed(() => tilesData.value.db?.clients ?? dataStore.clients.length)
const totalUsage = computed(() => HumanReadable.sizeFormat((tilesData.value.db?.clientUp ?? 0) + (tilesData.value.db?.clientDown ?? 0)))
const cpuPercent = computed(() => Math.round(Number(tilesData.value.cpu) || 0))
const memoryPercent = computed(() => percent(tilesData.value.mem?.current, tilesData.value.mem?.total))
const diskPercent = computed(() => percent(tilesData.value.dsk?.current, tilesData.value.dsk?.total))
const appVersion = computed(() => tilesData.value.sys?.appVersion ?? '-')
const serverSystemVersion = computed(() => tilesData.value.sys?.systemVersion ?? '-')
const singboxUptime = computed(() => HumanReadable.formatSecond(tilesData.value.sbd?.stats?.Uptime))

const firstIpAddress = (addresses?: unknown) => {
  if (!Array.isArray(addresses)) return ''
  const address = addresses.find((item): item is string => typeof item === 'string' && item.length > 0)
  return address ? address.split('/')[0] : ''
}

const serverIp = computed(() => firstIpAddress(tilesData.value.sys?.ipv4) || firstIpAddress(tilesData.value.sys?.ipv6) || '-')

const statCards = computed(() => [
  {
    title: i18n.global.t('main.dashboard.inboundCount'),
    value: inboundCount.value,
    caption: `${i18n.global.t('objects.outbound')} ${outboundCount.value}`,
    icon: 'mdi-cloud-download-outline',
    spark: 'mdi-trending-up',
    color: 'blue',
  },
  {
    title: i18n.global.t('main.dashboard.onlineUsers'),
    value: onlineUsersCount.value,
    caption: `${i18n.global.t('objects.inbound')} ${onlineInboundCount.value} / ${i18n.global.t('objects.outbound')} ${onlineOutboundCount.value}`,
    icon: 'mdi-account-check-outline',
    spark: 'mdi-chart-line',
    color: 'green',
  },
  {
    title: i18n.global.t('main.dashboard.todayTraffic'),
    value: totalUsage.value,
    caption: `${i18n.global.t('objects.user')} ${clientCount.value}`,
    icon: 'mdi-chart-bell-curve-cumulative',
    spark: 'mdi-pulse',
    color: 'purple',
  },
  {
    title: i18n.global.t('main.dashboard.systemLoad'),
    value: formatPercent(cpuPercent.value),
    caption: `${i18n.global.t('main.info.memory')} ${formatPercent(memoryPercent.value)}`,
    icon: 'mdi-speedometer',
    spark: 'mdi-waveform',
    color: 'orange',
  },
])

const resourceRows = computed(() => [
  { title: i18n.global.t('main.dashboard.cpuUsage'), value: formatPercent(cpuPercent.value), percent: cpuPercent.value, color: 'success' },
  { title: i18n.global.t('main.dashboard.memoryUsage'), value: formatPercent(memoryPercent.value), percent: memoryPercent.value, color: 'primary' },
  { title: i18n.global.t('main.dashboard.diskUsage'), value: formatPercent(diskPercent.value), percent: diskPercent.value, color: 'warning' },
])

const statusRequests = ['sys', 'sbd', 'db', 'cpu', 'mem', 'dsk']

const reloadData = async () => {
  const data = await HttpUtils.get('api/status', { r: statusRequests.join(',') })
  if (data.success) {
    tilesData.value = { ...tilesData.value, ...data.obj }
  }
}

const loadRecentLogs = async () => {
  recentLogsLoading.value = true
  const data = await HttpUtils.get('api/logs', { c: 5, l: 'info' })
  if (data.success) {
    recentLogs.value = data.obj ?? []
  }
  recentLogsLoading.value = false
}

let intervalId: ReturnType<typeof setInterval> | null = null

const startTimer = () => {
  if (intervalId) return
  intervalId = setInterval(() => {
    reloadData()
  }, 4000)
}

const stopTimer = () => {
  if (intervalId) {
    clearInterval(intervalId)
    intervalId = null
  }
}

const restartSingbox = async () => {
  loading.value = true
  await HttpUtils.post('api/restartSb', {})
  await reloadData()
  loading.value = false
}

onMounted(async () => {
  loading.value = true
  await Promise.all([reloadData(), loadRecentLogs()])
  startTimer()
  loading.value = false
})

onBeforeUnmount(() => {
  stopTimer()
})
</script>

<style scoped>
.dashboard-page {
  padding: 0;
}

.dashboard-hero {
  position: relative;
  display: flex;
  justify-content: space-between;
  gap: 24px;
  min-height: 150px;
  margin-bottom: 20px;
  overflow: hidden;
  border: 1px solid rgba(var(--v-border-color), 0.08);
  border-radius: 28px;
  background:
    radial-gradient(circle at 75% 34%, rgba(var(--v-theme-primary), 0.12), transparent 32%),
    linear-gradient(135deg, rgba(var(--v-theme-surface), 0.94), rgba(var(--v-theme-background), 0.78));
  box-shadow: 0 18px 46px rgba(31, 45, 61, 0.08);
  padding: 26px 30px;
}

.dashboard-hero h1 {
  margin: 12px 0 8px;
  color: rgba(var(--v-theme-on-surface), 0.92);
  font-size: clamp(1.45rem, 2.2vw, 2rem);
  font-weight: 800;
  letter-spacing: -0.02em;
}

.dashboard-hero p {
  margin: 0;
  color: rgba(var(--v-theme-on-surface), 0.62);
  font-size: 0.96rem;
}

.hero-status {
  box-shadow: 0 8px 22px rgba(36, 172, 112, 0.18);
}

.hero-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 22px;
  color: rgba(var(--v-theme-on-surface), 0.58);
  font-size: 0.92rem;
}

.hero-meta span {
  border-radius: 999px;
  background: rgba(var(--v-theme-surface), 0.68);
  padding: 7px 12px;
}

.hero-mark {
  align-self: center;
  color: rgba(var(--v-theme-primary), 0.09);
  font-size: clamp(4.2rem, 10vw, 7.6rem);
  font-weight: 900;
  letter-spacing: 0.08em;
  line-height: 0.8;
  white-space: nowrap;
}

.stats-row {
  margin-bottom: 16px;
}

.stat-card,
.shortcut-card,
.panel-card {
  border: 1px solid rgba(var(--v-border-color), 0.08);
  background:
    linear-gradient(145deg, rgba(var(--v-theme-surface), 0.92), rgba(var(--v-theme-surface), 0.78)) !important;
  box-shadow: 0 16px 38px rgba(31, 45, 61, 0.08) !important;
  backdrop-filter: blur(14px);
}

.stat-card {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 118px;
  overflow: hidden;
  border-radius: 22px;
  padding: 16px;
  isolation: isolate;
}

.stat-icon,
.shortcut-icon {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 20px;
}

.stat-icon {
  width: 48px;
  height: 48px;
  margin-inline-end: 14px;
  font-size: 1.45rem;
}

.stat-content {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
  padding-inline-end: 38px;
}

.stat-content span,
.stat-content small {
  overflow: hidden;
  color: rgba(var(--v-theme-on-surface), 0.58);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stat-content strong {
  color: rgba(var(--v-theme-on-surface), 0.94);
  font-size: 1.35rem;
  font-weight: 800;
}

.stat-spark {
  position: absolute;
  right: 16px;
  bottom: 16px;
  color: rgba(var(--v-theme-primary), 0.42);
  font-size: 1.75rem;
  pointer-events: none;
}

.blue { background: rgba(48, 132, 255, 0.12); color: #2f80ed; }
.green { background: rgba(38, 196, 126, 0.13); color: #23a96c; }
.purple { background: rgba(139, 92, 246, 0.13); color: #8b5cf6; }
.orange { background: rgba(255, 124, 54, 0.13); color: #ff7c36; }

.dashboard-grid {
  align-items: stretch;
}

.shortcut-row {
  margin-bottom: 4px;
}

.shortcut-card {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr) 22px;
  align-items: center;
  gap: 12px;
  min-height: 78px;
  overflow: hidden;
  cursor: pointer;
  border-radius: 20px;
  padding: 14px 16px;
  isolation: isolate;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.shortcut-card:hover {
  transform: translateY(-2px);
  border-color: rgba(var(--v-theme-primary), 0.16);
  box-shadow: 0 20px 44px rgba(31, 45, 61, 0.12) !important;
}

.shortcut-icon {
  width: 44px;
  height: 44px;
  margin: 0;
  border-radius: 16px;
  font-size: 1.24rem;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.28);
}

.shortcut-copy {
  display: grid;
  gap: 4px;
  min-width: 0;
  line-height: 1.25;
}

.shortcut-card strong {
  display: block;
  overflow: hidden;
  color: rgba(var(--v-theme-on-surface), 0.9);
  font-size: 0.96rem;
  font-weight: 760;
  letter-spacing: -0.01em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.shortcut-card span {
  display: block;
  overflow: hidden;
  color: rgba(var(--v-theme-on-surface), 0.54);
  font-size: 0.8rem;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.shortcut-arrow {
  justify-self: end;
  color: rgba(var(--v-theme-primary), 0.56);
  font-size: 1.05rem;
  opacity: 0.72;
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.shortcut-card:hover .shortcut-arrow {
  opacity: 1;
  transform: translateX(2px);
}

.panel-card {
  border-radius: 24px;
  padding: 18px;
}

.recent-card {
  margin-top: 16px;
}

.panel-title,
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
}

.panel-title h2,
.section-heading h2 {
  margin: 0 0 4px;
  color: rgba(var(--v-theme-on-surface), 0.9);
  font-size: 1.05rem;
  font-weight: 760;
}

.panel-title p,
.section-heading p {
  margin: 0;
  color: rgba(var(--v-theme-on-surface), 0.52);
  font-size: 0.88rem;
}

.log-list,
.status-list,
.resource-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.log-item {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 48px;
  border-radius: 16px;
  background: rgba(var(--v-theme-background), 0.54);
  padding: 10px 12px;
}

.log-item span {
  flex: 1;
  overflow: hidden;
  color: rgba(var(--v-theme-on-surface), 0.72);
  font-size: 0.88rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.log-dot {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 12px;
  background: rgba(var(--v-theme-primary), 0.1);
  color: rgb(var(--v-theme-primary));
}

.status-card {
  height: 100%;
}

.status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 46px;
  border-bottom: 1px solid rgba(var(--v-border-color), 0.08);
}

.status-row span {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: rgba(var(--v-theme-on-surface), 0.62);
}

.status-row strong {
  overflow: hidden;
  color: rgba(var(--v-theme-on-surface), 0.86);
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-list {
  margin-top: 20px;
}

.resource-row > div {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
  color: rgba(var(--v-theme-on-surface), 0.64);
  font-size: 0.92rem;
}

.resource-row strong {
  color: rgba(var(--v-theme-on-surface), 0.86);
}

.status-actions {
  margin-top: 22px;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 110px;
  gap: 10px;
  border-radius: 20px;
  color: rgba(var(--v-theme-on-surface), 0.52);
}

@media (max-width: 960px) {
  .dashboard-hero {
    flex-direction: column;
  }

  .hero-mark {
    align-self: flex-start;
  }
}

@media (max-width: 600px) {
  .dashboard-hero,
  .panel-card {
    border-radius: 20px;
    padding: 20px;
  }

  .panel-title,
  .section-heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .stat-card {
    min-height: 116px;
  }

  .shortcut-card {
    grid-template-columns: 42px minmax(0, 1fr);
    min-height: 72px;
    padding: 12px 14px;
  }

  .shortcut-arrow {
    display: none;
  }
}
</style>
