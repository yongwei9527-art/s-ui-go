<template>
  <v-dialog transition="dialog-bottom-transition" width="800" :model-value="visible">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.addbulk') }} {{ $t('objects.outbound') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-row v-if="outbounds.length==0">
          <v-col cols="12">
            <v-text-field v-model="link"
              dir="ltr"
              :label="$t('client.sub')"
              placeholder="http[s]://<domain>[:]<port>/<path>"
              hide-details />
          </v-col>
          <v-col cols="12">
            <v-checkbox v-model="addUrlTest" :label="$t('out.addUrlTest')" />
          </v-col>
          <v-col cols="12" align="center">
            <v-btn hide-details variant="tonal" :loading="loading" @click="linkCheck">{{ $t('submit') }}</v-btn>
          </v-col>
        </v-row>
        <v-data-table
          v-if="outbounds.length>0"
          :items="outbounds"
          :loading="loading"
          :items-per-page="0"
          hide-default-footer
          density="compact"
          :headers="[
            { value: 'check' },
            { title: $t('type'), value: 'type' },
            { title: $t('objects.tag'), value: 'tag' },
            { title: $t('out.addr'), value: 'server' },
            { title: $t('objects.tls'), value: 'tls' }
          ]"
        >
          <template v-slot:[`item.check`]="{ index }">
            <v-icon color="success" icon="mdi-check" v-if="outChecks[index]==1" />
            <v-icon color="error" icon="mdi-close" v-else-if="outChecks[index]==2" />
            <v-progress-circular v-else-if="outChecks[index]==3" indeterminate />
            <v-icon v-else icon="mdi-help"></v-icon>
          </template>
          <template v-slot:[`item.type`]="{ item }">
            {{ item.type }}
          </template>
          <template v-slot:[`item.tag`]="{ item }">
            {{ item.tag }}
          </template>
          <template v-slot:[`item.tls`]="{ item }">
            {{ Object.hasOwn(item,'tls') ? $t(item.tls?.enabled ? 'enable' : 'disable') : '-' }}
          </template>
          <template v-slot:[`item.server`]="{ item }">
            {{ item.server }}{{ item.server_port ? ':' + item.server_port : '' }}
          </template>
        </v-data-table>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn color="primary" variant="outlined" @click="closeModal">{{ $t('actions.close') }}</v-btn>
        <v-btn color="primary" variant="tonal" :loading="loading" :disabled="outbounds.length==0" @click="saveChanges">{{ $t('actions.save') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import HttpUtils from '@/plugins/httputil'
import { push } from 'notivue'
import { i18n } from '@/locales'
import RandomUtil from '@/plugins/randomUtil';
import Data from '@/store/modules/data'
import { createOutbound, Outbound } from '@/types/outbounds'

export default {
  props: ['visible', 'outboundTags'],
  emits: ['close'],
  data() {
    return {
      requestId: 0,
      closed: false,
      loading: false,
      link: "",
      outbounds: <Outbound[]>[],
      outChecks: <number[]>[],
      addUrlTest: false,
    }
  },
  methods: {
    resetData() {
      ++this.requestId
      this.outbounds = []
      this.outChecks = []
      this.link = ""
      this.addUrlTest = false
      this.loading = false
    },
    closeModal() {
      this.closed = true
      this.resetData()
      this.$emit('close')
    },
    async linkCheck() {
      if (!this.$props.visible || this.closed) return
      const requestId = ++this.requestId
      const link = this.link
      const addUrlTest = this.addUrlTest
      this.loading = true
      this.outbounds = []
      this.outChecks = []
      try {
        const msg = await HttpUtils.post('api/subConvert', { link })
        if (requestId !== this.requestId || this.closed || !this.$props.visible) return
        if (msg?.success && Array.isArray(msg.obj)) {
          const usedTags = new Set<string>(this.outboundTags ?? [])
          const uniqueTag = (tag: string) => {
            let unique = tag
            let suffix = 1
            while (usedTags.has(unique)) unique = tag + '-' + suffix++
            usedTags.add(unique)
            return unique
          }
          this.outbounds = msg.obj.filter((o: any) => o && typeof o.type === 'string').map((o: any) => {
            const data = JSON.parse(JSON.stringify(o))
            data.tag = uniqueTag(typeof data.tag === 'string' && data.tag ? data.tag : data.type)
            return createOutbound(data.type, data)
          })
          if (addUrlTest && this.outbounds.length) {
            this.outbounds.push(createOutbound("urltest", {
              tag: uniqueTag("urltest-" + RandomUtil.randomSeq(3)),
              outbounds: this.outbounds.map((o: Outbound) => o.tag),
              interrupt_exist_connections: false, interval: "30s",
            }))
          }
          this.outChecks = this.outbounds.map(() => 0)
        }
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) push.error({ message: i18n.global.t('failed') })
      } finally {
        if (requestId === this.requestId) this.loading = false
      }
    },
    async saveChanges() {
      if (!this.$props.visible || this.closed || this.loading || !this.outbounds.length) return
      const requestId = ++this.requestId
      const outbounds: Outbound[] = JSON.parse(JSON.stringify(this.outbounds))
      this.loading = true
      try {
        const tags = new Set<string>()
        for (let index = 0; index < outbounds.length; index++) {
          if (requestId !== this.requestId || this.closed || !this.$props.visible) return
          const outbound = outbounds[index]
          if (this.outChecks[index] === 1) { tags.add(outbound.tag); continue }
          if (tags.has(outbound.tag) || Data().checkTag("outbound", 0, outbound.tag)) {
            this.outChecks[index] = 2
            continue
          }
          tags.add(outbound.tag)
          this.outChecks[index] = 3
          try {
            const success = await Data().save("outbounds", "new", outbound)
            if (requestId !== this.requestId || this.closed || !this.$props.visible) return
            this.outChecks[index] = success ? 1 : 2
          } catch {
            if (requestId !== this.requestId || this.closed || !this.$props.visible) return
            this.outChecks[index] = 2
            push.error({ message: i18n.global.t('failed') })
          }
        }
      } finally {
        if (requestId === this.requestId) this.loading = false
      }
    },
  },
  computed: {
    newOutboundTags(): string[] {
      return this.outbounds.map((o:Outbound) => o.tag)
    }
  },
  beforeUnmount() { ++this.requestId; this.closed = true },
  watch: {
    visible: {
      immediate: true,
      handler(v: boolean) {
        this.closed = !v
        this.resetData()
      },
    },
  },
}
</script>
