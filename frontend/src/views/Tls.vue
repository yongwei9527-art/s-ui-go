<template>
    <TlsVue
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :data="modal.data"
    @close="closeModal"
    @save="saveModal"
  />
  <v-row>
    <v-col cols="12" justify="center" align="center">
      <v-btn color="primary" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>tlsConfigs" :key="item.id">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.name">
        <v-card-subtitle style="margin-top: -15px;">
          {{ item.server?.server_name?.length>0 ? item.server.server_name : "-" }}
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('pages.inbounds') }}</v-col>
            <v-col>
              <template v-if="tlsInbounds(item.id).length>0">
                <v-tooltip activator="parent" dir="ltr" location="bottom">
                  <span v-for="i in tlsInbounds(item.id)">{{ i }}<br /></span>
                </v-tooltip>
                {{ tlsInbounds(item.id).length }}
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
          <v-row>
            <v-col>ACME</v-col>
            <v-col>
              {{ $t(item.server?.enabled && item.server?.acme != null ? 'yes' : 'no') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>ECH</v-col>
            <v-col>
              {{ $t(item.server?.enabled && item.server?.ech?.enabled ? 'yes' : 'no') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>Reality</v-col>
            <v-col>
              {{ $t(item.server?.enabled && item.server?.reality?.enabled ? 'yes' : 'no') }}
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-file-edit" @click="showModal(item.id)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn v-if="!tlsInUse(item.id)" icon="mdi-file-remove" style="margin-inline-start:0;" color="warning" @click="delOverlay[index] = true">
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
                <v-btn color="error" variant="outlined" @click="delTls(item.id)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-content-duplicate" @click="clone(item)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.clone')"></v-tooltip>
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import TlsVue from '@/layouts/modals/Tls.vue'
import Data from '@/store/modules/data'
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { i18n } from '@/locales'
import { push } from 'notivue'
import { Inbound } from '@/types/inbounds'
import { tls } from '@/types/tls'

const tlsConfigs = computed((): any[] => {
  return Data().tlsConfigs
})

const inbounds = computed((): Inbound[] => {
  return Data().inbounds
})

const tlsInbounds = (id: number): string[] => {
  return inbounds.value.filter(i => i.tls_id == id).map(i => i.tag)
}

const tlsInUse = (id: number) => tlsInbounds(id).length > 0 || Data().services.some(s => s.tls_id === id)
let active = true
let modalSession = 0
let saving = false
onBeforeUnmount(() => { active = false; modalSession++ })

const modal = ref({
  visible: false,
  id: 0,
  data: "",
})

const delOverlay = ref(new Array<boolean>(tlsConfigs.value.length).fill(false))
watch(() => tlsConfigs.value.map(t => t.id), () => { delOverlay.value = [] }, { flush: 'sync' })

const showModal = (id: number) => {
  const item = tlsConfigs.value.findLast(t => t.id === id)
  if (id !== 0 && !item) return
  modalSession++
  modal.value.id = id
  modal.value.data = id === 0 ? '{}' : JSON.stringify(item)
  modal.value.visible = true
}
const clone = (obj: any) => {
  let data = JSON.parse(JSON.stringify(obj))
  data.id = 0
  while (tlsConfigs.value.findIndex(t => t.name == data.name) != -1){
    data.name += "-copy"
  }
  void saveModal(data, false)
}
const closeModal = () => {
  modalSession++
  modal.value.visible = false
}
const saveModal = async (data:tls, closeEditor = true) => {
  if (!active || saving) return
  saving = true
  const session = modalSession
  try {
    const success = await Data().save('tls', data.id > 0 ? 'edit' : 'new', JSON.parse(JSON.stringify(data)))
    if (success && active && closeEditor && session === modalSession) closeModal()
  } catch {
    if (active) push.error({ title: i18n.global.t('failed') })
  } finally {
    saving = false
  }
}

const delTls = async (id: number) => {
  if (!tlsConfigs.value.some(t => t.id === id) || tlsInUse(id)) return
  const success = await Data().save('tls', 'del', id)
  if (success) delOverlay.value = []
}

</script>
