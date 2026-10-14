<template>
  <v-row>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.grpcServiceName')"
      hide-details
      v-model="transport.service_name">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-switch
        color="primary"
        v-model="transport.permit_without_stream"
        :label="$t('transport.grpcPws')"
        hide-details>
      </v-switch>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.idleTimeout')"
      hide-details
      type="number"
      suffix="s"
      min="1"
      v-model.number="idle_timeout">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.pingTimeout')"
      hide-details
      type="number"
      suffix="s"
      min="1"
      v-model.number="ping_timeout">
      </v-text-field>
    </v-col>
  </v-row>
</template>

<script lang="ts">
import { gRPC } from '../../types/transport'
// Sing-box durations accept compound Go units, not just an integer followed by s.
function durationSeconds(value: unknown): number | '' {
  if (typeof value !== 'string' || value === '') return ''
  const sign = value.startsWith('-') ? -1 : 1
  const text = value.replace(/^[+-]/, '')
  if (text === '') return ''
  if (text === '0') return 0
  const parts = [...text.matchAll(/(\d+(?:\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h|d)/g)]
  if (parts.map(part => part[0]).join('') !== text) return ''
  const units: Record<string, number> = { ns: 1e-9, us: 1e-6, 'µs': 1e-6, 'μs': 1e-6, ms: 1e-3, s: 1, m: 60, h: 3600, d: 86400 }
  const seconds = parts.reduce((sum, part) => sum + Number(part[1]) * units[part[2]], 0)
  return Number.isFinite(seconds) && seconds <= 9_223_372_036 ? sign * seconds : ''
}

export default {
  props: ['transport'],
  data() {
    return {
    }
  },
  computed: {
    GRPC(): gRPC {
      return <gRPC> this.$props.transport?? {}
    },
    idle_timeout: {
      get() { return durationSeconds(this.GRPC.idle_timeout) },
      set(value: number | null) {
        if (!this.transport) return
        const seconds = Number(value)
        this.transport.idle_timeout = Number.isFinite(seconds) && seconds >= 1e-9 && seconds <= 9_223_372_036 ? seconds.toFixed(9).replace(/\.?0+$/, '') + 's' : undefined
      }
    },
    ping_timeout: {
      get() { return durationSeconds(this.GRPC.ping_timeout) },
      set(value: number | null) {
        if (!this.transport) return
        const seconds = Number(value)
        this.transport.ping_timeout = Number.isFinite(seconds) && seconds >= 1e-9 && seconds <= 9_223_372_036 ? seconds.toFixed(9).replace(/\.?0+$/, '') + 's' : undefined
      }
    }
  }
}
</script>