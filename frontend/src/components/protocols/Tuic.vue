<template>
  <v-card subtitle="TUIC">
    <v-alert
      v-if="direction == 'in'"
      color="info"
      icon="mdi-lan-pending"
      variant="tonal"
      density="compact"
      class="mb-3"
    >
      {{ $t('inbounds.tuicNotice') }}
    </v-alert>
    <v-row v-if="direction == 'out'">
      <v-col cols="12" sm="6">
        <v-text-field v-model="data.uuid" label="UUID" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="data.password" :label="$t('types.pw')" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <Network :data="data" />
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          label="UDP Relay Mode"
          :items="['native', 'quic']"
          clearable
          @click:clear="delete data.udp_relay_mode"
          v-model="data.udp_relay_mode">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" label="UDP Over Stream" v-model="data.udp_over_stream" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :label="$t('types.tuic.congControl')"
          :items="congestion_controls"
          v-model="data.congestion_control">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" label="Zero-RTT Handshake" v-model="data.zero_rtt_handshake" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="direction == 'in'">
        <v-text-field
        :label="$t('types.tuic.authTimeout')"
        hide-details
        type="number"
        :suffix="$t('date.s')"
        min="1"
        v-model.number="auth_timeout">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('types.tuic.hb')"
        hide-details
        type="number"
        :suffix="$t('date.s')"
        min="1"
        v-model.number="heartbeat">
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import Network from '@/components/Network.vue'

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
  props: ['direction', 'data'],
  data() {
    return {
      congestion_controls: [
        "cubic","new_reno", "bbr"
      ]
    }
  },
  computed: {
    auth_timeout: {
      get() { return durationSeconds(this.data?.auth_timeout) },
      set(value: number | null) {
        if (!this.data) return
        const seconds = Number(value)
        this.data.auth_timeout = Number.isFinite(seconds) && seconds >= 1e-9 && seconds <= 9_223_372_036 ? seconds.toFixed(9).replace(/\.?0+$/, '') + 's' : undefined
      }
    },
    heartbeat: {
      get() { return durationSeconds(this.data?.heartbeat) },
      set(value: number | null) {
        if (!this.data) return
        const seconds = Number(value)
        this.data.heartbeat = Number.isFinite(seconds) && seconds >= 1e-9 && seconds <= 9_223_372_036 ? seconds.toFixed(9).replace(/\.?0+$/, '') + 's' : undefined
      }
    }
  },
  components: { Network }
}
</script>