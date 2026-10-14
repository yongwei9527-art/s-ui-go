<template>
  <v-card subtitle="ShadowTls">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :items="[1,2,3]"
          :label="$t('version')"
          v-model="version">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="version > 1">
        <v-text-field
        :label="$t('types.pw')"
        hide-details
        v-model="password">
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['data'],
  data() {
    return {}
  },
  computed: {
    password: {
      get() { return this.data.password ?? '' },
      set(v: string | null) { if (this.version > 1) this.data.password = v ?? '' }
    },
    version: {
      get() { return this.$props.data.version || 1 },
      set(v: number) {
        if (![1, 2, 3].includes(v)) return
        this.$props.data.version = v
        if (v==1) {
          delete this.$props.data.password
        } else if (this.$props.data.password === undefined ) {
          this.$props.data.password = ""
        }
      }
    },
  }
}
</script>