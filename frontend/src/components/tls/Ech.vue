<template>
  <v-card subtitle="ECH" style="background-color: inherit;">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" :label="$t('enable')" v-model="enabled" hide-details></v-switch>
      </v-col>
    </v-row>
    <template v-if="enabled">
      <v-row>
        <v-col cols="auto">
          <v-btn-toggle v-model="useEchPath"
          class="rounded-xl"
          density="compact"
          variant="outlined"
          shaped
          mandatory>
            <v-btn
              @click="delete ech.key"
            >{{ $t('tls.usePath') }}</v-btn>
            <v-btn
              @click="delete ech.key_path"
            >{{ $t('tls.useText') }}</v-btn>
          </v-btn-toggle>
        </v-col>
        <v-spacer></v-spacer>
        <v-col cols="auto">
          <v-btn
            variant="tonal"
            density="compact"
            icon="mdi-key-star"
            @click="genECH"
            :loading="loading">
            <v-icon />
            <v-tooltip activator="parent" location="top">
              {{ $t('actions.generate') }}
            </v-tooltip>
          </v-btn>
        </v-col>
      </v-row>
      <v-row v-if="useEchPath == 0">
        <v-col cols="12">
          <v-text-field
            :label="$t('tls.keyPath')"
            hide-details
            v-model="ech.key_path">
          </v-text-field>
        </v-col>
      </v-row>
      <v-row v-else>
        <v-col cols="12">
          <v-textarea
            :label="$t('tls.key')"
            hide-details
            v-model="echKeyText">
          </v-textarea>
        </v-col>
      </v-row>
      <v-row>
        <v-col cols="12" sm="6" v-if="oTls.ech">
          <v-text-field
            :label="$t('tls.queryServerName')"
            hide-details
            v-model="oTls.ech.query_server_name"
            placeholder="ech.example.com">
          </v-text-field>
        </v-col>
      </v-row>
      <v-row>
        <v-col cols="12">
          <v-textarea
            :label="$t('tls.cert')"
            hide-details
            v-model="echConfigText">
          </v-textarea>
        </v-col>
      </v-row>
    </template>
  </v-card>
</template>

<script lang="ts">
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { ech } from '@/types/tls'
import { push } from 'notivue'

export default {
  props: ['iTls','oTls'],
  data() {
    return {
      useEchPath: this.$props.iTls?.ech?.key? 1:0,
      loading: false,
      requestId: 0,
    }
  },
  methods: {
    cancelGeneration() {
      this.requestId++
      this.loading = false
    },
    async genECH(){
      if (this.loading || !this.iTls.ech || !this.oTls.ech) return
      const requestId = ++this.requestId
      const iTls = this.iTls
      const oTls = this.oTls
      const ech = iTls.ech
      const clientEch = oTls.ech
      const serverName = iTls.server_name
      this.loading = true
      try {
        const msg = await HttpUtils.get('api/keypairs', {
          k: "ech",
          o: serverName?? "''"
        })
        if (requestId !== this.requestId || this.iTls !== iTls || this.oTls !== oTls ||
            iTls.ech !== ech || oTls.ech !== clientEch || iTls.server_name !== serverName) return
        if (msg.success) {
          if (Array.isArray(msg.obj) && msg.obj.length > 0 && msg.obj.every(line => typeof line === 'string')) {
            const config = <string[]>[]
            const key = <string[]>[]
            let isConfig = false
            let isKey = false

            msg.obj.forEach((line:string) => {
              if (line === "-----BEGIN ECH CONFIGS-----") {
                isConfig = true
                isKey = false
                config.push(line)
              } else if (line === "-----END ECH CONFIGS-----") {
                isConfig = false
                config.push(line)
              } else if (line === "-----BEGIN ECH KEYS-----") {
                isKey = true
                isConfig = false
                key.push(line)
              } else if (line === "-----END ECH KEYS-----") {
                isKey = false
                key.push(line)
              } else if (isConfig) {
                config.push(line)
              } else if (isKey) {
                key.push(line)
              }
            })
            if (key.length < 3 || key[0] !== '-----BEGIN ECH KEYS-----' || key.at(-1) !== '-----END ECH KEYS-----' ||
                config.length < 3 || config[0] !== '-----BEGIN ECH CONFIGS-----' || config.at(-1) !== '-----END ECH CONFIGS-----') {
              throw new Error('Missing ECH key or config')
            }
            ech.key_path = undefined
            this.useEchPath = 1
            ech.key = key
            clientEch.config = config
          } else {
            throw new Error('Invalid ECH response')
          }
        }
      } catch {
        if (requestId === this.requestId && this.iTls === iTls && this.oTls === oTls &&
            iTls.ech === ech && oTls.ech === clientEch) {
          push.error({ message: i18n.global.t('error.invalidData') })
        }
      } finally {
        if (requestId === this.requestId) this.loading = false
      }
    },
  },
  watch: {
    iTls() { this.cancelGeneration() },
    oTls() { this.cancelGeneration() },
  },
  beforeUnmount() { this.cancelGeneration() },
  computed: {
    ech() {
      return <ech>this.$props.iTls.ech
    },
    enabled: {
      get() { return this.ech?.enabled?? false },
      set(v: boolean) {
        this.cancelGeneration()
        this.$props.iTls.ech = v ? { enabled: true } : undefined
        this.$props.oTls.ech = v ? {} : undefined
      }
    },
    echKeyText: {
      get(): string { return this.ech?.key ? this.ech.key.join('\n') : '' },
      set(newValue:string) { newValue ??= ''; if (this.ech) this.ech.key = newValue.split('\n') }
    },
    echConfigText: {
      get(): string { return this.oTls.ech?.config ? this.oTls.ech.config.join('\n') : '' },
      set(newValue:string) { newValue ??= ''; if (this.oTls.ech) this.oTls.ech.config = newValue.split('\n') }
    },
  }
}
</script>
