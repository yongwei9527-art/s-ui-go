<template>
  <v-dialog transition="dialog-bottom-transition" width="400">
    <v-card class="rounded-lg" id="qrcode-modal" :loading="loading">
      <v-card-title>
        <v-row>
          <v-col>QrCode</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto"><v-icon icon="mdi-close-box" @click="$emit('close')" /></v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-skeleton-loader
          class="mx-auto border"
          width="80%"
          type="text, image, divider, text, image"
          v-if="loading"
        ></v-skeleton-loader>
      <v-card-text v-if="clientSub" style="overflow-y: auto; padding: 0" :hidden="loading">
        <v-tabs
          v-model="tab"
          density="compact"
          fixed-tabs
          align-tabs="center"
        >
          <v-tab value="sub">{{ $t('setting.sub') }}</v-tab>
          <v-tab value="link">{{ $t('client.links') }}</v-tab>
        </v-tabs>
        <v-window v-model="tab" style="margin-top: 10px;">
          <v-window-item value="sub">
            <v-row>
              <v-col style="text-align: center;">
                <v-chip>{{ $t('setting.sub') }}</v-chip><br />
                <QrcodeVue :value="clientSub" :size="size" @click="copyToClipboard(clientSub)" :margin="1" style="border-radius: 1rem; cursor: copy;" />
                <div class="text-caption text-medium-emphasis">{{ $t('client.clickQrCopy') }}</div>
                <v-text-field dir="ltr" density="compact" variant="underlined" readonly hide-details :model-value="clientSub" append-inner-icon="mdi-content-copy" @click:append-inner="copyToClipboard(clientSub)" />
              </v-col>
            </v-row>
            <v-row>
              <v-col style="text-align: center;">
                <v-chip>{{ $t('setting.jsonSub') }}</v-chip><br />
                <QrcodeVue :value="clientSub + '?format=json'" :size="size" @click="copyToClipboard(clientSub + '?format=json')" :margin="1" style="border-radius: 1rem; cursor: copy;" />
                <div class="text-caption text-medium-emphasis">{{ $t('client.clickQrCopy') }}</div>
                <v-text-field dir="ltr" density="compact" variant="underlined" readonly hide-details :model-value="clientSub + '?format=json'" append-inner-icon="mdi-content-copy" @click:append-inner="copyToClipboard(clientSub + '?format=json')" />
              </v-col>
            </v-row>
            <v-row>
              <v-col style="text-align: center;">
                <v-chip>{{ $t('setting.clashSub') }}</v-chip><br />
                <v-alert class="mt-2 text-left" density="compact" type="info" variant="tonal">
                  {{ $t('setting.clashSubCompatibilityNotice') }}
                </v-alert>
                <QrcodeVue :value="clientSub + '?format=clash'" :size="size" @click="copyToClipboard(clientSub + '?format=clash')" :margin="1" style="border-radius: 1rem; cursor: copy;" />
                <div class="text-caption text-medium-emphasis">{{ $t('client.clickQrCopy') }}</div>
                <v-text-field dir="ltr" density="compact" variant="underlined" readonly hide-details :model-value="clientSub + '?format=clash'" append-inner-icon="mdi-content-copy" @click:append-inner="copyToClipboard(clientSub + '?format=clash')" />
              </v-col>
            </v-row>
            <v-row>
              <v-col style="text-align: center;">
                <v-chip>SING-BOX (scan only)</v-chip><br />
                <QrcodeVue :value="singbox" :size="size" :margin="1" style="border-radius: .8rem; cursor: not-allowed;" />
              </v-col>
            </v-row>
          </v-window-item>
          <v-window-item value="link">
            <v-row v-for="l in clientLinks">
              <v-col style="text-align: center;">
                <v-chip>{{ l.remark?? $t('client.' + l.type) }}</v-chip><br />
                <QrcodeVue :value="l.uri" :size="size" @click="copyToClipboard(l.uri)" :margin="1" style="border-radius: .5rem; cursor: copy;" />
                <div class="text-caption text-medium-emphasis">{{ $t('client.clickQrCopy') }}</div>
                <v-text-field dir="ltr" density="compact" variant="underlined" readonly hide-details :model-value="l.uri" append-inner-icon="mdi-content-copy" @click:append-inner="copyToClipboard(l.uri)" />
              </v-col>
            </v-row>
          </v-window-item>
        </v-window>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import QrcodeVue from 'qrcode.vue'
import Data from '@/store/modules/data'
import Clipboard from 'clipboard'
import { i18n } from '@/locales'
import { push } from 'notivue'

export default {
  props: ['id', 'visible'],
  data() {
    return {
      tab: "sub",
      client: <any>{},
      loading: false,
      loadRequest: 0,
    }
  },
  methods: {
    async load() {
      const request = ++this.loadRequest
      const id = this.$props.id
      // Never leave a previous user's subscription visible during a new load.
      this.client = {}
      this.loading = true
      try {
        const newData = await Data().loadClients(id)
        if (request === this.loadRequest && id === this.$props.id && this.$props.visible) {
          this.client = newData && typeof newData === 'object' && !Array.isArray(newData) ? newData : {}
        }
      } catch (error) {
        if (request === this.loadRequest && this.$props.visible) {
          push.error({ message: error instanceof Error ? error.message : String(error) })
        }
      } finally {
        if (request === this.loadRequest) this.loading = false
      }
    },
    copyToClipboard(txt:string) {
      const hiddenButton = document.createElement('button')
      hiddenButton.className = 'clipboard-btn'
      document.body.appendChild(hiddenButton)

      const clipboard = new Clipboard('.clipboard-btn', {
        text: () => txt,
        container: document.getElementById('qrcode-modal')?? undefined
      });

      clipboard.on('success', () => {
        clipboard.destroy()
        push.success({
          message: i18n.global.t('success') + ": " + i18n.global.t('copyToClipboard'),
          duration: 5000,
        })
      })

      clipboard.on('error', () => {
        clipboard.destroy()
        push.error({
          message: i18n.global.t('failed') + ": " + i18n.global.t('copyToClipboard'),
          duration: 5000,
        })
      })

      // Perform click on hidden button to trigger copy
      hiddenButton.click()
      document.body.removeChild(hiddenButton)
    }
  },
  computed: {
    clientSub() {
      return this.client.name && Data().subURI ? Data().subURI + this.client.name : ''
    },
    singbox() {
      if (!this.clientSub) return ''
      const url = this.clientSub + "?format=json"
      return "sing-box://import-remote-profile?url=" +  encodeURIComponent(url) + "#" + this.client.name
    },
    clientLinks() {
      return Array.isArray(this.client.links) ? this.client.links : []
    },
    size() {
      if (window.innerWidth > 380) return 300
      if (window.innerWidth > 330) return 280
      return 250
    }
  },
  watch: {
    visible(v) {
      ++this.loadRequest
      this.loading = false
      this.client = {}
      if (v) {
        this.tab = "sub"
        this.load()
      }
    },
    id() {
      if (this.$props.visible) this.load()
    },
  },
  components: { QrcodeVue }
}
</script>
