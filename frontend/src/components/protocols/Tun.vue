<template>
  <v-card subtitle="Tun">
    <v-row class="tun-row" align="start">
      <v-col cols="12">
        <v-alert type="info" variant="tonal" density="compact">
          {{ $t('types.tun.windowsMultiNicHint') }}
        </v-alert>
      </v-col>
    </v-row>
    <v-row class="tun-row" align="start">
      <v-col cols="12" sm="8">
        <v-text-field v-model="addrs" :label="$t('types.tun.addr') + ' ' + $t('commaSeparated')" placeholder="172.18.0.1/30" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row class="tun-row" align="start">
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="data.interface_name" :label="$t('types.tun.ifName')" placeholder="tun0" hide-details clearable @click:clear="delete data.interface_name"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field type="number" v-model.number="data.mtu" label="MTU" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row class="tun-row" align="start">
      <v-col cols="12" sm="6" md="4">
        <v-text-field
          type="number"
          v-model.number="udpTimeout"
          label="UDP timeout"
          min="1"
          :suffix="$t('date.m')"
          hide-details>
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-select
          v-model="data.stack"
          label="Stack"
          :items="['system','gvisor','mixed']"
          hide-details
        ></v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4" class="switch-col">
        <v-switch v-model="data.endpoint_independent_nat" color="primary" label="Independent NAT" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row class="tun-row" align="start">
      <v-col cols="12" sm="6" md="4" class="switch-col">
        <v-switch v-model="autoRoute" color="primary" label="Auto Route" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="autoRoute" class="switch-col">
        <v-switch v-model="data.auto_redirect" color="primary" label="Auto Redirect" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="autoRoute" class="switch-col">
        <v-switch v-model="data.strict_route" color="primary" label="Strict Route" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="autoRoute" class="switch-col">
        <v-switch v-model="dnsHijack" color="primary" label="DNS hijack" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="autoRoute && data.auto_redirect" class="switch-col">
        <v-switch v-model="data.exclude_mptcp" color="primary" :label="$t('types.tun.excludeMptcp')" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="autoRoute && data.auto_redirect">
        <v-text-field
          type="number"
          v-model.number="fallbackRuleIndex"
          :label="$t('types.tun.fallbackRuleIndex')"
          min="0"
          hide-details>
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['data'],
  data() {
    return {
      menu: false
    }
  },
  computed: {
    addrs: {
      get() { return this.$props.data.address?.join(',') },
      set(v:string) { this.$props.data.address = v.length > 0 ? v.split(',') : undefined }
    },
    udpTimeout: {
      get() { return this.$props.data.udp_timeout ? parseInt(this.$props.data.udp_timeout.replace('m','')) : 5 },
      set(v:number) { this.$props.data.udp_timeout = v > 0 ? v + 'm' : '5m' }
    },
    autoRoute: {
      get() { return this.$props.data.auto_route ?? false },
      set(v:boolean) {
        this.$props.data.auto_route = v
        this.$props.data.auto_redirect = v ? false : undefined
        this.$props.data.strict_route = v ? false : undefined
      }
    },
    fallbackRuleIndex: {
      get() { return this.$props.data.auto_redirect_iproute2_fallback_rule_index ?? 32768 },
      set(v: number) {
        const val = typeof v === 'number' && !isNaN(v) && v >= 0 ? v : undefined
        this.$props.data.auto_redirect_iproute2_fallback_rule_index = val
      }
    },
    dnsHijack: {
      get() { return (this.$props.data.dns_hijack?.length ?? 0) > 0 || (this.$props.data.dns_hijack_address?.length ?? 0) > 0 },
      set(v:boolean) {
        if (v) {
          this.$props.data.dns_hijack = ['any:53']
        } else {
          delete this.$props.data.dns_hijack
          delete this.$props.data.dns_hijack_address
        }
      }
    }
  }
}
</script>

<style scoped>
.tun-row {
  row-gap: 8px;
}

.switch-col {
  display: flex;
  align-items: center;
  min-height: 56px;
}
</style>
