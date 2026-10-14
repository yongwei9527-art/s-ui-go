<template>
  <v-card style="padding: 8px;" rounded="xl" class="border">
    <v-card-subtitle>Shadowsocks API
      <v-chip color="primary" density="compact" variant="elevated" @click="add_server"><v-icon icon="mdi-plus" /></v-chip>
    </v-card-subtitle>
    <v-row v-for="(server, index) in servers">
      <v-col cols="auto" align-self="center" justify-self="center">
        <v-icon @click="del_server(index)" color="error" icon="mdi-delete" />
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
          :label="$t('transport.path')"
          hide-details
          @update:model-value="update_key(index,$event)"
          :model-value="server.name">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-select
          :label="$t('objects.inbound')"
          hide-details
          :items="ssTags"
          @update:model-value="update_value(index,$event)"
          :model-value="server.value">
        </v-select>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import { push } from 'notivue'
import { i18n } from '@/locales'

type Server = {
  name: string
  value: string
}
export default {
  props: ['data', 'ssTags'],
  data() {
    return {}
  },
  methods: {
    add_server() {
      const rows = this.servers
      let index = 0
      while (rows.some((row: Server) => row.name === '/ss' + index)) index++
      this.servers = [...rows, { name: '/ss' + index, value: this.ssTags?.[0] ?? '' }]
    },
    del_server(index: number) {
      const rows = [...this.servers]
      if (!Number.isInteger(index) || index < 0 || index >= rows.length) return
      rows.splice(index, 1)
      this.servers = rows
    },
    update_key(index: number, name: string | null) {
      const rows = this.servers.map((row: Server) => ({ ...row }))
      if (!Number.isInteger(index) || index < 0 || !rows[index]) return
      rows[index].name = name ?? ''
      this.servers = rows
    },
    update_value(index: number, value: string | null) {
      const rows = this.servers.map((row: Server) => ({ ...row }))
      if (!Number.isInteger(index) || index < 0 || !rows[index]) return
      rows[index].value = value ?? ''
      this.servers = rows
    },
  },
  computed: {
    servers: {
      get(): Server[] {
        const servers: Server[] = []
        const source = this.data.servers
        if (source && typeof source === 'object') {
          Object.entries(source).forEach(([name, value]) => {
            // Keep legacy array entries editable, but never save an array-valued map.
            const values = Array.isArray(value) ? value : [value]
            values.forEach(value => servers.push({ name, value: typeof value === 'string' ? value : '' }))
          })
        }
        return servers
      },
      set(rows: Server[] | null) {
        if (!rows?.length) {
          this.data.servers = undefined
          return
        }
        if (new Set(rows.map(row => row.name)).size !== rows.length) {
          push.error({ message: i18n.global.t('error.dplData') })
          return
        }
        // Object.fromEntries treats __proto__/constructor as ordinary own keys.
        this.data.servers = Object.fromEntries(rows.map(row => [row.name, row.value ?? '']))
      }
    }
  },
}
</script>