<template>
  <v-card subtitle="DERP">
    <v-row>
      <v-col cols="12" sm="8">
        <v-text-field
        :label="$t('types.derp.configPath')"
        hide-details
        v-model="data.config_path">
        </v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionHome">
      <v-col cols="12" sm="8">
        <v-text-field
        :label="$t('pages.home')"
        hide-details
        placeholder="blank | http[s]://example.com:port/path"
        v-model="home">
        </v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionVerifyCE">
      <v-col cols="12" sm="8">
        <v-select
        :label="$t('types.derp.verifyClientEndpoint')"
        hide-details
        :items="tsTags"
        multiple
        v-model="verifyClientEndpoint">
        </v-select>
      </v-col>
    </v-row>
    <template v-if="optionVerifyCU">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('types.derp.verifyClientUrl') }}</v-col>
          <v-col cols="auto" align-self="center" justify-self="center">
            <v-chip color="primary" density="compact" variant="elevated" @click="addClientUrl"><v-icon icon="mdi-plus" /></v-chip>
          </v-col>
        </v-row>
      </v-card-title>
      <v-card v-for="clientUrl, index in data.verify_client_url" :key="index" class="border" style="padding: 8px;" rounded="xl">
        <v-row>
          <v-col cols="auto" align-self="center" justify-self="center">
            <v-icon @click="delClientUrl(index)" color="error" icon="mdi-delete" />
          </v-col>
          <v-col cols="11">
            <v-text-field
            :label="$t('types.derp.verifyClientUrl')"
            hide-details
            v-model="clientUrl.url">
            </v-text-field>
            <Dial :dial="clientUrl" />
          </v-col>
        </v-row>
      </v-card>
    </template>
    <template v-if="optionMesh">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('types.derp.meshWith') }}</v-col>
          <v-col cols="auto" align-self="center" justify-self="center">
            <v-chip color="primary" density="compact" variant="elevated" @click="addMesh"><v-icon icon="mdi-plus" /></v-chip>
          </v-col>
        </v-row>
      </v-card-title>
      <v-card v-for="mesh, index in data.mesh_with" :key="index" class="border" style="padding: 8px;" rounded="xl">
        <v-row>
          <v-col cols="auto" align-self="center" justify-self="center">
            <v-icon @click="delMesh(index)" color="error" icon="mdi-delete" />
          </v-col>
          <v-col cols="11">
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                :label="$t('out.addr')"
                hide-details
                v-model="mesh.server">
                </v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                :label="$t('out.port')"
                hide-details
                type="number"
                v-model.number="mesh.server_port">
                </v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                :label="$t('transport.host')"
                hide-details
                v-model="mesh.host">
                </v-text-field>
              </v-col>
            </v-row>
            <Dial :dial="mesh" />
            <OutTLS :outbound="mesh" />
          </v-col>
        </v-row>
      </v-card>
      <v-row>
        <v-col cols="auto">
          <v-btn-toggle v-model="usePskText"
          class="rounded-xl"
          density="compact"
          variant="outlined"
          shaped
          mandatory>
            <v-btn
              :value="0"
            >{{ $t('types.derp.meshPsk') }}</v-btn>
            <v-btn
              :value="1"
            >{{ $t('types.derp.meshPskFile') }}</v-btn>
          </v-btn-toggle>
        </v-col>
      </v-row>
      <v-row v-if="usePskText == 1">
        <v-col cols="12">
          <v-text-field
            :label="$t('types.derp.meshPskFile')"
            hide-details
            v-model="meshPskFile">
          </v-text-field>
        </v-col>
      </v-row>
      <v-row v-else>
        <v-col cols="12">
          <v-text-field
            :label="$t('types.derp.meshPsk')"
            hide-details
            v-model="meshPsk">
          </v-text-field>
        </v-col>
      </v-row>
    </template>
    <template v-if="optionStun">
      <v-card :title="$t('types.derp.stun')" class="border" style="padding: 8px;" rounded="xl">
        <Listen :data="data.stun" :inTags="inTags" />
      </v-card>
    </template>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="tonal">{{ $t('types.derp.options') }}</v-btn>
        </template>
        <v-card>
          <v-list>
            <v-list-item>
              <v-switch v-model="optionVerifyCE" color="primary" :label="$t('types.derp.verifyClientEndpoint')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionVerifyCU" color="primary" :label="$t('types.derp.verifyClientUrl')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionHome" color="primary" :label="$t('pages.home')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionMesh" color="primary" :label="$t('types.derp.meshWith')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionStun" color="primary" :label="$t('types.derp.stun')" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">
