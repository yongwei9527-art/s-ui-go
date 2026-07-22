<template>
  <section class="xhttp-page">
    <v-card rounded="xl" class="mb-5" elevation="0">
      <v-card-title class="d-flex align-center ga-3 flex-wrap">
        <v-icon color="primary">mdi-lan-connect</v-icon>
        <span>Xray · VLESS + Reality + XHTTP</span>
        <v-chip :color="status.running ? 'success' : 'warning'" size="small" variant="tonal">
          {{ status.running ? '侧车运行中' : '侧车未运行' }}
        </v-chip>
      </v-card-title>
      <v-card-text>
        <v-alert type="info" variant="tonal" density="compact" class="mb-3">
          XHTTP 由独立 Xray-core 侧车提供，不会写入 sing-box 入站。启用前请安装 Xray-core，并设置环境变量 <code>SUI_XRAY_BINARY</code> 指向 xray 可执行文件；默认使用独立 TCP 端口 2088。
        </v-alert>
        <v-alert type="warning" variant="tonal" density="compact" class="mb-3">
          当前侧车流量不会计入 S-UI 的上传/下载统计或配额；同时它使用独立的 Xray 直连出站，不会自动继承 sing-box 的路由与 DNS 策略。需要严格限额或分流时，请先完成额外的流量统计与策略配置。
        </v-alert>
        <div class="text-body-2">
          二进制：<code>{{ status.binary || '未发现' }}</code>
          <span v-if="status.error" class="text-error ml-2">{{ status.error }}</span>
          <span v-if="status.sidecarError" class="text-error ml-2">侧车异常退出：{{ status.sidecarError }}</span>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-btn color="primary" variant="tonal" prepend-icon="mdi-restart" :loading="loading" @click="restart">重启 Xray 侧车</v-btn>
        <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
      </v-card-actions>
    </v-card>

    <v-row>
      <v-col cols="12" md="7">
        <v-card rounded="xl" elevation="0">
          <v-card-title>{{ editing ? '编辑 XHTTP 入站' : '新建 XHTTP 入站' }}</v-card-title>
          <v-card-text>
            <v-row>
              <v-col cols="12" sm="4"><v-switch v-model="form.enable" color="primary" label="启用" hide-details /></v-col>
              <v-col cols="12" sm="8"><v-text-field v-model="form.tag" label="标签" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.listen" label="监听地址" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model.number="form.port" type="number" min="1" max="65535" label="TCP 端口" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.serverName" label="Reality Server Name / SNI" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.dest" label="Reality destination (host:port)" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.path" label="XHTTP path" /></v-col>
              <v-col cols="12" sm="6"><v-select v-model="form.mode" :items="modes" label="XHTTP mode" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.shortId" label="Reality short ID（十六进制）" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.fingerprint" label="客户端指纹" /></v-col>
              <v-col cols="12"><v-text-field v-model="form.privateKey" type="password" label="Reality private key（仅服务器）" hint="编辑已有入站时留空即可保留现有私钥；私钥不会由接口回显。" persistent-hint /></v-col>
              <v-col cols="12"><v-text-field v-model="form.publicKey" label="Reality public key（订阅链接使用）" /></v-col>
              <v-col cols="12">
                <v-select v-model="clientIds" :items="clientOptions" item-title="title" item-value="value" label="分配给 S-UI 客户端" multiple chips closable-chips hint="每位客户端会获得独立 VLESS UUID，并自动写入其订阅。" persistent-hint />
              </v-col>
            </v-row>
          </v-card-text>
          <v-card-actions class="px-4 pb-4">
            <v-btn color="primary" prepend-icon="mdi-content-save" :loading="loading" @click="save">保存并应用</v-btn>
            <v-btn variant="text" @click="newInbound">清空</v-btn>
          </v-card-actions>
        </v-card>
      </v-col>

      <v-col cols="12" md="5">
        <v-card rounded="xl" elevation="0">
          <v-card-title>已配置的 XHTTP 入站</v-card-title>
          <v-list v-if="inbounds.length" lines="two">
            <v-list-item v-for="inbound in inbounds" :key="inbound.id" :title="inbound.tag" :subtitle="`${inbound.listen}:${inbound.port} · ${inbound.serverName}${inbound.enable ? '' : ' · 已禁用'}`">
              <template #prepend><v-icon :color="inbound.enable ? 'success' : ''">mdi-lan</v-icon></template>
              <template #append>
                <v-btn icon="mdi-pencil" variant="text" size="small" @click="edit(inbound)" />
                <v-btn icon="mdi-delete-outline" color="error" variant="text" size="small" @click="remove(inbound.id)" />
              </template>
            </v-list-item>
          </v-list>
          <v-card-text v-else class="text-medium-emphasis">尚未配置 XHTTP 入站。</v-card-text>
        </v-card>
      </v-col>
    </v-row>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import Data from '@/store/modules/data'

interface XHTTPInbound {
  id?: number
  enable: boolean
  tag: string
  listen: string
  port: number
  serverName: string
  dest: string
  privateKey: string
  publicKey: string
  shortId: string
  fingerprint: string
  path: string
  mode: string
}

const blank = (): XHTTPInbound => ({ enable: false, tag: 'vless-reality-xhttp-2088', listen: '0.0.0.0', port: 2088, serverName: '', dest: '', privateKey: '', publicKey: '', shortId: '', fingerprint: 'chrome', path: '/xhttp', mode: 'auto' })
const form = reactive<XHTTPInbound>(blank())
const inbounds = ref<XHTTPInbound[]>([])
const assignments = ref<Record<string, number[]>>({})
const status = ref<any>({})
const clientIds = ref<number[]>([])
const loading = ref(false)
const editing = computed(() => !!form.id)
const modes = ['auto', 'packet-up', 'stream-up', 'stream-one']
const clientOptions = computed(() => Data().clients.map((client: any) => ({ title: `${client.name}${client.enable ? '' : '（已禁用）'}`, value: client.id })))

const applyData = (obj: any) => {
  inbounds.value = obj?.inbounds ?? []
  assignments.value = obj?.clientIds ?? {}
  status.value = obj?.status ?? {}
}

const load = async () => {
  loading.value = true
  try {
    const msg = await HttpUtils.get('api/xray-xhttp')
    if (msg.success) applyData(msg.obj)
  } finally { loading.value = false }
}

const newInbound = () => {
  Object.assign(form, blank())
  clientIds.value = []
}

const edit = (inbound: XHTTPInbound) => {
  Object.assign(form, JSON.parse(JSON.stringify(inbound)))
  clientIds.value = [...(assignments.value[String(inbound.id)] ?? [])]
}

const save = async () => {
  loading.value = true
  try {
    const msg = await HttpUtils.post('api/xray-xhttp-save', { data: JSON.stringify({ inbound: form, clientIds: clientIds.value }) })
    if (msg.success) {
      applyData(msg.obj)
      newInbound()
    }
  } finally { loading.value = false }
}

const remove = async (id?: number) => {
  if (!id || !confirm('确定删除此 XHTTP 入站？')) return
  loading.value = true
  try {
    const msg = await HttpUtils.post('api/xray-xhttp-delete', { id })
    if (msg.success) {
      applyData(msg.obj)
      if (form.id === id) newInbound()
    }
  } finally { loading.value = false }
}

const restart = async () => {
  loading.value = true
  try { await HttpUtils.post('api/restartXrayXHTTP', {}) ; await load() } finally { loading.value = false }
}

onMounted(load)
</script>

<style scoped>
.xhttp-page :deep(code) { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; word-break: break-all; }
</style>
