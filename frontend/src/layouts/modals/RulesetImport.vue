<template>
  <v-dialog transition="dialog-top-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('rule.import.title') }}</v-col>
          <v-col cols="auto" v-if="importPreview.length > 0">
            <v-chip size="small" color="primary" variant="tonal">
              {{ $t('count') }}: {{ importPreview.length }}
            </v-chip>
          </v-col>
        </v-row>
      </v-card-title>
      <v-divider />
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-tabs v-model="tab" @update:modelValue="tabChanged">
          <v-tab value="text">
            {{ $t('rule.import.pasteUrls') }}
          </v-tab>
          <v-tab value="file">
            {{ $t('rule.import.uploadTxt') }}
          </v-tab>
        </v-tabs>
        <v-window v-model="tab" class="mb-4">
          <v-window-item value="text">
            <v-alert variant="text" type="info">{{ $t('rule.import.urlsHint') }}</v-alert>
            <v-textarea
              v-model="importRawText"
              label="URLs"
              variant="outlined"
              rows="10"
              auto-grow
              hide-details
              spellcheck="false"
              placeholder="https://github.com/.../geoip-telegram.srs&#10;https://github.com/.../geosite-youtube.srs"
            ></v-textarea>
          </v-window-item>
          <v-window-item value="file">
            <v-alert variant="text" type="info">{{ $t('rule.import.fileHint') }}</v-alert>
            <v-file-input
              :label="$t('rule.import.selectTxt')"
              accept=".txt"
              variant="outlined"
              hide-details
              prepend-icon="mdi-file-document"
              @update:modelValue="onFileUpload"
              @click:clear="tabChanged"
            ></v-file-input>
          </v-window-item>
        </v-window>
        <v-row class="mb-4">
          <v-col cols="12" sm="6" md="4">
            <v-select
              hide-details
              :label="$t('ruleset.format')"
              :items="['source', 'binary']"
              v-model="importFormat">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-select
              hide-details
              :label="$t('objects.outbound')"
              :items="outTags"
              clearable
              @click:clear="importDetour=''"
              v-model="importDetour">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model.number="importInterval" :suffix="$t('date.d')" type="number" min="0" :label="$t('ruleset.interval')" hide-details></v-text-field>
          </v-col>
        </v-row>

        <template v-if="importPreview.length > 0">
          <v-divider class="my-4" />
          <span class="v-card-subtitle">
            {{ $t('rule.import.preview') }}
            <v-badge v-if="importPreview.length > 0" color="success" :content="importPreview.length" inline />
            <v-badge v-if="importSkipped > 0" color="warning" :content="importSkipped" inline v-tooltip:top="$t('rule.import.skipped')" />
          </span>
          <v-table density="compact" striped="even" class="mb-4">
            <thead>
              <tr><th>{{ $t('objects.tag') }}</th><th>{{ $t('ruleset.format') }}</th><th>URL</th><th>{{ $t('actions.del') }}</th></tr>
            </thead>
            <tbody>
              <tr v-for="(item, i) in importPreview" :key="i" :style="item.exists ? 'opacity:0.4' : ''">
                <td>
                  {{ item.tag }}
                </td>
                <td>{{ item.format }}</td>
                <td v-tooltip:top="item.url" dir="ltr">.../{{ item.url.split('/').pop() ?? item.url }}</td>
                <td><v-icon icon="mdi-delete" color="error" @click="importPreview.splice(i, 1)" /></td>
              </tr>
            </tbody>
          </v-table>
        </template>
      </v-card-text>
      <v-divider />
      <v-card-actions class="pa-3">
        <v-btn @click="parseImport" variant="tonal" :disabled="(importRawText ?? '').trim().length === 0">
          <v-icon icon="mdi-magnify" class="mr-1" />{{ $t('rule.import.parse') }}
        </v-btn>
        <v-spacer />
        <v-btn @click="close" variant="text">{{ $t('actions.close') }}</v-btn>
        <v-btn @click="save" color="primary" variant="flat" :disabled="newCount === 0">
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { push } from 'notivue'
interface ImportItem { tag: string; url: string; format: string; exists: boolean }

