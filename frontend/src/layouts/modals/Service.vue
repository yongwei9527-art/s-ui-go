<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.service') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-select
            hide-details
            :label="$t('type')"
            :items="Object.keys(srvTypes).map((key,index) => ({title: key, value: Object.values(srvTypes)[index]}))"
            v-model="srv.type"
            @update:modelValue="changeType">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="srv.tag" :label="$t('objects.tag')" hide-details></v-text-field>
          </v-col>
        </v-row>

        <Listen :data="srv" :inTags="inTags" />
        <Derp v-if="srv.type == srvTypes.DERP" :data="srv" :inTags="inTags" :tsTags="tsTags" />
        <SSMapi v-if="srv.type == srvTypes.SSMAPI" :data="srv" :ssTags="ssTags" />
        <Ocm v-if="srv.type == srvTypes.OCM" :data="srv" />
        <Ccm v-if="srv.type == srvTypes.CCM" :data="srv" />
        <InTLS v-if="HasTls.includes(srv.type)"  :inbound="srv" :tlsConfigs="tlsConfigs" :tls_id="srv.tls_id" />
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="primary"
          variant="outlined"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn
          color="primary"
          variant="tonal"
          :loading="loading"
          @click="saveChanges"
        >
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { SrvTypes, createSrv } from '@/types/services'
import RandomUtil from '@/plugins/randomUtil'
import Listen from '@/components/Listen.vue'
import Derp from '@/components/services/Derp.vue'
import Ocm from '@/components/services/Ocm.vue'
import Ccm from '@/components/services/Ccm.vue'
import InTLS from '@/components/tls/InTLS.vue'
import SSMapi from '@/components/services/SSMAPI.vue'
import Data from '@/store/modules/data'
import { push } from 'notivue'
import { i18n } from '@/locales'
export default {
  props: ['visible', 'data', 'id', 'inTags', 'tsTags', 'ssTags', 'tlsConfigs'],
  emits: ['close'],
  data() {
    return {
      srv: createSrv("derp",{ "tag": "" }),
      title: "add",
      tab: "t1",
      requestId: 0,
      closed: false,
      loading: false,
      srvTypes: SrvTypes,
      HasTls: [SrvTypes.DERP, SrvTypes.SSMAPI, SrvTypes.OCM, SrvTypes.CCM],
    }
  },
  methods: {
    updateData(id: number) {
      ++this.requestId
      this.loading = false
      try {
        if (id > 0) {
          const newData = JSON.parse(this.$props.data)
          if (!newData || typeof newData !== 'object' || Array.isArray(newData)) throw new Error('Invalid service')
          this.srv = createSrv(newData.type, newData)
          this.title = "edit"
        }
        else {
          const port = RandomUtil.randomIntRange(10000, 60000)
          this.srv = createSrv("derp", {
            tag: "derp-" + RandomUtil.randomSeq(3),
            listen: '::',
            listen_port: port,
          })
          this.title = "add"
        }
        this.tab = "t1"
      } catch { this.closeModal() }
    },
    changeType() {
      // Tag change only in add service
      const tag = this.$props.id > 0 ? this.srv.tag : this.srv.type + "-" + RandomUtil.randomSeq(3)
      // Use previous data
      const prevConfig = { id: this.srv.id, tag: tag, listen: this.srv.listen, listen_port: this.srv.listen_port }
      this.srv = createSrv(this.srv.type, prevConfig)
    },
    closeModal() {
      ++this.requestId
      this.closed = true
      this.loading = false
      this.$emit('close')
    },
    async saveChanges() {
      if (!this.$props.visible || this.closed || this.loading) return

      // check duplicate tag
      const isDuplicatedTag = Data().checkTag("service",this.srv.id, this.srv.tag)
      if (isDuplicatedTag) return

      // save data
      const requestId = ++this.requestId
      const id = this.$props.id
      this.loading = true
      try {
        const success = await Data().save("services", id == 0 ? "new" : "edit", JSON.parse(JSON.stringify(this.srv)))
        if (requestId === this.requestId && !this.closed && this.$props.visible && id === this.$props.id && success) this.closeModal()
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) push.error({ message: i18n.global.t('failed') })
      } finally {
        if (requestId === this.requestId) this.loading = false
      }
    },
  },
  beforeUnmount() { ++this.requestId; this.closed = true },
  watch: {
    visible: {
      immediate: true,
      handler(v: boolean) {
        this.closed = !v
        if (v) this.updateData(this.$props.id)
        else { ++this.requestId; this.loading = false }
      },
    },
    id() { if (this.$props.visible && !this.closed) this.updateData(this.$props.id) },
  },
  components: { Listen, InTLS, Derp, Ocm, Ccm, SSMapi },
}
</script>