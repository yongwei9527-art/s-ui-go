<template>
  <v-card subtitle="Selector">
    <v-row>
      <v-col cols="12" sm="6">
        <v-combobox
          :model-value="data.outbounds ?? []"
          :items="tags"
          :label="$t('pages.outbounds')"
          multiple
          @update:model-value="updateDefault"
          chips
          hide-details
        ></v-combobox>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-select
          v-model="defaultOutbound"
          :items="data.outbounds ?? []"
          :label="$t('types.lb.defaultOut')"
          clearable
          hide-details
        ></v-select>
      </v-col>
      <v-col cols="12" sm="6">
        <v-switch v-model="data.interrupt_exist_connections" color="primary" :label="$t('types.lb.interruptConn')" hide-details></v-switch>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['data','tags'],
  data() {
    return {}
  },
  computed: {
    defaultOutbound: {
      get() { return this.data.default ?? null },
      set(value: string | null) {
        if (!value) delete this.data.default
        else if (this.data.outbounds?.includes(value)) this.data.default = value
      }
    }
  },
  methods: {
    updateDefault(value: string[] | null) {
      this.data.outbounds = Array.isArray(value) ? value.filter(tag => typeof tag === 'string') : []
      if (!this.data.outbounds.includes(this.data.default)) delete this.data.default
    }
  },
}
</script>