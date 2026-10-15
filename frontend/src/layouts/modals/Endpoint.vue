<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.endpoint') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-select
            hide-details
            :disabled="endpoint.id > 0"
            :label="$t('type')"
            :items="Object.keys(epTypes).map((key,index) => ({title: key, value: Object.values(epTypes)[index]}))"
            v-model="endpoint.type"
            @update:modelValue="changeType">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="endpoint.tag" :label="$t('objects.tag')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <Wireguard v-if="endpoint.type == epTypes.Wireguard"
          :data="endpoint"
          @getWgPubKey="getWgPubKey"
          @newWgKey="newWgKey"
          @addPeer="addWgPeer"
          @delPeer="delWgPeer"
          @refreshPeerKey="refreshWgPeerKey" />
        <Warp v-if="endpoint.type == epTypes.Warp" :data="endpoint" />
        <TailscaleVue v-if="endpoint.type == epTypes.Tailscale" :data="endpoint" />
        <Dial :dial="endpoint" />
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
import { EpTypes, createEndpoint } from '@/types/endpoints'
import RandomUtil from '@/plugins/randomUtil'
import Dial from '@/components/Dial.vue'
import Wireguard from '@/components/protocols/Wireguard.vue'
import Warp from '@/components/protocols/Warp.vue'
import TailscaleVue from '@/components/protocols/Tailscale.vue'
import HttpUtils from '@/plugins/httputil'
import { push } from 'notivue'
import { i18n } from '@/locales'
import Data from '@/store/modules/data'
export default {
  props: ['visible', 'data', 'id', 'tags'],
  emits: ['close'],
  data() {
    return {
      endpoint: createEndpoint("wireguard", { tag: "", peers: [], ext: { keys: [] } }),
      title: "add", tab: "t1", loading: false, saving: false,
      requestId: 0, keyRequestId: 0, pendingRequests: 0, closed: false,
      peerRequests: new Map<any, number>(),
      epTypes: EpTypes,
    }
  },
  methods: {
    updateData(id: number) {
      this.saving = false
      ++this.requestId
      this.pendingRequests = 0
      this.loading = false
      this.peerRequests.clear()
      this.tab = "t1"
      this.title = id > 0 ? "edit" : "add"
      if (id > 0) {
        try {
          const data = JSON.parse(this.$props.data)
          if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error('Invalid endpoint')
          this.endpoint = createEndpoint(data.type, data)
          if (this.endpoint.type === EpTypes.Wireguard) {
            if (!Array.isArray(this.endpoint.peers)) this.endpoint.peers = []
            this.endpoint.ext = { ...this.endpoint.ext, keys: Array.isArray(this.endpoint.ext?.keys) ? this.endpoint.ext.keys : [] }
          }
        } catch { this.closeModal() }
      } else {
        this.endpoint = createEndpoint(EpTypes.Wireguard, { tag: "", peers: [], ext: { keys: [] } })
        this.changeType()
      }
    },
    async changeType() {
      if (!this.$props.visible || this.closed || this.saving) return
      ++this.requestId
      this.pendingRequests = 0
      this.loading = false
      this.peerRequests.clear()
      const type = this.endpoint.type
      const tag = type + "-" + RandomUtil.randomSeq(3)
      const config: any = { tag }
      if (type === EpTypes.Wireguard) {
        const octet = RandomUtil.randomIntRange(1, 255)
        Object.assign(config, {
          listen_port: this.endpoint.listen_port > 0 ? this.endpoint.listen_port : RandomUtil.randomIntRange(10000, 60000),
          address: ['10.0.0.' + octet + '/32', 'fe80::' + octet.toString(16) + '/128'],
          peers: [], ext: { keys: [] },
        })
      }
      this.endpoint = createEndpoint(type, config)
      if (type === EpTypes.Wireguard) await this.newWgKey()
    },
    closeModal() {
      this.saving = false
      ++this.requestId
      this.closed = true
      this.pendingRequests = 0
      this.loading = false
      this.peerRequests.clear()
      this.endpoint = createEndpoint(EpTypes.Wireguard, { tag: "", peers: [], ext: { keys: [] } })
      this.$emit('close')
    },
    async saveChanges() {
      if (!this.$props.visible || this.closed || this.loading) return
      if (Data().checkTag("endpoint", this.endpoint.id, this.endpoint.tag)) return
      const requestId = this.requestId
      const id = this.$props.id
      this.saving = true
      this.loading = true
      try {
        const success = await Data().save("endpoints", id == 0 ? "new" : "edit", JSON.parse(JSON.stringify(this.endpoint)))
        if (requestId === this.requestId && !this.closed && this.$props.visible && id === this.$props.id && success) this.closeModal()
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) push.error({ message: i18n.global.t('failed') })
      } finally {
        if (requestId === this.requestId) {
          this.saving = false
          this.loading = this.pendingRequests > 0
        }
      }
    },
    async genWgKey() {
      const msg = await HttpUtils.get('api/keypairs', { k: "wireguard" })
      if (!msg?.success || !Array.isArray(msg.obj)) throw new Error('Key generation failed')
      const result = { private_key: "", public_key: "" }
      for (const line of msg.obj) {
        if (typeof line !== 'string') continue
        if (line.startsWith("PrivateKey:")) result.private_key = line.slice(11).trim()
        if (line.startsWith("PublicKey:")) result.public_key = line.slice(10).trim()
      }
      if (!result.private_key || !result.public_key) throw new Error('Incomplete key pair')
      return result
    },
    async withWgRequest(work: (endpoint: any, isCurrent: () => boolean) => Promise<void>) {
      if (!this.$props.visible || this.closed || this.saving || this.endpoint.type !== EpTypes.Wireguard) return
      const requestId = this.requestId
      const endpoint = this.endpoint
      const isCurrent = () => requestId === this.requestId && !this.closed && this.$props.visible && endpoint === this.endpoint
      ++this.pendingRequests
      this.loading = true
      try { await work(endpoint, isCurrent) }
      catch { if (isCurrent()) push.error({ message: i18n.global.t('failed') }) }
      finally {
        if (isCurrent()) {
          --this.pendingRequests
          this.loading = this.pendingRequests > 0
        }
      }
    },
    async newWgKey() {
      const keyRequestId = ++this.keyRequestId
      await this.withWgRequest(async (endpoint, isCurrent) => {
        const privateKey = endpoint.private_key
        const publicKey = endpoint.ext?.public_key
        const keys = await this.genWgKey()
        if (!isCurrent() || keyRequestId !== this.keyRequestId || endpoint.private_key !== privateKey || endpoint.ext?.public_key !== publicKey) return
        endpoint.private_key = keys.private_key
        endpoint.ext = { ...endpoint.ext, public_key: keys.public_key, keys: endpoint.ext?.keys ?? [] }
      })
    },
    async getWgPubKey(private_key: string) {
      const keyRequestId = ++this.keyRequestId
      await this.withWgRequest(async (endpoint, isCurrent) => {
        const msg = await HttpUtils.get('api/keypairs', { k: "wireguard", o: private_key })
        if (!isCurrent() || keyRequestId !== this.keyRequestId || endpoint.private_key !== private_key) return
        if (!msg?.success || !Array.isArray(msg.obj) || typeof msg.obj[0] !== 'string' || !msg.obj[0]) throw new Error('Public key derivation failed')
        endpoint.ext = { ...endpoint.ext, keys: endpoint.ext?.keys ?? [], public_key: msg.obj[0] }
      })
    },
    async addWgPeer() {
      await this.withWgRequest(async (endpoint, isCurrent) => {
        const keys = await this.genWgKey()
        if (!isCurrent()) return
        endpoint.ext = { ...endpoint.ext, keys: endpoint.ext?.keys ?? [] }
        const address = this.findFreeIP()
        endpoint.ext.keys.push(keys)
        endpoint.peers.push({ public_key: keys.public_key, allowed_ips: [address] })
      })
    },
    findFreeIP(): string {
      const used = (this.endpoint.peers ?? []).flatMap((peer: any) => peer?.allowed_ips ?? [])
      for (let i = 2; i < 255; i++) {
        const ip = '10.0.1.' + i + '/32'
        if (!used.includes(ip)) return ip
      }
      throw new Error('No free peer address')
    },
    delWgPeer(index: number) {
      if (!this.$props.visible || this.closed || this.endpoint.type !== EpTypes.Wireguard) return
      const peer = this.endpoint.peers?.[index]
      if (!peer) return
      this.peerRequests.delete(peer)
      if (this.endpoint.ext) this.endpoint.ext.keys = (this.endpoint.ext.keys ?? []).filter((key: any) => key.public_key !== peer.public_key)
      this.endpoint.peers.splice(index, 1)
    },
    async refreshWgPeerKey(index: number) {
      const peer = this.endpoint.peers?.[index]
      if (!peer) return
      const publicKey = peer.public_key
      const version = (this.peerRequests.get(peer) ?? 0) + 1
      this.peerRequests.set(peer, version)
      await this.withWgRequest(async (endpoint, isCurrent) => {
        const keys = await this.genWgKey()
        if (!isCurrent() || !endpoint.peers.includes(peer) || this.peerRequests.get(peer) !== version || peer.public_key !== publicKey) return
        endpoint.ext = { ...endpoint.ext, keys: endpoint.ext?.keys ?? [] }
        const keyIndex = endpoint.ext.keys.findIndex((key: any) => key.public_key === publicKey)
        endpoint.ext.keys[keyIndex < 0 ? endpoint.ext.keys.length : keyIndex] = keys
        peer.public_key = keys.public_key
      })
    },
  },
  beforeUnmount() { ++this.requestId; this.closed = true },
  watch: {
    visible: {
      immediate: true,
      handler(v: boolean) {
        this.closed = !v
        if (v) this.updateData(this.$props.id)
        else { ++this.requestId; this.loading = false; this.saving = false; this.pendingRequests = 0 }
      },
    },
    id() { if (this.$props.visible && !this.closed) this.updateData(this.$props.id) },
  },
  components: { Dial, Wireguard, Warp, TailscaleVue }
}
</script>