<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.editbulk') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-container style="padding: 0;">
          <v-card :subtitle="$t('actions.action')" class="mb-4">
            <v-card-text>
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-select
                    v-model="actionMode"
                    :items="actionModes"
                    :label="$t('actions.action')"
                    hide-details
                    @update:model-value="onActionChange"
                  ></v-select>
                </v-col>
              </v-row>
              <v-row v-if="actionMode === 'change_limits'">
                <v-col cols="12" sm="6" md="4">
                  <v-text-field
                    v-model.number="editData.addDays"
                    type="number"
                    :label="$t('bulk.addDays')"
                    :suffix="$t('date.d')"
                    hide-details
                  ></v-text-field>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field
                    v-model.number="editData.addVolume"
                    type="number"
                    :label="$t('bulk.addVolume')"
                    :suffix="$t('stats.GB')"
                    hide-details
                  ></v-text-field>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-switch
                    v-model="editData.enable"
                    :label="$t('enable')"
                    color="primary"
                    hide-details
                  ></v-switch>
                </v-col>
              </v-row>
              <v-row v-if="actionMode === 'add_inbounds' || actionMode === 'remove_inbounds'">
                <v-col cols="12" sm="8">
                  <v-select
                    v-model="editData.inboundTags"
                    :items="inboundTags"
                    :label="$t('client.inboundTags')"
                    multiple
                    chips
                    hide-details
                  ></v-select>
                </v-col>
              </v-row>
            </v-card-text>
          </v-card>

          <!-- Section two: Clients (like init users in Inbound modal) -->
          <Users :clients="clients" :data="selectedClients" />
        </v-container>
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
          :disabled="selectedClients.model === 'all' ? !(clients ?? []).length : selectedClients.values.length === 0"
          @click="saveChanges"
        >
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import Users from '@/components/Users.vue'
import { i18n } from '@/locales'
import { push } from 'notivue'
import Data from '@/store/modules/data';
import { Client } from '@/types/clients';

export default {
  props: ['visible', 'clients', 'inboundTags'],
  emits: ['close'],
  components: { Users },
  data() {
    return {
      requestId: 0,
      closed: false,
      loading: false,
      actionMode: 'change_limits',
      actionModes: [
        { title: i18n.global.t('bulk.changeLimits'), value: 'change_limits' },
        { title: i18n.global.t('bulk.addInbounds'), value: 'add_inbounds' },
        { title: i18n.global.t('bulk.removeInbounds'), value: 'remove_inbounds' },
        { title: i18n.global.t('actions.delbulk'), value: 'delete_bulk' },
      ],
      editData: {
        enable: true,
        addDays: 0,
        addVolume: 0,
        inboundTags: [] as number[],
      },
      selectedClients: {
        model: 'none',
        values: [] as any[],
      },
    }
  },
  methods: {
    onActionChange() {
      this.editData.inboundTags = []
    },
    closeModal() {
      ++this.requestId
      this.closed = true
      this.loading = false
      this.$emit('close')
    },
    getTargetClients(): Client[] {
      const clients: Client[] = JSON.parse(JSON.stringify(this.clients ?? []))
      switch (this.selectedClients.model) {
        case 'all':
          return clients
        case 'group':
          return clients
            .filter((c: any) => this.selectedClients.values.includes(c.group))
        case 'client':
          return clients.filter((c: any) => this.selectedClients.values.includes(c.id))
        default:
          return []
      }
    },
    async saveChanges() {
      if (!this.$props.visible || this.closed || this.loading) return
      let targetClients = this.getTargetClients()
      if (!targetClients.length) return
      const actionMode = this.actionMode
      if (!['change_limits', 'add_inbounds', 'remove_inbounds', 'delete_bulk'].includes(actionMode)) return
      const editData = { ...this.editData, inboundTags: [...(this.editData.inboundTags ?? [])] }
      const requestId = ++this.requestId
      this.loading = true
      const tags: number[] = editData.inboundTags ?? []
      try {
        if (actionMode === 'change_limits' && (!Number.isFinite(Number(editData.addVolume)) || !Number.isFinite(Number(editData.addDays)))) throw new Error('Invalid limits')
        // The list API omits reset settings and lifetime counters; never save its summaries.
        if (actionMode !== 'delete_bulk') {
          const fullClients: Client[] = []
          for (const target of targetClients) {
            const client = await Data().loadClients(target.id!)
            if (requestId !== this.requestId || this.closed || !this.$props.visible) return
            if (!client?.name || client.id !== target.id) throw new Error('Client no longer available')
            fullClients.push(JSON.parse(JSON.stringify(client)))
          }
          targetClients = fullClients
        }
        targetClients.forEach(c => { if (!Array.isArray(c.inbounds)) c.inbounds = [] })
        switch (actionMode) {
          case 'change_limits':
            targetClients.forEach((c: Client) => {
              if (editData.addVolume != 0 && c.volume > 0)
                c.volume += Number(editData.addVolume)*(1024 ** 3)
              if (editData.addDays != 0 && c.expiry > 0)
                c.expiry += Number(editData.addDays)*(24 * 60 * 60)
              if (editData.enable)
                c.enable = (c.volume == 0 || c.up + c.down < c.volume) && (c.expiry == 0 || c.expiry > Date.now()/1000)
            })
            break
          case 'add_inbounds':
            targetClients.forEach((c: Client) => {
              tags.forEach((t: number) => {
                if (!c.inbounds.includes(t)) {
                  c.inbounds.push(t)
                }
              })
              c.inbounds = c.inbounds.sort((a,b) => a-b)
            })
            break
          case 'remove_inbounds':
            targetClients.forEach((c: Client) => {
              c.inbounds = c.inbounds.filter((i: number) => !tags.includes(i))
            })
            break
          case 'delete_bulk':
            const deleted = await Data().save("clients", "delbulk", targetClients.map((c: Client) => c.id))
            if (requestId === this.requestId && !this.closed && this.$props.visible && deleted) this.closeModal()
            return
          default:
            return
        }
        if (targetClients.some(c => !Number.isFinite(c.volume) || !Number.isFinite(c.expiry) || c.volume < 0 || c.expiry < 0)) throw new Error('Invalid limits')
        const success = await Data().save("clients", 'editbulk', targetClients)
        if (requestId === this.requestId && !this.closed && this.$props.visible && success) this.closeModal()
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
      handler(newVal: boolean) {
        ++this.requestId
        this.closed = !newVal
        this.loading = false
        if (newVal) {
          this.actionMode = 'change_limits'
          this.editData = { enable: true, addDays: 0, addVolume: 0, inboundTags: [] }
          this.selectedClients = { model: 'none', values: [] }
        }
      },
    },
  },
}
</script>
