<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg" :loading="loading">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.client') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-skeleton-loader
          class="mx-auto border"
          width="95%"
          type="card, text, divider, list-item-two-line"
          v-if="loading"
        ></v-skeleton-loader>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-container style="padding: 0;" :hidden="loading">
          <v-tabs
            v-model="tab"
            align-tabs="center"
          >
            <v-tab value="t1">{{ $t('client.basics') }}</v-tab>
            <v-tab value="t2">{{ $t('client.config') }}</v-tab>
            <v-tab value="t3">{{ $t('client.links') }}</v-tab>
          </v-tabs>
          <v-window v-model="tab">
            <v-window-item value="t1">
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-switch color="primary" v-model="client.enable" :label="$t('enable')" hide-details></v-switch>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-combobox v-model="client.group" :items="groups" :label="$t('client.group')" hide-details></v-combobox>
                </v-col>
              </v-row>
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model="client.name" :label="$t('client.name')" hide-details></v-text-field>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model="client.desc" :label="$t('client.desc')" hide-details></v-text-field>
                </v-col>
              </v-row>
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model.number="Volume" type="number" min="0" :label="$t('stats.volume')" suffix="GiB" hide-details></v-text-field>
                </v-col>
                <v-col cols="12" sm="6" md="4" v-if="!(client.delayStart && !client.autoReset)">
                  <DatePick :expiry="expDate" @submit="setDate" />
                </v-col>
                <v-col cols="12" sm="6" md="4" v-if="client.autoReset || client.delayStart">
                  <v-text-field v-model.number="resetDays" type="number" min="1" :label="$t('client.resetDays')" hide-details></v-text-field>
                </v-col>
              </v-row>
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-switch color="primary"
                    :disabled="client.up+client.down>0"
                    v-model="delayStart"
                    :label="$t('client.delayStart')" hide-details>
                  </v-switch>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-switch color="primary" v-model="autoReset" :label="$t('client.autoReset')" hide-details></v-switch>
                </v-col>
              </v-row>
              <v-row v-if="id > 0">
                <v-col cols="12" sm="6" md="4" class="d-flex flex-column">
                  <div class="d-flex justify-space-between align-center">
                    <div>
                      {{ $t('stats.usage') }}: {{ total }}<sup dir="ltr" v-if="percent>0">({{ percent }}%)</sup>
                    </div>
                    <v-btn density="compact" variant="text" icon="mdi-restore" @click="resetUsage">
                      <v-tooltip activator="parent" location="top">
                        {{ $t('reset') }}
                      </v-tooltip>
                      <v-icon />
                    </v-btn>
                  </div>
                  <v-progress-linear
                    v-model="percent"
                    :color="percentColor"
                    v-if="client.volume>0"
                    bottom
                  >
                  </v-progress-linear>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-icon icon="mdi-upload" color="orange" /><span class="text-orange">{{ up }}</span>
                  /
                  <v-icon icon="mdi-download" color="success" /><span class="text-success">{{ down }}</span>
                </v-col>
              </v-row>
              <v-row v-if="id >0 && client.autoReset">
                <v-col cols="12" sm="6" md="4">
                  <div class="text-medium-emphasis">{{ $t('client.nextReset') }}</div>
                  <div dir="ltr">{{ nextResetFormatted }}</div>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <div class="text-medium-emphasis">{{ $t('main.stats.totalUsage') }}</div>
                  <div>
                    <v-icon icon="mdi-upload" color="orange" /><span class="text-orange">{{ totalUp }}</span>
                    /
                    <v-icon icon="mdi-download" color="success" /><span class="text-success">{{ totalDown }}</span>
                  </div>
                </v-col>
              </v-row>
              <v-row>
                <v-col>
                  <v-select
                    v-model="clientInbounds"
                    :items="inboundTags"
                    :label="$t('client.inboundTags')"
                    clearable
                    multiple
                    chips
                    hide-details>
                    <template v-slot:append>
                      <v-icon @click="setAllInbounds" icon="mdi-set-all" v-tooltip:top="$t('all')" />
                    </template>
                  </v-select>
                </v-col>
              </v-row>
            </v-window-item>
            <v-window-item value="t2">
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-btn variant="tonal" @click="shuffle()">{{ $t('reset') + ' - ' + $t('all') }}<v-icon icon="mdi-refresh" /></v-btn>
                </v-col>
              </v-row>
              <v-row v-for="key in Object.keys(clientConfig)">
                <v-col cols="12" md="3" align="end" align-self="center">
                    {{ key }}
                    <v-icon @click="shuffle(key)" icon="mdi-refresh" v-tooltip:top="$t('reset')" />
                </v-col>
                <v-col>
                  <v-text-field
                    v-if="clientConfig[key].password != undefined"
                    label="Password"
                    v-model="clientConfig[key].password"
                    hide-details>
                  </v-text-field>
                  <v-text-field
                    v-if="clientConfig[key].uuid != undefined"
                    label="UUID"
                    v-model="clientConfig[key].uuid"
                    hide-details>
                  </v-text-field>
                  <v-text-field
                    v-if="key == 'vless'"
                    label="Flow"
                    v-model="clientConfig[key].flow"
                    hide-details>
                  </v-text-field>
                  <v-text-field
                    v-if="key == 'hysteria'"
                    label="Auth"
                    v-model="clientConfig[key].auth_str"
                    hide-details>
                  </v-text-field>
                </v-col>
              </v-row>
            </v-window-item>
            <v-window-item value="t3">
              <v-row v-for="(lnk, index) in links">
                <v-col cols="auto">{{ index + 1 }}</v-col>
                <v-col style="direction: ltr; overflow-y: hidden;">{{ lnk.uri }}</v-col>
                <v-col cols="auto">
                  <v-btn density="compact" variant="text" icon="mdi-content-copy" @click="copyToClipboard(lnk.uri)">
                    <v-tooltip activator="parent" location="top" :text="$t('copyToClipboard')"></v-tooltip>
                    <v-icon />
                  </v-btn>
                </v-col>
              </v-row>
              <v-row>
                <v-col>
                  <v-btn color="primary" @click="extLinks.push({ type: 'external', uri: ''})">{{ $t('actions.add') }} {{ $t('client.external') }}</v-btn>
                </v-col>
              </v-row>
              <v-row v-for="(lnk, index) in extLinks">
                <v-col>
                  <v-text-field
                  dir="ltr"
                  :label="$t('client.external') + ' ' + (index+1)"
                  append-icon="mdi-delete"
                  @click:append="extLinks.splice(index,1)"
                  placeholder="<protocol>://<data>"
                  v-model="lnk.uri" />
                </v-col>
              </v-row>
              <v-row>
                <v-col>
                  <v-btn color="primary" @click="subLinks.push({ type: 'sub', uri: ''})">{{ $t('actions.add') }} {{ $t('client.sub') }}</v-btn>
                </v-col>
              </v-row>
              <v-row v-for="(lnk, index) in subLinks">
                <v-col>
                  <v-text-field
                  dir="ltr"
                  :label="$t('client.sub') + ' ' + (index+1)"
                  append-icon="mdi-delete"
                  @click:append="subLinks.splice(index,1)"
                  placeholder="http[s]://<domain>[:]<port>/<path>"
                  v-model="lnk.uri" />
                </v-col>
              </v-row>
            </v-window-item>
          </v-window>
        </v-container>
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
import { createClient, randomConfigs, updateConfigs, Link, shuffleConfigs } from '@/types/clients'
import DatePick from '@/components/DateTime.vue'
import { HumanReadable } from '@/plugins/utils'
import Data from '@/store/modules/data'
import { locale } from '@/locales'
import { push } from 'notivue'
import Clipboard from 'clipboard'

