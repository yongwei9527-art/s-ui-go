<template>
  <InboundVue
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :inTags="inTags"
    :tlsConfigs="tlsConfigs"
    @close="closeModal"
  />
  <Stats
    v-model="stats.visible"
    :visible="stats.visible"
    :resource="stats.resource"
    :tag="stats.tag"
    @close="closeStats"
  />
  <v-row>
    <v-col cols="12">
      <v-alert
        v-model="defaultProtocolNoticeVisible"
        color="primary"
        icon="mdi-information-outline"
        variant="tonal"
        rounded="lg"
        closable
        class="mb-2"
      >
        <div class="text-subtitle-1 font-weight-bold">{{ $t('inbounds.defaultProtocolNoticeTitle') }}</div>
        <div class="text-body-2">{{ $t('inbounds.defaultProtocolNoticeDesc') }}</div>
      </v-alert>
    </v-col>
    <v-col cols="12" class="d-flex flex-wrap justify-center align-center ga-2">
      <v-btn color="primary" variant="text" @click="showDefaultProtocolNotice">
        <v-icon start>mdi-information-outline</v-icon>
        {{ $t('inbounds.defaultProtocolNoticeTitle') }}
      </v-btn>
      <v-btn color="primary" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>inbounds" :key="item.tag">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.tag">
        <v-card-subtitle style="margin-top: -15px;">
          <v-row>
            <v-col>{{ item.type }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('in.addr') }}</v-col>
            <v-col>
              {{ item.listen }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('in.port') }}</v-col>
            <v-col>
              {{ item.listen_port }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('objects.tls') }}</v-col>
            <v-col>
              {{ tlsEnabled(item.tls_id) ? $t('enable') : $t('disable') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('pages.clients') }}</v-col>
            <v-col>
              <template v-if="item.users">
                <v-tooltip activator="parent" dir="ltr" location="bottom" v-if="item.users.length > 0">
                  <span v-for="u in item.users">{{ u }}<br /></span>
                </v-tooltip>
                {{ item.users.length }}
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('online') }}</v-col>
            <v-col>
              <template v-if="onlines.includes(item.tag)">
                <v-chip density="comfortable" size="small" color="success" variant="flat">{{ $t('online') }}</v-chip>
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-file-edit" @click="showModal(item.id)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove" style="margin-inline-start:0;" color="warning" @click="delOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="delOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="delInbound(item.id)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-content-duplicate" :loading="cloneLoading" @click="clone(item.id)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.clone')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-chart-line" @click="showStats(item.tag)" v-if="Data().enableTraffic">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('stats.graphTitle')"></v-tooltip>
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import InboundVue from '@/layouts/modals/Inbound.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Config } from '@/types/config'
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { createInbound, Inbound } from '@/types/inbounds'
import RandomUtil from '@/plugins/randomUtil'
import { useOneTimeNotice } from '@/composables/useOneTimeNotice'
import { push } from 'notivue'
import { i18n } from '@/locales'

const { visible: defaultProtocolNoticeVisible, show: showDefaultProtocolNotice } = useOneTimeNotice(
  's-ui.inbound-default-protocol-notice.v1'
)

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const inbounds = computed((): Inbound[] => {
  return <Inbound[]> Data().inbounds
})

const tlsConfigs = computed((): any[] => {
  return <any[]> Data().tlsConfigs
})
const tlsEnabled = (id: number) => id > 0 && !!tlsConfigs.value.find(t => t.id === id)?.server?.enabled

const inTags = computed((): string[] => {
  return [...inbounds.value?.map(i => i.tag), ...Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag)]
})

const onlines = computed(() => {
  return Data().onlines?.inbound?? []
})

const modal = ref({
  visible: false,
  id: 0,
})

let delOverlay = ref(new Array<boolean>)
// Index-based confirmations cannot survive a row deletion/reorder.
watch(() => inbounds.value.map(item => [item.id, item.tag]), () => {
  delOverlay.value = []
}, { flush: 'sync' })

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.visible = true
}
const closeModal = () => {
  modal.value.visible = false
}

const delInbound = async (id: number) => {
  const item = inbounds.value.find(item => item.id === id)
  if (!item) return
  const success = await Data().save('inbounds', 'del', item.tag)
  if (success) delOverlay.value = []
}

let cloneLoading = ref(false)

let active = true
onBeforeUnmount(() => { active = false; cloneLoading.value = false })
const clone = async (id: number) => {
  if (!active || cloneLoading.value) return
  cloneLoading.value = true
  try {
    const inboundArray = await Data().loadInbounds([id])
    const inbound = inboundArray?.[0]
    if (!active || !inbound?.type) return
    const newInbound = createInbound(inbound.type, {
      ...inbound, id: 0, tag: inbound.type + '-' + RandomUtil.randomSeq(3),
      listen_port: RandomUtil.randomIntRange(10000, 60000),
    })
    await Data().save('inbounds', 'new', newInbound)
  } catch {
    if (active) push.error({ title: i18n.global.t('failed') })
  } finally {
    cloneLoading.value = false
  }
}

const stats = ref({
  visible: false,
  resource: "inbound",
  tag: "",
})

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}
</script>