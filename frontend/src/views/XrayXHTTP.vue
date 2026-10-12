<template>
  <section class="xhttp-page">
    <v-card rounded="xl" class="mb-5" elevation="0">
      <v-card-title class="d-flex align-center ga-3 flex-wrap">
        <v-icon color="primary">mdi-lan-connect</v-icon>
        <span>Xray Reality · TCP Vision / XHTTP</span>
        <v-chip :color="status.running ? 'success' : 'warning'" size="small" variant="tonal">
          {{ status.running ? '内置引擎运行中' : '内置引擎未运行' }}
        </v-chip>
      </v-card-title>
      <v-card-text>
        <v-alert type="info" variant="tonal" density="compact" class="mb-3">
          Xray-core Go 源码依赖已编译进 S-UI 主程序，无需安装或配置外部 xray 可执行文件。XHTTP 默认 TCP 端口为 2088，目标为 go.microsoft.com:443。新节点保存时自动生成独立密钥；确认目标可达、分配客户端后再手动启用。Reality 无需自己的域名证书。
        </v-alert>
        <v-alert type="warning" variant="tonal" density="compact" class="mb-3">
          当前内置 Xray 流量不会计入 S-UI 的上传/下载统计或配额；同时它使用独立的 Xray 直连出站，不会自动继承 sing-box 的路由与 DNS 策略。需要严格限额或分流时，请先完成额外的流量统计与策略配置。
        </v-alert>
        <div class="text-body-2">
          内置 Xray 版本：<code>{{ status.version || '未知' }}</code>
          <span v-if="status.runtimeError" class="text-error ml-2">运行异常：{{ status.runtimeError }}</span>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-btn color="primary" variant="tonal" prepend-icon="mdi-restart" :loading="loading" @click="restart">重启内置 Xray</v-btn>
        <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
      </v-card-actions>
    </v-card>

    <v-row>
      <v-col cols="12" md="7">
        <v-card rounded="xl" elevation="0">
          <v-card-title>{{ editing ? '编辑 Xray Reality 入站' : '新建 Xray Reality 入站' }}</v-card-title>
          <v-card-text>
            <v-row>
              <v-col cols="12" sm="4"><v-switch v-model="form.enable" color="primary" label="启用" hide-details /></v-col>
              <v-col cols="12" sm="8"><v-text-field v-model="form.tag" label="标签" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.listen" label="监听地址" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model.number="form.port" type="number" min="1" max="65535" label="TCP 端口" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.serverName" label="Reality Server Name / SNI" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.dest" label="Reality destination (host:port)" /></v-col>
              <v-col cols="12" sm="6"><v-select v-model="form.transport" :items="transports" label="Transport" /></v-col>
              <v-col v-if="form.transport === 'xhttp'" cols="12" sm="6"><v-text-field v-model="form.path" label="XHTTP path" /></v-col>
              <v-col v-if="form.transport === 'xhttp'" cols="12" sm="6"><v-select v-model="form.mode" :items="modes" label="XHTTP mode" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.shortId" label="Reality short ID（十六进制）" /></v-col>
              <v-col cols="12" sm="6"><v-text-field v-model="form.fingerprint" label="客户端指纹" /></v-col>
              <v-col cols="12">
                <v-btn variant="tonal" prepend-icon="mdi-key-plus" :loading="loading" @click="generateRealityCredentials">
                  生成匹配的 Reality 密钥与 Short ID
                </v-btn>
              </v-col>
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
          <v-card-title>已配置的 Xray Reality 入站</v-card-title>
          <v-list v-if="inbounds.length" lines="two">
            <v-list-item v-for="inbound in inbounds" :key="inbound.id" :title="inbound.tag" :subtitle="`${inbound.listen}:${inbound.port} · ${inbound.serverName}${inbound.enable ? '' : ' · 已禁用'}`">
              <template #prepend><v-icon :color="inbound.enable ? 'success' : ''">mdi-lan</v-icon></template>
              <template #append>
                <v-btn icon="mdi-pencil" variant="text" size="small" @click="edit(inbound)" />
                <v-btn icon="mdi-delete-outline" color="error" variant="text" size="small" @click="remove(inbound.id)" />
              </template>
            </v-list-item>
          </v-list>
          <v-card-text v-else class="text-medium-emphasis">尚未配置 Xray Reality 入站。</v-card-text>
        </v-card>
      </v-col>
    </v-row>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import Data from '@/store/modules/data'
import { createXrayDefaults } from '@/plugins/xrayDefaults'

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
  transport: 'tcp' | 'xhttp'
  path: string
  mode: string
}

const blank = (): XHTTPInbound => createXrayDefaults()
const form = reactive<XHTTPInbound>(blank())
const inbounds = ref<XHTTPInbound[]>([])
const assignments = ref<Record<string, number[]>>({})
const status = ref<any>({})
const clientIds = ref<number[]>([])
const loading = ref(false)
const editing = computed(() => !!form.id)
const modes = ['auto', 'packet-up', 'stream-up', 'stream-one']
const transports = [ { title: 'VLESS Reality TCP + XTLS Vision', value: 'tcp' }, { title: 'VLESS Reality XHTTP', value: 'xhttp' } ]
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
  // XHTTP was the only transport before this field existed.
  if (!form.transport) form.transport = 'xhttp'
  clientIds.value = [...(assignments.value[String(inbound.id)] ?? [])]
}

const generateRealityCredentials = async () => {
  loading.value = true
  try {
    const msg = await HttpUtils.get('api/keypairs', { k: 'reality' })
    if (!msg.success || !Array.isArray(msg.obj)) return
    for (const line of msg.obj as string[]) {
      if (line.startsWith('PrivateKey:')) form.privateKey = line.slice('PrivateKey:'.length).trim()
      if (line.startsWith('PublicKey:')) form.publicKey = line.slice('PublicKey:'.length).trim()
    }
    const shortId = new Uint8Array(8)
    crypto.getRandomValues(shortId)
    form.shortId = Array.from(shortId, value => value.toString(16).padStart(2, '0')).join('')
  } finally {
    loading.value = false
  }
}

const save = async () => {
  // Generate only for new nodes; never rotate an edited node implicitly.
  if (!editing.value && !form.privateKey && !form.publicKey) {
    await generateRealityCredentials()
    if (!form.privateKey || !form.publicKey) return
  }
  if (!editing.value && !form.shortId) {
    const bytes = new Uint8Array(8)
    crypto.getRandomValues(bytes)
    form.shortId = Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('')
  }
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