export default {
  props: ['visible', 'id', 'inboundTags', 'groups'],
  emits: ['close'],
  data() {
    return {
      requestId: 0,
      closed: false,
      client: createClient(),
      title: "add",
      loading: false,
      tab: "t1",
      clientConfig: <any>[],
      links: <Link[]>[],
      extLinks: <Link[]>[],
      subLinks: <Link[]>[],
    }
  },
  methods: {
    async updateData(id: number) {
      const requestId = ++this.requestId
      this.client = createClient()
      this.clientConfig = this.client.config
      this.links = []
      this.extLinks = []
      this.subLinks = []
      this.tab = "t1"
      this.title = id > 0 ? "edit" : "add"
      this.loading = id > 0
      if (id <= 0) {
        this.clientConfig = randomConfigs('client')
        return
      }
      try {
        const data = await Data().loadClients(id)
        if (requestId !== this.requestId || this.closed || !this.$props.visible || id !== this.$props.id) return
        if (!data?.name) { this.closeModal(); return }
        this.client = createClient(JSON.parse(JSON.stringify(data)))
        this.client.inbounds = Array.isArray(this.client.inbounds) ? this.client.inbounds : []
        this.client.links = Array.isArray(this.client.links) ? this.client.links : []
        this.clientConfig = Object.fromEntries(Object.entries(this.client.config ?? {}).filter(([, value]) => value && typeof value === 'object'))
        this.links = this.client.links.filter(l => l?.type === 'local')
        this.extLinks = this.client.links.filter(l => l?.type === 'external')
        this.subLinks = this.client.links.filter(l => l?.type === 'sub')
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) {
          push.error({ message: this.$t('failed') })
          this.closeModal()
        }
      } finally {
        if (requestId === this.requestId) this.loading = false
      }
    },
    closeModal() {
      ++this.requestId
      this.closed = true
      this.loading = false
      this.client = createClient()
      this.clientConfig = this.client.config
      this.links = []
      this.extLinks = []
      this.subLinks = []
      this.$emit('close')
    },
    async saveChanges() {
      if (!this.$props.visible || this.closed || this.loading) return
      const id = this.$props.id
      if (Data().checkClientName(id, this.client.name)) return
      const requestId = ++this.requestId
      const client = JSON.parse(JSON.stringify(this.client))
      if (client.delayStart && !client.autoReset) client.expiry = 0
      client.config = updateConfigs(JSON.parse(JSON.stringify(this.clientConfig)), client.name)
      client.links = JSON.parse(JSON.stringify([...this.extLinks, ...this.subLinks].filter(l => l?.uri?.trim())))
      this.loading = true
      try {
        const success = await Data().save("clients", id == 0 ? "new" : "edit", client)
        if (requestId === this.requestId && !this.closed && this.$props.visible && id === this.$props.id && success) this.closeModal()
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) push.error({ message: this.$t('failed') })
      } finally {
        if (requestId === this.requestId) this.loading = false
      }
    },
    setDate(newDate:number){
      if (this.$props.visible && !this.closed && !this.loading) this.client.expiry = newDate
    },
    setAllInbounds(){
      this.client.inbounds = (this.inboundTags ?? []).map((i:any) => i.value).sort((a:number,b:number) => a-b)
    },
    shuffle(k?:string) {
      shuffleConfigs(this.clientConfig, k)
    },
    resetUsage(){
      this.client.totalUp = (this.client.totalUp ?? 0) + this.client.up
      this.client.totalDown = (this.client.totalDown ?? 0) + this.client.down
      this.client.up = 0
      this.client.down = 0
    },
    copyToClipboard(txt:string) {
      const hiddenButton = document.createElement('button')
      hiddenButton.className = 'clipboard-btn'
      document.body.appendChild(hiddenButton)

      const clipboard = new Clipboard('.clipboard-btn', {
        text: () => txt,
      })

      clipboard.on('success', () => {
        clipboard.destroy()
        push.success({
          message: this.$t('success') + ": " + this.$t('copyToClipboard'),
          duration: 5000,
        })
      })

      clipboard.on('error', () => {
        clipboard.destroy()
        push.error({
          message: this.$t('failed') + ": " + this.$t('copyToClipboard'),
          duration: 5000,
        })
      })

      hiddenButton.click()
      document.body.removeChild(hiddenButton)
    }
  },
  computed: {
    clientInbounds: {
      get() { return [...(this.client.inbounds ?? [])].sort((a,b) => a-b) },
      set(v:number[] | null) { this.client.inbounds = [...(v ?? [])].sort((a,b) => a-b) }
    },
    expDate: {
      get() { return this.client.expiry},
      set(v:any) { this.client.expiry = v }
    },
    Volume: {
      get() { return this.client.volume == 0 ? 0 : (this.client.volume / (1024 ** 3)) },
      set(v:number) { this.client.volume = v > 0 ? v*(1024 ** 3) : 0 }
    },
    delayStart: {
      get() { return this.client.delayStart?? false },
      set(v:boolean) {
        this.client.delayStart = v
        this.client.resetDays = (this.client.delayStart || this.client.autoReset) ? Math.max(1, this.client.resetDays ?? 1) : 0
        if (v && !this.autoReset) this.client.expiry = 0
      }
    },
    autoReset: {
      get() { return this.client.autoReset?? false },
      set(v:boolean) {
        this.client.autoReset = v
        this.client.resetDays = (this.client.delayStart || this.client.autoReset) ? Math.max(1, this.client.resetDays ?? 1) : 0
        if (!v) this.client.nextReset = 0
      }
    },
    resetDays: {
      get() { return this.client.resetDays?? 1 },
      set(v:number|null) {
        if (v == null || !Number.isFinite(Number(v)) || Number(v) < 1) v = 1
        v = Math.trunc(Number(v))
        if (this.client.nextReset && this.client.nextReset > 0) {
          this.client.nextReset += (v-(this.client.resetDays?? 0))*24*60*60
        }
        this.client.resetDays = v
      }
    },
    up() :string { return HumanReadable.sizeFormat(this.client.up) },
    down() :string { return HumanReadable.sizeFormat(this.client.down) },
    total() :string { return HumanReadable.sizeFormat(this.client.down + this.client.up) },
    totalUp() :string { return HumanReadable.sizeFormat((this.client.totalUp ?? 0) + this.client.up) },
    totalDown() :string { return HumanReadable.sizeFormat((this.client.totalDown ?? 0) + this.client.down) },
    nextResetFormatted() :string {
      const ts = this.client.nextReset?? 0
      if (ts == 0) return '-'
      const date = new Date(ts*1000)
      return date.toLocaleString(locale)
    },
    percent() :number { return this.client.volume>0 ? Math.round((this.client.up + this.client.down) *100 / this.client.volume) : 0 },
    percentColor() :string { return (this.client.up+this.client.down) >= this.client.volume ? 'error' : this.percent>90 ? 'warning' : 'success' },
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
  components: { DatePick },
}

</script>