export default {
  props: ["visible", "outTags", "rsTags"],
  emits: ['save', 'close'],
  data() {
    return {
      requestId: 0,
      closed: false,
      tab: 'text',
      importRawText: '',
      importFormat: 'binary',
      importDetour: '',
      importInterval: 1,
      importPreview: [] as ImportItem[],
    }
  },
  computed: {
    importSkipped(): number {
      return this.importPreview.filter(i => i.exists).length
    },
    newCount(): number {
      return this.importPreview.filter(i => !i.exists).length
    },
  },
  methods: {
    tabChanged() {
      ++this.requestId
      this.importPreview = []
      this.importRawText = ''
    },
    urlToTag(url: string): string {
      try {
        const filename = new URL(url).pathname.split('/').pop() ?? ''
        return filename.replace(/\.[^.]+$/, '')
      } catch {
        const parts = url.split('/')
        return parts[parts.length - 1].replace(/\.[^.]+$/, '') || url
      }
    },
    close() {
      this.closed = true
      this.tabChanged()
      this.$emit('close')
    },
    parseImport() {
      if (!this.$props.visible || this.closed) return
      ++this.requestId
      const existingTags = new Set(this.rsTags ?? [])
      const seenUrls = new Set<string>()
      const seenTags = new Set<string>()
      this.importPreview = []
      for (const line of (this.importRawText ?? '').split('\n')) {
        const url = line.trim()
        try {
          const parsed = new URL(url)
          if (!['http:', 'https:'].includes(parsed.protocol) || seenUrls.has(url)) continue
          const tag = this.urlToTag(url)
          if (!tag) continue
          seenUrls.add(url)
          const exists = existingTags.has(tag) || seenTags.has(tag)
          this.importPreview.push({ tag, url, format: this.importFormat, exists })
          seenTags.add(tag)
        } catch { /* Ignore lines that are not HTTP(S) URLs. */ }
      }
    },
    save() {
      if (!this.$props.visible || this.closed) return
      const seenTags = new Set<string>(this.rsTags ?? [])
      const toAdd = this.importPreview.filter(item => {
        if (item.exists || seenTags.has(item.tag)) return false
        seenTags.add(item.tag)
        return true
      }).map(item => {
        const rs: any = { type: 'remote', tag: item.tag, format: this.importFormat, url: item.url }
        if (this.importDetour) rs.download_detour = this.importDetour
        if (Number.isFinite(Number(this.importInterval)) && Number(this.importInterval) > 0) rs.update_interval = Number(this.importInterval) + 'd'
        return rs
      })
      if (toAdd.length) this.$emit('save', toAdd)
    },
    async onFileUpload(files: File | File[] | null) {
      if (!this.$props.visible || this.closed) return
      ++this.requestId
      this.importPreview = []
      this.importRawText = ''
      // The synchronous text watcher also invalidates the previous source.
      const requestId = this.requestId
      const file = Array.isArray(files) ? files[0] : files
      if (!file) return
      try {
        const text = await file.text()
        if (requestId !== this.requestId || this.closed || !this.$props.visible) return
        this.importRawText = text
        this.parseImport()
      } catch {
        if (requestId === this.requestId && !this.closed && this.$props.visible) push.error({ message: this.$t('failed') })
      }
    },
  },
  beforeUnmount() { ++this.requestId; this.closed = true },
  watch: {
    visible: {
      immediate: true,
      handler(v: boolean) {
        this.closed = !v
        this.tabChanged()
        if (v) {
          this.tab = 'text'
          this.importFormat = 'binary'
          this.importDetour = ''
          this.importInterval = 1
        }
      },
    },
    importRawText: {
      flush: 'sync' as const,
      handler() { ++this.requestId; this.importPreview = [] },
    },
    importFormat() { this.importPreview.forEach(item => { item.format = this.importFormat }) },
  },
}
</script>
