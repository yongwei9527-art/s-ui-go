<template>
  <v-dialog transition="dialog-bottom-transition" width="90%" max-width="500">
    <v-card class="rounded-lg" :loading="restoring">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('main.backup.title') }}</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto">
            <v-icon icon="mdi-close" @click="control.visible = false" />
          </v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclStats')" value="stats" hide-details></v-checkbox>
          </v-col>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclChanges')" value="changes" hide-details></v-checkbox>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" :disabled="restoring" @click="backup()" hide-details>{{ $t('main.backup.backup') }}</v-btn>
          </v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" :loading="restoring" @click="restore()" hide-details>{{ $t('main.backup.restore') }}</v-btn>
          </v-col>
        </v-row>
        <v-row>
          <v-divider></v-divider>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" :disabled="restoring" @click="config()" hide-details>{{ $t('main.backup.sbConfig') }}</v-btn>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import HttpUtils from '@/plugins/httputil'
import { push } from 'notivue'
export default {
  props: ['control', 'visible'],
  data() {
    return {
      exclude: ["stats", "changes"],
      restoring: false,
    }
  },
  methods: {
    backup() {
      const excludeOption = this.exclude.length>0 ? '?exclude=' +this.exclude.join(',') : ''
      window.location.href = 'api/getdb' + excludeOption
    },
    config() {
      window.location.href = 'api/singbox-config'
    },
    restore() {
      if (this.restoring) return
      const fileInput = document.createElement('input')
      fileInput.type = 'file'
      fileInput.accept = '.db'

      fileInput.addEventListener('change', async (event: Event) => {
        const inputElement = event.target as HTMLInputElement
        const dbFile = inputElement.files ? inputElement.files[0] : null

        if (dbFile) await this.uploadDatabase(dbFile)
      })

      fileInput.click()
    },
    async waitForRecovery() {
      const deadline = Date.now() + 45_000
      while (Date.now() < deadline) {
        const controller = new AbortController()
        const timeout = setTimeout(() => controller.abort(), 2000)
        try {
          const response = await fetch('api/recovery-ready', { cache: 'no-store', signal: controller.signal })
          if (response.status === 204) return true
          if (response.status === 404) return false // Backup changed the panel path/port.
        } catch {
          // Restart temporarily closes the listener; retry the readiness probe,
          // never upload/import the database again.
        } finally {
          clearTimeout(timeout)
        }
        await new Promise(resolve => setTimeout(resolve, 1000))
      }
      return false
    },
    async uploadDatabase(dbFile: File) {
      if (this.restoring) return
      this.restoring = true
      try {
        const formData = new FormData()
        formData.append('db', dbFile)
        const result = await HttpUtils.post('api/importdb', formData, {
          headers: { 'Content-Type': 'multipart/form-data' },
        })
        if (!result.success) return
        this.control.visible = false
        localStorage.removeItem('s-ui-authenticated')
        if (await this.waitForRecovery()) {
          location.reload()
        } else {
          push.error({ message: '备份已导入，但尚未确认服务恢复。请检查服务状态，并使用备份中的面板端口和路径重新访问；不要重复导入。' })
        }
      } catch {
        push.error({ message: '恢复请求未能确认成功。请先检查服务日志和当前数据；不要直接重复导入。' })
      } finally {
        this.restoring = false
      }
    }
  },
  watch: {
    visible(v) {
      if (v) {
        this.exclude = ["stats", "changes"]
      }
    },
  },
}
</script>
