<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg" :loading="loading">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('admin.api.title') }}</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto"><v-icon icon="mdi-close-box" @click="closeModal" /></v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-alert
          v-if="newToken.token.length>0"
          color="success"
          density="compact"
          icon="mdi-alert-circle-outline"
        >
          {{ $t('admin.api.msg') }}
          <v-text-field
            readonly
            variant="outlined"
            bg-color="warning"
            append-inner-icon="mdi-content-copy"
            @click:append-inner="copyToClipboard(newToken.token)"
            v-model="newToken.token"
          ></v-text-field>
        </v-alert>
        <v-table density="compact">
          <thead>
            <tr>
              <th>#</th>
              <th>{{ $t('admin.api.token') }}</th>
              <th>{{ $t('client.desc') }}</th>
              <th>{{ $t('date.expiry') }}</th>
              <th>{{ $t('actions.del') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(token, index) of tokens" :key="token.id">
              <td>{{ token.id }}</td>
              <td>{{ token.token }}</td>
              <td>{{ token.desc }}</td>
              <td>{{ dateFormatted(token.expiry) }}</td>
              <td>
                <v-menu
                  v-model="delOverlay[index]"
                  :close-on-content-click="false"
                  location="top center"
                >
                  <template v-slot:activator="{ props }">
                    <v-icon
                      class="me-2"
                      color="error"
                      v-bind="props"
                    >
                      mdi-delete
                    </v-icon>
                  </template>
                  <v-card :title="$t('actions.del')" rounded="lg">
                    <v-divider></v-divider>
                    <v-card-text>{{ $t('confirm') }}</v-card-text>
                    <v-card-actions>
                      <v-btn color="error" variant="outlined" @click="deleteToken(token.id)">{{ $t('yes') }}</v-btn>
                      <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
                    </v-card-actions>
                  </v-card>
                </v-menu>
              </td>
            </tr>
          </tbody>
        </v-table>
        <v-btn color="primary" @click="showAddToken()">
          {{ $t('actions.add') }}
        </v-btn>
        <v-dialog v-model="showNewToken" width="300">
          <v-card class="rounded-lg">
            <v-card-title>
              <v-row>
                <v-col>{{ $t('admin.api.token') }}</v-col>
                <v-spacer></v-spacer>
                <v-col cols="auto"><v-icon icon="mdi-close-box" @click="showNewToken = false" /></v-col>
              </v-row>
            </v-card-title>
            <v-divider></v-divider>
            <v-card-text>
              <v-row>
                <v-col>
                  <v-text-field :label="$t('client.desc')" v-model="newToken.desc"></v-text-field>
                </v-col>
              </v-row>
              <v-row>
                <v-col>
                  <v-text-field :label="$t('date.expiry')" v-model.number="newToken.expiry" min="0" type="number" :suffix="$t('date.d')"></v-text-field>
                </v-col>
              </v-row>
            </v-card-text>
            <v-card-actions>
              <v-spacer></v-spacer>
              <v-btn
                color="blue-darken-1"
                variant="outlined"
                @click="showNewToken = false"
              >
                {{ $t('actions.close') }}
              </v-btn>
              <v-btn
                color="blue-darken-1"
                variant="tonal"
                @click="addToken"
              >
                {{ $t('actions.add') }}
              </v-btn>
            </v-card-actions>
          </v-card>
        </v-dialog>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="blue-darken-1"
          variant="outlined"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import Clipboard from 'clipboard'
import { push } from 'notivue';

export default {
  props: ['visible', 'user'],
  emits: ['close'],
  data() {
    return {
      requestId: 0,
      loadRequestId: 0,
      fetching: false,
      mutating: false,
      closed: false,
      loading: false,
      tokens: <any[]>[],
      showNewToken: false,
      newToken: {
        desc: '',
        token: '',
        expiry: 0,
      },
      delOverlay: new Array<boolean>(0),
    }
  },
  computed: {
    locale() {
      const l = i18n.global.locale.value
      switch (l) {
        case "zhHans":
          return "zh-cn"
        default:
          return l
      }
    },
  },
  methods: {
    async loadData() {
      if (!this.$props.visible || this.closed) return
      const requestId = this.requestId
      const loadRequestId = ++this.loadRequestId
      this.fetching = true
      this.loading = true
      try {
        const data = await HttpUtils.get('api/tokens')
        if (requestId !== this.requestId || loadRequestId !== this.loadRequestId || this.closed || !this.$props.visible) return
        if (data?.success) {
          this.tokens = Array.isArray(data.obj) ? data.obj.filter((token: any) => token && typeof token === 'object') : []
          this.delOverlay = new Array<boolean>(this.tokens.length).fill(false)
        }
      } catch {
        if (requestId === this.requestId && loadRequestId === this.loadRequestId && !this.closed && this.$props.visible) push.error({ message: i18n.global.t('failed') })
      } finally {
        if (requestId === this.requestId && loadRequestId === this.loadRequestId) {
          this.fetching = false
          this.loading = this.mutating
        }
      }
    },
    resetNewToken() {
      this.newToken={
          desc: '',
          token: '',
          expiry: 30,
        }
    },
    showAddToken() {
      if (!this.$props.visible || this.closed || this.mutating) return
      this.resetNewToken()
      this.showNewToken = true
    },
    async addToken() {
      if (!this.$props.visible || this.closed || this.mutating || !this.showNewToken) return
      const expiry = Number(this.newToken.expiry)
      if (!Number.isInteger(expiry) || expiry < 0) {
        push.error({ message: i18n.global.t('failed') })
        return
      }
      const requestId = this.requestId
      const draft = this.newToken
      this.mutating = true
      this.loading = true
      try {
        const response = await HttpUtils.post('api/addToken', { desc: draft.desc ?? '', expiry })
        if (requestId !== this.requestId || this.closed || !this.$props.visible) return
        if (response?.success) {
          if (draft === this.newToken && typeof response.obj === 'string') {
            draft.token = response.obj
            this.showNewToken = false
          }
          await this.loadData()
        }
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) push.error({ message: i18n.global.t('failed') })
      } finally {
        if (requestId === this.requestId) {
          this.mutating = false
          this.loading = this.fetching
        }
      }
    },
    async deleteToken(id: number) {
      if (!this.$props.visible || this.closed || this.mutating) return
      const requestId = this.requestId
      this.mutating = true
      this.loading = true
      try {
        const response = await HttpUtils.post('api/deleteToken', { id })
        if (requestId === this.requestId && !this.closed && this.$props.visible && response?.success) await this.loadData()
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) push.error({ message: i18n.global.t('failed') })
      } finally {
        if (requestId === this.requestId) {
          this.mutating = false
          this.loading = this.fetching
        }
      }
    },
    dateFormatted(expiry: number) {
      if (expiry == 0) return i18n.global.t('unlimited')
      const date = new Date(expiry*1000)
      return date.toLocaleString(this.locale, {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
      })
    },
    copyToClipboard(txt:string) {
      const hiddenButton = document.createElement('button')
      hiddenButton.className = 'clipboard-btn'
      document.body.appendChild(hiddenButton)

      const clipboard = new Clipboard('.clipboard-btn', {
        text: () => txt,
        container: document.getElementById('qrcode-modal')?? undefined
      });

      clipboard.on('success', () => {
        clipboard.destroy()
        push.success({
          message: i18n.global.t('success') + ": " + i18n.global.t('copyToClipboard'),
          duration: 5000,
        })
      })

      clipboard.on('error', () => {
        clipboard.destroy()
        push.error({
          message: i18n.global.t('failed') + ": " + i18n.global.t('copyToClipboard'),
          duration: 5000,
        })
      })

      // Perform click on hidden button to trigger copy
      hiddenButton.click()
      document.body.removeChild(hiddenButton)
    },
    invalidate() {
      ++this.requestId
      ++this.loadRequestId
      this.fetching = false
      this.mutating = false
      this.loading = false
      this.tokens = []
      this.delOverlay = []
      this.showNewToken = false
      this.resetNewToken()
    },
    closeModal() {
      this.closed = true
      this.invalidate()
      this.$emit('close')
    },
  },
  beforeUnmount() { this.closed = true; this.invalidate() },
  watch: {
    visible: {
      immediate: true,
      handler(v: boolean) {
        this.closed = !v
        this.invalidate()
        if (v) this.loadData()
      },
    },
    user() {
      this.invalidate()
      if (this.$props.visible && !this.closed) this.loadData()
    },
  },
}
</script>