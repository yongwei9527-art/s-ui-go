<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.tls') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-card class="rounded-lg">
          <v-row>
            <v-col cols="12" sm="6" md="4">
              <v-text-field
                :label="$t('client.name')"
                hide-details
                v-model="tls.name">
              </v-text-field>
            </v-col>
            <v-col align="end">
              <v-btn-toggle v-model="tlsType"
              class="rounded-xl"
              density="compact"
              variant="outlined"
              @update:model-value="changeTlsType"
              shaped
              mandatory>
                <v-btn>TLS</v-btn>
                <v-btn>Reality</v-btn>
              </v-btn-toggle>
            </v-col>
          </v-row>
          <v-row>
            <v-col cols="12">
              <v-alert
                :color="tlsType == 1 ? 'info' : 'warning'"
                :icon="tlsType == 1 ? 'mdi-shield-key-outline' : 'mdi-certificate-outline'"
                variant="tonal"
                density="compact"
                class="mb-2"
              >
                {{ tlsType == 1 ? $t('tls.realityNotice') : $t('tls.selfSignedNotice') }}
              </v-alert>
            </v-col>
          </v-row>
          <v-row>
            <v-col cols="12" sm="6" md="4" v-if="inTls.server_name != undefined">
              <v-text-field
                label="SNI"
                hide-details
                v-model="inTls.server_name">
              </v-text-field>
            </v-col>
            <template v-if="tlsType == 0">
              <v-col cols="12" sm="6" md="4" v-if="inTls.min_version">
                <v-select
                  hide-details
                  :label="$t('tls.minVer')"
                  :items="tlsVersions"
                  v-model="inTls.min_version">
                </v-select>
              </v-col>
              <v-col cols="12" sm="6" md="4" v-if="inTls.max_version">
                <v-select
                  hide-details
                  :label="$t('tls.maxVer')"
                  :items="tlsVersions"
                  v-model="inTls.max_version">
                </v-select>
              </v-col>
              <v-col cols="12" sm="6" md="4" v-if="inTls.alpn">
                <v-select
                  hide-details
                  label="ALPN"
                  multiple
                  :items="alpn"
                  v-model="inTls.alpn">
                </v-select>
              </v-col>
              <v-col cols="12" md="8" v-if="inTls.cipher_suites != undefined">
                <v-select
                  hide-details
                  :label="$t('tls.cs')"
                  multiple
                  :items="cipher_suites"
                  v-model="inTls.cipher_suites">
                </v-select>
              </v-col>
            </template>
          </v-row>
          <template v-if="tlsType == 0">
            <v-row>
              <v-col>
                <v-btn-toggle v-model="usePath"
                class="rounded-xl"
                density="compact"
                variant="outlined"
                shaped
                mandatory>
                  <v-btn
                    @click="inTls.key=undefined; inTls.certificate=undefined"
                  >{{ $t('tls.usePath') }}</v-btn>
                  <v-btn
                    @click="inTls.key_path=undefined; inTls.certificate_path=undefined"
                  >{{ $t('tls.useText') }}</v-btn>
                </v-btn-toggle>
              </v-col>
              <v-spacer></v-spacer>
              <v-col cols="auto">
                <v-btn
                  variant="tonal"
                  density="compact"
                  icon="mdi-key-star"
                  @click="genSelfSigned"
                  :loading="loading">
                  <v-icon />
                  <v-tooltip activator="parent" location="top">
                    {{ $t('actions.generate') }}
                  </v-tooltip>
                </v-btn>
              </v-col>
            </v-row>
            <v-row v-if="usePath == 0">
              <v-col cols="12" sm="6">
                <v-text-field
                  :label="$t('tls.certPath')"
                  hide-details
                  v-model="inTls.certificate_path">
                </v-text-field>
              </v-col>
              <v-col cols="12" sm="6">
                <v-text-field
                  :label="$t('tls.keyPath')"
                  hide-details
                  v-model="inTls.key_path">
                </v-text-field>
              </v-col>
            </v-row>
            <v-row v-else>
              <v-col cols="12">
                <v-textarea
                  :label="$t('tls.cert')"
                  hide-details
                  v-model="certText">
                </v-textarea>
              </v-col>
              <v-col cols="12">
                <v-textarea
                  :label="$t('tls.key')"
                  hide-details
                  v-model="keyText">
                </v-textarea>
              </v-col>
            </v-row>
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-switch color="primary" :label="$t('tls.disableSni')" v-model="disableSni" hide-details></v-switch>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-switch color="primary" :label="$t('tls.insecure')" v-model="insecure" hide-details></v-switch>
              </v-col>
            </v-row>
          </template>
          <template v-if="outTls.reality && inTls.reality">
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                :label="$t('types.shdwTls.hs')"
                hide-details
                v-model="inTls.reality.handshake.server">
                </v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                :label="$t('out.port')"
                type="number"
                min="0"
                hide-details
                v-model="server_port">
                </v-text-field>
              </v-col>
              <v-spacer></v-spacer>
              <v-col cols="auto">
                <v-btn
                  variant="tonal"
                  density="compact"
                  icon="mdi-key-star"
                  @click="genRealityKey"
                  :loading="loading">
                  <v-icon />
                  <v-tooltip activator="parent" location="top">
                    {{ $t('actions.generate') }}
                  </v-tooltip>
                </v-btn>
              </v-col>
            </v-row>
            <v-row>
              <v-col cols="12">
                <v-text-field
                  :label="$t('tls.privKey')"
                  hide-details
                  v-model="inTls.reality.private_key">
                </v-text-field>
              </v-col>
              <v-col cols="12">
                <v-text-field
                  :label="$t('tls.pubKey')"
                  hide-details
                  v-model="outTls.reality.public_key">
                </v-text-field>
              </v-col>
              <v-col cols="12">
                <v-text-field
                  label="Short IDs"
                  hide-details
                  append-icon="mdi-refresh"
                  @click:append="randomSID"
                  v-model="short_id">
                </v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4" v-if="optionTime">
                <v-text-field
                label="Max Time Diference"
                type="number"
                min="1"
                :suffix="$t('date.m')"
                hide-details
                v-model="max_time">
                </v-text-field>
              </v-col>
            </v-row>
          </template>
          <v-row v-if="optionStore || optionKtls">
            <v-col cols="12" sm="6" md="4" v-if="optionStore">
              <v-select
                hide-details
                :label="$t('tls.store')"
                :items="storeItems"
                v-model="inTls.store">
              </v-select>
            </v-col>
            <template v-if="optionKtls">
              <v-col cols="12" sm="6" md="4">
                <v-switch color="primary" :label="$t('tls.kernelTx')" v-model="inTls.kernel_tx" hide-details></v-switch>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-switch color="primary" :label="$t('tls.kernelRx')" v-model="inTls.kernel_rx" hide-details></v-switch>
              </v-col>
            </template>
          </v-row>
          <v-row v-if="outTls.utls != undefined">
            <v-col cols="12" sm="6" md="4">
              <v-select
                hide-details
                label="Fingerprint"
                :items="fingerprints"
                v-model="outTls.utls.fingerprint">
              </v-select>
            </v-col>
          </v-row>
          <v-card-actions>
            <v-spacer></v-spacer>
            <v-menu v-model="menu" :close-on-content-click="false" location="start">
              <template v-slot:activator="{ props }">
                <v-btn v-bind="props" hide-details variant="tonal">{{ $t('tls.options') }}</v-btn>
              </template>
              <v-card>
                <v-list>
                  <template v-if="tlsType == 0">
                    <v-list-item>
                      <v-switch v-model="optionSNI" color="primary" label="SNI" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionALPN" color="primary" label="ALPN" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionMinV" color="primary" :label="$t('tls.minVer')" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionMaxV" color="primary" :label="$t('tls.maxVer')" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionCS" color="primary" :label="$t('tls.cs')" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionFP" color="primary" label="UTLS" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionStore" color="primary" :label="$t('tls.store')" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionKtls" color="primary" :label="$t('tls.ktls')" hide-details></v-switch>
                    </v-list-item>
                  </template>
                  <template v-else>
                    <v-list-item>
                      <v-switch v-model="optionTime" color="primary" label="Max Time Difference" hide-details></v-switch>
                    </v-list-item>
                  </template>
                </v-list>
              </v-card>
            </v-menu>
          </v-card-actions>
        </v-card>
        <AcmeVue :tls="inTls" />
        <EchVue :iTls="inTls" :oTls="outTls" />
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="primary"
          variant="outlined"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn
          color="primary"
          variant="tonal"
          :loading="loading"
          @click="saveChanges"
        >
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { tls, iTls, defaultInTls, oTls, defaultOutTls } from '@/types/tls'
import AcmeVue from '@/components/tls/Acme.vue'
import EchVue from '@/components/tls/Ech.vue'
import HttpUtils from '@/plugins/httputil'
import { push } from 'notivue'
import { i18n } from '@/locales'
import RandomUtil from '@/plugins/randomUtil'
export default {
  props: ['visible', 'data', 'id'],
  emits: ['close', 'save'],
  data() {
    return {
      tls: <tls>{ id: 0, name: '', server: <iTls>{ enabled: true }, client: <oTls>{} },
      title: "add",
      requestId: 0,
      closed: false,
      loading: false,
      menu: false,
      tlsType: 0,
      usePath: 0,
      alpn: [
        { title: "H3", value: 'h3' },
        { title: "H2", value: 'h2' },
        { title: "Http/1.1", value: 'http/1.1' },
      ],
      tlsVersions: [ '1.0', '1.1', '1.2', '1.3' ],
      cipher_suites: [
        { title: "RSA-AES128-CBC-SHA", value: "TLS_RSA_WITH_AES_128_CBC_SHA" },
        { title: "RSA-AES256-CBC-SHA", value: "TLS_RSA_WITH_AES_256_CBC_SHA" },
        { title: "RSA-AES128-GCM-SHA256", value: "TLS_RSA_WITH_AES_128_GCM_SHA256" },
        { title: "RSA-AES256-GCM-SHA384", value: "TLS_RSA_WITH_AES_256_GCM_SHA384" },
        { title: "AES128-GCM-SHA256", value: "TLS_AES_128_GCM_SHA256" },
        { title: "AES256-GCM-SHA384", value: "TLS_AES_256_GCM_SHA384" },
        { title: "CHACHA20-POLY1305-SHA256", value: "TLS_CHACHA20_POLY1305_SHA256" },
        { title: "ECDHE-ECDSA-AES128-CBC-SHA", value: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA" },
        { title: "ECDHE-ECDSA-AES256-CBC-SHA", value: "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA" },
        { title: "ECDHE-RSA-AES128-CBC-SHA", value: "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA" },
        { title: "ECDHE-RSA-AES256-CBC-SHA", value: "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA" },
        { title: "ECDHE-ECDSA-AES128-GCM-SHA256", value: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256" },
        { title: "ECDHE-ECDSA-AES256-GCM-SHA384", value: "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384" },
        { title: "ECDHE-RSA-AES128-GCM-SHA256", value: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" },
        { title: "ECDHE-RSA-AES256-GCM-SHA384", value: "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" },
        { title: "ECDHE-ECDSA-CHACHA20-POLY1305-SHA256", value: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256" },
        { title: "ECDHE-RSA-CHACHA20-POLY1305-SHA256", value: "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256" }
      ],
      storeItems: [
        { title: "Mozilla", value: "mozilla" },
        { title: "Chrome", value: "chrome" },
      ],
      fingerprints: [
        { title: "Chrome", value: "chrome" },
        { title: "Firefox", value: "firefox" },
        { title: "Microsoft Edge", value: "edge" },
        { title: "Apple Safari", value: "safari" },
        { title: "360", value: "360" },
        { title: "QQ", value: "qq" },
        { title: "Apple IOS", value: "ios" },
        { title: "Android", value: "android" },
        { title: "Random", value: "random" },
        { title: "Randomized", value: "randomized" },
      ]
    }
  },
  methods: {
    updateData(id: number) {
      ++this.requestId
      this.loading = false
      this.menu = false
      try {
      if (id > 0) {
        const newData = <tls>JSON.parse(this.$props.data)
        if (!newData || typeof newData !== 'object' || Array.isArray(newData)) throw new Error('Invalid TLS config')
        this.tls = newData
        if (this.tls.server == null) this.tls.server = { enabled: true }
        if (this.tls.client == null) this.tls.client = {}
        if (this.tls.server.reality) {
          this.tls.server.reality = {
            ...this.tls.server.reality,
            handshake: Object.assign({ server: '', server_port: 443 }, this.tls.server.reality.handshake),
            short_id: this.tls.server.reality.short_id ?? [],
          }
          this.tls.client.reality = { ...structuredClone(defaultOutTls.reality!), ...this.tls.client.reality }
        }
        this.tlsType = newData.server?.reality == undefined ? 0 : 1
        this.usePath = newData.server?.key == undefined ? 0 : 1
        this.title = "edit"
      }
      else {
        this.tls = <tls>{ id: 0, name: '', server: {enabled: true}, client: {} }
        this.tlsType = 0
        this.usePath = 0
        this.title = "add"
      }
      } catch { this.closeModal() }
    },
    changeTlsType(){
      if (!this.$props.visible || this.closed) return
      ++this.requestId
      this.loading = false
      if (this.tlsType) {
        this.tls.server = <iTls>{
          enabled: true,
          reality: { enabled: true, handshake: { server_port: 443 }, short_id: RandomUtil.randomShortId() },
          server_name: ""
        }
        this.tls.client = <oTls>{ reality: structuredClone(defaultOutTls.reality!), utls: structuredClone(defaultOutTls.utls!) }
      } else {
        this.tls.server = <iTls>{ enabled: true }
        this.tls.client = <oTls>{}
      }
    },
    closeModal() {
      ++this.requestId
      this.closed = true
      this.loading = false
      this.menu = false
      this.tls = <tls>{ id: 0, name: '', server: { enabled: true }, client: {} }
      this.$emit('close')
    },
    saveChanges() {
      if (!this.$props.visible || this.closed || this.loading) return
      this.$emit('save', JSON.parse(JSON.stringify(this.tls)))
    },
    async genSelfSigned() {
      if (!this.$props.visible || this.closed || this.tlsType !== 0) return
      const requestId = ++this.requestId
      const server = this.inTls
      const serverName = server.server_name
      const key = server.key
      const certificate = server.certificate
      const keyPath = server.key_path
      const certificatePath = server.certificate_path
      const usePath = this.usePath
      this.loading = true
      try {
        const msg = await HttpUtils.get('api/keypairs', { k: "tls", o: serverName ?? "''" })
        if (requestId !== this.requestId || this.closed || !this.$props.visible || server !== this.inTls ||
            serverName !== server.server_name || key !== server.key || certificate !== server.certificate ||
            keyPath !== server.key_path || certificatePath !== server.certificate_path || usePath !== this.usePath) return
        if (!msg?.success || !Array.isArray(msg.obj) || msg.obj.some((line: any) => typeof line !== 'string')) throw new Error('Invalid certificate response')
        const readPem = (begin: string, end: string): string[] => {
          const start = msg.obj.indexOf(begin)
          const finish = msg.obj.indexOf(end, start + 1)
          if (start < 0 || finish <= start) throw new Error('Incomplete PEM')
          return msg.obj.slice(start, finish + 1)
        }
        const privateKey = readPem("-----BEGIN PRIVATE KEY-----", "-----END PRIVATE KEY-----")
        const publicKey = readPem("-----BEGIN CERTIFICATE-----", "-----END CERTIFICATE-----")
        server.key_path = undefined
        server.certificate_path = undefined
        server.key = privateKey
        server.certificate = publicKey
        this.usePath = 1
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) push.error({ message: i18n.global.t('failed') })
      } finally {
        if (requestId === this.requestId) this.loading = false
      }
    },
    async genRealityKey() {
      if (!this.$props.visible || this.closed || this.tlsType !== 1 || !this.inTls.reality || !this.outTls.reality) return
      const requestId = ++this.requestId
      const server = this.inTls.reality
      const client = this.outTls.reality
      const oldPrivateKey = server.private_key
      const oldPublicKey = client.public_key
      this.loading = true
      try {
        const msg = await HttpUtils.get('api/keypairs', { k: "reality" })
        if (requestId !== this.requestId || this.closed || !this.$props.visible || server !== this.inTls.reality || client !== this.outTls.reality ||
            oldPrivateKey !== server.private_key || oldPublicKey !== client.public_key) return
        if (!msg?.success || !Array.isArray(msg.obj)) throw new Error('Invalid key response')
        const privateKey = msg.obj.find((line: any) => typeof line === 'string' && line.startsWith('PrivateKey:'))?.slice(11).trim()
        const publicKey = msg.obj.find((line: any) => typeof line === 'string' && line.startsWith('PublicKey:'))?.slice(10).trim()
        if (!privateKey || !publicKey) throw new Error('Incomplete key pair')
        server.private_key = privateKey
        client.public_key = publicKey
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) push.error({ message: i18n.global.t('failed') })
      } finally {
        if (requestId === this.requestId) this.loading = false
      }
    },
    randomSID(){
      this.short_id = RandomUtil.randomShortId().join(',')
    }
  },
  computed: {
    inTls(): iTls {
      return this.tls.server
    },
    outTls(): oTls {
      return this.tls.client
    },
    certText: {
      get(): string { return this.inTls.certificate ? this.inTls.certificate.join('\n') : '' },
      set(v:string | null) { this.inTls.certificate = v?.length ? v.split('\n') : undefined }
    },
    keyText: {
      get(): string { return this.inTls.key ? this.inTls.key.join('\n') : '' },
      set(v:string | null) { this.inTls.key = v?.length ? v.split('\n') : undefined }
    },
    disableSni: {
      get() { return this.outTls.disable_sni ?? false },
      set(v: boolean) { this.tls.client.disable_sni = v ? true : undefined }
    },
    insecure: {
      get() { return this.outTls.insecure ?? false },
      set(v: boolean) { this.tls.client.insecure = v ? true : undefined }
    },
    server_port: {
      get() { return this.inTls.reality?.handshake?.server_port ? this.inTls.reality.handshake.server_port : 443 },
      set(v: any) {
        if (this.inTls.reality){
          const port = Number(v)
          this.inTls.reality.handshake.server_port = Number.isInteger(port) && port > 0 && port <= 65535 ? port : 443
        }
      }
    },
    short_id: {
      get() { return this.inTls.reality?.short_id ? this.inTls.reality.short_id.join(',') : undefined },
      set(v: string | null) {
        if (this.inTls.reality){
          this.inTls.reality.short_id = v?.length ? v.split(',').map(id => id.trim()).filter(Boolean) : []
        }
      }
    },
    max_time: {
      get() { return this.inTls?.reality?.max_time_difference ? this.inTls.reality.max_time_difference.replace('m','') : 1 },
      set(v: number) {
        if (this.inTls.reality){
          this.inTls.reality.max_time_difference = v > 0 ? v + 'm' : '1m'
        }
      }
    },
    optionSNI: {
      get(): boolean { return this.inTls.server_name != undefined },
      set(v:boolean) { this.inTls.server_name = v ? '' : undefined }
    },
    optionALPN: {
      get(): boolean { return this.inTls.alpn != undefined },
      set(v:boolean) { this.inTls.alpn = v ? structuredClone(defaultInTls.alpn!) : undefined }
    },
    optionMinV: {
      get(): boolean { return this.inTls.min_version != undefined },
      set(v:boolean) { this.inTls.min_version = v ? defaultInTls.min_version : undefined }
    },
    optionMaxV: {
      get(): boolean { return this.inTls.max_version != undefined },
      set(v:boolean) { this.inTls.max_version = v ? defaultInTls.max_version : undefined }
    },
    optionCS: {
      get(): boolean { return this.inTls.cipher_suites != undefined },
      set(v:boolean) { this.inTls.cipher_suites = v ? structuredClone(defaultInTls.cipher_suites!) : undefined }
    },
    optionFP: {
      get(): boolean { return this.outTls.utls != undefined },
      set(v:boolean) { this.outTls.utls = v ? structuredClone(defaultOutTls.utls!) : undefined }
    },
    optionStore: {
      get(): boolean { return this.inTls.store != undefined },
      set(v:boolean) { this.inTls.store = v ? 'mozilla' : undefined }
    },
    optionKtls: {
      get(): boolean { return this.inTls.kernel_tx != undefined || this.inTls.kernel_rx != undefined },
      set(v:boolean) {
        if (v) {
          this.inTls.kernel_tx = false
          this.inTls.kernel_rx = false
        } else {
          delete this.inTls.kernel_tx
          delete this.inTls.kernel_rx
        }
      }
    },
    optionEch: {
      get(): boolean { return this.outTls.ech != undefined },
      set(v:boolean) { this.outTls.ech = v ? structuredClone(defaultOutTls.ech!) : undefined }
    },
    optionTime: {
      get(): boolean { return this.inTls?.reality?.max_time_difference != undefined },
      set(v:boolean) { if (this.inTls.reality) this.inTls.reality.max_time_difference = v ? "1m" : undefined }
    }
  },
  beforeUnmount() { ++this.requestId; this.closed = true },
  watch: {
    visible: {
      immediate: true,
      handler(v: boolean) {
        this.closed = !v
        if (v) this.updateData(this.$props.id)
        else { ++this.requestId; this.loading = false }
      },
    },
    id() { if (this.$props.visible && !this.closed) this.updateData(this.$props.id) },
  },
  components: { AcmeVue, EchVue }
}
</script>