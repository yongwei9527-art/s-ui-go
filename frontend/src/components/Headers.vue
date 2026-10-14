<template>
  <v-card v-if="data">
    <v-card-text>
      <v-card-subtitle>
        {{ $t('objects.headers') }}
        <v-chip color="primary" density="compact" variant="elevated" @click="add_header">
        <v-icon icon="mdi-plus" />
        </v-chip>
      </v-card-subtitle>
      <v-row v-for="(header, index) in hdrs">
        <v-col cols="12" sm="6" md="4">
          <v-text-field
            :label="$t('objects.key')"
            hide-details
            @update:model-value="update_key(index,$event)"
            :model-value="header.name">
          </v-text-field>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-text-field
            :label="$t('objects.value')"
            hide-details
            @update:model-value="update_value(index,$event)"
            :model-value="header.value">
            <template v-slot:append>
              <v-icon @click="del_header(index)" color="error" icon="mdi-delete" />
            </template>
          </v-text-field>
        </v-col>
      </v-row>
      </v-card-text>
    </v-card>
</template>

<script lang="ts">

type Header = {
  name: string
  value: string
}
export default {
  props: ['data'],
  data() {
    return {}
  },
  methods: {
    add_header() {
      this.hdrs = [...this.hdrs, {name: "Host", value: ""}]
    },
    del_header(i:number) {
      const h = [...this.hdrs]
      if (!Number.isInteger(i) || i < 0 || i >= h.length) return
      h.splice(i,1)
      this.hdrs = h
    },
    update_key(i:number,k:string | null) {
      const h = this.hdrs.map((header: Header) => ({ ...header }))
      if (!Number.isInteger(i) || i < 0 || !h[i]) return
      h[i].name = k ?? ''
      this.hdrs = h
    },
    update_value(i:number,v:string | null) {
      const h = this.hdrs.map((header: Header) => ({ ...header }))
      if (!Number.isInteger(i) || i < 0 || !h[i]) return
      h[i].value = v ?? ''
      this.hdrs = h
    },
  },
  computed: {
    hdrs: {
      get() :Header[] {
        let headers: Header[] = []
        const h = this.$props.data?.headers
        if (h) {
          Object.keys(h).forEach(key => {
            if (Array.isArray(h[key])){
              h[key].forEach((v:string) => headers.push({ name: key, value: v }))
            } else {
              headers.push({ name: key, value: h[key] })
            }
          })
        }
        return headers
      },
      set(v:Header[] | null) {
        if (!this.data) return
        if (v?.length) {
          const groups = new Map<string, string[]>()
          for (const header of v) {
            const values = groups.get(header.name) ?? []
            values.push(header.value ?? '')
            groups.set(header.name, values)
          }
          // Group by presence, not truthiness; preserve empty first values and
          // treat __proto__/constructor as ordinary own header names.
          this.data.headers = Object.fromEntries([...groups].map(([key, values]) => [key, values.length === 1 ? values[0] : values]))
        } else {
          this.$props.data.headers = undefined
        }
      }
    }
  }
}
</script>