import Dial from '@/components/Dial.vue'
import OutTLS from '../tls/OutTLS.vue'
import Listen from '../Listen.vue'
export default {
  props: ['data', 'tsTags', 'inTags'],
  data() {
    return {
      menu: false,
    }
  },
  methods: {
    addClientUrl() { if (Array.isArray(this.data.verify_client_url)) this.data.verify_client_url.push({ url: '' }) },
    delClientUrl(value: number | string) {
      if (typeof value === 'string' && value.trim() === '') return
      const index = Number(value)
      const rows = this.data.verify_client_url
      if (Array.isArray(rows) && Number.isInteger(index) && index >= 0 && index < rows.length) rows.splice(index, 1)
    },
    addMesh() { if (Array.isArray(this.data.mesh_with)) this.data.mesh_with.push({ tls: {} }) },
    delMesh(value: number | string) {
      if (typeof value === 'string' && value.trim() === '') return
      const index = Number(value)
      const rows = this.data.mesh_with
      if (Array.isArray(rows) && Number.isInteger(index) && index >= 0 && index < rows.length) rows.splice(index, 1)
    },
  },
  watch: {
    data: {
      immediate: true,
      handler() {
        const data = this.data
        if (!data) return
        // Normalize the core's legal listable/scalar forms before binding nested fields.
        if (typeof data.verify_client_endpoint === 'string') data.verify_client_endpoint = [data.verify_client_endpoint]
        if (data.verify_client_url != null) {
          const urls = Array.isArray(data.verify_client_url) ? data.verify_client_url : [data.verify_client_url]
          data.verify_client_url = urls.map((url: any) => typeof url === 'string' ? { url } : url)
        }
        if (data.mesh_with != null && !Array.isArray(data.mesh_with)) data.mesh_with = [data.mesh_with]
        if (typeof data.stun === 'number') data.stun = { enabled: true, listen_port: data.stun }
      }
    }
  },
  computed: {
    home: {
      get() { return this.data.home ?? '' },
      set(value: string | null) { if (this.optionHome) this.data.home = value ?? '' }
    },
    verifyClientEndpoint: {
      get() { return this.data.verify_client_endpoint ?? [] },
      set(value: string[] | null) { if (this.optionVerifyCE) this.data.verify_client_endpoint = value ?? [] }
    },
    meshPsk: {
      get() { return this.data.mesh_psk ?? '' },
      set(value: string | null) { if (this.optionMesh && this.usePskText === 0) this.data.mesh_psk = value ?? '' }
    },
    meshPskFile: {
      get() { return this.data.mesh_psk_file ?? '' },
      set(value: string | null) { if (this.optionMesh && this.usePskText === 1) this.data.mesh_psk_file = value ?? '' }
    },
    usePskText: {
      get() { return this.data.mesh_psk == null ? 1 : 0 },
      set(mode: number) {
        if (!this.optionMesh || ![0, 1].includes(mode)) return
        if (mode === 0) {
          delete this.data.mesh_psk_file
          this.data.mesh_psk ??= ''
        } else {
          delete this.data.mesh_psk
          this.data.mesh_psk_file ??= ''
        }
      }
    },
    optionVerifyCE: {
      get() { return this.$props.data.verify_client_endpoint != undefined },
      set(v: boolean) { if (v !== this.optionVerifyCE) this.data.verify_client_endpoint = v ? [] : undefined }
    },
    optionVerifyCU: {
      get() { return this.$props.data.verify_client_url != undefined },
      set(v: boolean) { if (v !== this.optionVerifyCU) this.data.verify_client_url = v ? [{ url: '' }] : undefined }
    },
    optionHome: {
      get() { return this.$props.data.home != undefined },
      set(v: boolean) { if (v !== this.optionHome) this.data.home = v ? '' : undefined }
    },
    optionMesh: {
      get() { return this.$props.data.mesh_with != undefined },
      set(v: boolean) {
        if (v) {
          if (this.optionMesh) return
          this.$props.data.mesh_with = [{tls: {}}]
          delete this.$props.data.mesh_psk_file
          this.$props.data.mesh_psk = ''
        } else {
          delete this.$props.data.mesh_with
          delete this.$props.data.mesh_psk_file
          delete this.$props.data.mesh_psk
        }
      }
    },
    optionStun: {
      get() { return this.data.stun?.enabled === true },
      set(v: boolean) {
        if (v === this.optionStun) return
        this.data.stun = v ? { ...this.data.stun, enabled: true } : undefined
      }
    }
  },
  components: { Dial, Listen, OutTLS },
}
</script>
