<template>
  <v-dialog :model-value="visible" @update:model-value="setVisible" transition="dialog-bottom-transition" width="90%" max-width="400">
    <v-card class="rounded-lg" :loading="loading">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('main.stats.title') }}</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto">
            <v-icon icon="mdi-refresh" class="me-2" @click="refresh" v-tooltip:top="$t('actions.update')" />
            <v-icon icon="mdi-close" @click="setVisible(false)" />
          </v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-table density="compact">
          <tbody>
            <tr v-for="row in tableRows" :key="row.key">
              <td class="pa-2" style="width: 40px;">
                <v-icon :icon="row.icon" size="small" :color="row.color || undefined" />
              </td>
              <td class="pa-2">{{ row.label }}</td>
              <td class="pa-2 text-end" style="direction: ltr;">{{ row.value }}</td>
            </tr>
          </tbody>
        </v-table>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { push } from 'notivue'
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import { i18n } from '@/locales'

export default {
  props: {
    visible: { type: Boolean, default: false },
  },
  emits: ['update:visible'],
  setup(props, { emit }) {
    let requestId = 0
    let closed = !props.visible
    const loading = ref(false)
    const info = ref<{
      clients?: number
      inbounds?: number
      outbounds?: number
      services?: number
      endpoints?: number
      clientUp?: number
      clientDown?: number
    }>({})

    const clientUp = computed(() => HumanReadable.sizeFormat(info.value.clientUp ?? 0))
    const clientDown = computed(() => HumanReadable.sizeFormat(info.value.clientDown ?? 0))
    const totalUsage = computed(() => {
      const up = info.value.clientUp ?? 0
      const down = info.value.clientDown ?? 0
      return HumanReadable.sizeFormat(up + down)
    })

    const tableRows = computed(() => {
      const t = (key: string) => i18n.global.t(key)
      return [
        { key: 'clients', icon: 'mdi-account-multiple', label: t('pages.clients'), value: info.value.clients ?? 0, color: undefined },
        { key: 'inbounds', icon: 'mdi-cloud-download', label: t('pages.inbounds'), value: info.value.inbounds ?? 0, color: undefined },
        { key: 'outbounds', icon: 'mdi-cloud-upload', label: t('pages.outbounds'), value: info.value.outbounds ?? 0, color: undefined },
        { key: 'services', icon: 'mdi-server', label: t('pages.services'), value: info.value.services ?? 0, color: undefined },
        { key: 'endpoints', icon: 'mdi-cloud-tags', label: t('pages.endpoints'), value: info.value.endpoints ?? 0, color: undefined },
        { key: 'clientUp', icon: 'mdi-cloud-upload', label: t('stats.upload'), value: clientUp.value, color: 'orange' },
        { key: 'clientDown', icon: 'mdi-cloud-download', label: t('stats.download'), value: clientDown.value, color: 'success' },
        { key: 'totalUsage', icon: 'mdi-chart-box', label: t('main.stats.totalUsage'), value: totalUsage.value, color: 'primary' },
      ]
    })

    const refresh = async () => {
      if (!props.visible || closed) return
      const current = ++requestId
      loading.value = true
      try {
        const data = await HttpUtils.get('api/status', { r: 'db' })
        if (current !== requestId || closed || !props.visible) return
        if (data?.success && data.obj) info.value = data.obj.db ?? data.obj
      } catch {
        if (current === requestId && !closed && props.visible) push.error({ message: i18n.global.t('failed') })
      } finally {
        if (current === requestId) loading.value = false
      }
    }

    const setVisible = (visible: boolean) => {
      if (!visible) { closed = true; ++requestId; loading.value = false; info.value = {} }
      emit('update:visible', visible)
    }

    watch(() => props.visible, (v) => {
      closed = !v
      ++requestId
      loading.value = false
      info.value = {}
      if (v) refresh()
    }, { immediate: true })

    onBeforeUnmount(() => { closed = true; ++requestId })

    return {
      loading,
      info,
      clientUp,
      clientDown,
      totalUsage,
      tableRows,
      refresh,
      setVisible,
    }
  },
}
</script>
