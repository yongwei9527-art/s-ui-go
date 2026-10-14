<template>
  <v-card subtitle="URL Test">
    <v-row>
      <v-col cols="12" sm="6">
        <v-combobox
          v-model="data.outbounds"
          :items="tags"
          :label="$t('pages.outbounds')"
          multiple
          chips
          hide-details
        ></v-combobox>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" v-if="optionUrl">
        <v-text-field v-model="url" :label="$t('types.lb.testUrl')" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="optionInterval">
        <v-text-field
        :label="$t('types.lb.interval')"
        hide-details
        type="number"
        min="3"
        :suffix="$t('date.s')"
        v-model.number="interval"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="optionTolerance">
        <v-text-field
        :label="$t('types.lb.tolerance')"
        hide-details
        type="number"
        min="0"
        :suffix="$t('date.ms')"
        v-model.number="tolerance"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="optionIdle">
        <v-text-field
        :label="$t('transport.idleTimeout')"
        hide-details
        type="number"
        min="0"
        :suffix="$t('date.m')"
        v-model.number="idle_timeout"></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6">
        <v-switch v-model="data.interrupt_exist_connections" color="primary" :label="$t('types.lb.interruptConn')" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
          <template v-slot:activator="{ props }">
            <v-btn v-bind="props" hide-details variant="tonal">{{ $t('types.lb.urlTestOptions') }}</v-btn>
          </template>
          <v-card>
            <v-list>
              <v-list-item>
                <v-switch v-model="optionUrl" color="primary" :label="$t('types.lb.testUrl')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionInterval" color="primary" :label="$t('types.lb.interval')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionTolerance" color="primary" :label="$t('types.lb.tolerance')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionIdle" color="primary" :label="$t('transport.idleTimeout')" hide-details></v-switch>
              </v-list-item>
            </v-list>
          </v-card>
        </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">

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
  props: ['data', 'tags'],
  data() {
    return {
      menu: false,
    }
  },
  computed: {
    url: {
      get() { return this.data.url ?? '' },
      set(value: string | null) { if (this.optionUrl) this.data.url = value ?? '' }
    },
    optionUrl: {
      get(): boolean { return this.$props.data.url != undefined },
      set(v:boolean) { if (v !== this.optionUrl) this.data.url = v ? 'https://www.gstatic.com/generate_204' : undefined }
    },
    optionInterval: {
      get(): boolean { return this.$props.data.interval != undefined },
      set(v:boolean) { if (v !== this.optionInterval) this.data.interval = v ? '3s' : undefined }
    },
    optionTolerance: {
      get(): boolean { return this.$props.data.tolerance != undefined },
      set(v:boolean) { if (v !== this.optionTolerance) this.data.tolerance = v ? 50 : undefined }
    },
    optionIdle: {
      get(): boolean { return this.$props.data.idle_timeout != undefined },
      set(v:boolean) { if (v !== this.optionIdle) this.data.idle_timeout = v ? '30m' : undefined }
    },
    interval: {
      get() { return durationSeconds(this.data.interval) || 180 },
      set(v: number | null) {
        if (!this.optionInterval) return
        const value = Number(v)
        if (Number.isFinite(value) && value >= 3 && value <= 9_223_372_036) this.data.interval = value.toFixed(9).replace(/\.?0+$/, '') + 's'
      }
    },
    tolerance: {
      get() { return this.data.tolerance || 50 },
      set(v: number | string | null) {
        if (!this.optionTolerance || v == null || v === '') return
        const value = Number(v)
        if (Number.isInteger(value) && value >= 0 && value <= 65535) this.data.tolerance = value
      }
    },
    idle_timeout: {
      get() { return (durationSeconds(this.data.idle_timeout) || 1800) / 60 },
      set(v: number | null) {
        if (!this.optionIdle) return
        const value = Number(v)
        if (Number.isFinite(value) && value * 60 >= 1e-9 && value * 60 <= 9_223_372_036) this.data.idle_timeout = (value * 60).toFixed(9).replace(/\.?0+$/, '') + 's'
      }
    }
  },
}
</script>