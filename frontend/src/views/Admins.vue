<template>
  <AdminModal
    v-model="editModal.visible"
    :visible="editModal.visible"
    :user="editModal.user"
    @close="closeEditModal"
    @save="saveEditModal"
  />
  <ChangeModal
    v-model="changesModal.visible"
    :visible="changesModal.visible"
    :admins="users.map((u:any) => u.username)"
    :actor="changesModal.actor"
    @close="closeChangesModal"
  />
  <TokenModal
    v-model="tokenModal.visible"
    :visible="tokenModal.visible"
    @close="closeTokenModal"
  />
  <v-row>
    <v-col cols="12" justify="center" align="center">
      <v-btn color="primary" @click="showChangesModal('')" style="margin: 0 5px;">{{ $t('admin.changes') }}</v-btn>
      <v-btn color="primary" @click="showTokenModal()">{{ $t('admin.api.token') }}</v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>users" :key="item.id">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.username">
        <v-card-subtitle style="margin-top: -15px;">
          {{ $t('admin.lastLogin') }}
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('admin.date') }}</v-col>
            <v-col>
              {{ item.loginDate }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('admin.time') }}</v-col>
            <v-col>
              {{ item.loginTime }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>IP</v-col>
            <v-col>
              {{ item.ip }}
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-account-edit" @click="showEditModal(item)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-list-box-outline" @click="showChangesModal(item.username)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('admin.changes')"></v-tooltip>
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import AdminModal from '@/layouts/modals/Admin.vue'
import ChangeModal  from '@/layouts/modals/Changes.vue'
import TokenModal from '@/layouts/modals/Token.vue'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { push } from 'notivue'
import { Ref, ref, inject, onMounted, onBeforeUnmount } from 'vue'

const loading:Ref = inject('loading')?? ref(false)

const users = ref(<any[]>[])

let active = true
let loadRequest = 0
let pendingRequests = 0
let saving = false
onBeforeUnmount(() => { active = false; loadRequest++; loading.value = false })
const beginRequest = () => { pendingRequests++; loading.value = true }
const endRequest = () => { pendingRequests--; if (active) loading.value = pendingRequests > 0 }
onMounted(() => { void loadData() })

const loadData = async () => {
  if (!active) return
  const request = ++loadRequest
  beginRequest()
  try {
    const msg = await HttpUtils.get('api/users')
    if (active && request === loadRequest && msg.success && Array.isArray(msg.obj)) {
      const locale = i18n.global.locale.value.replace('zh', 'zh-')
      users.value = msg.obj.map((u: any) => {
        const lastLogin = (typeof u.lastLogin === 'string' ? u.lastLogin : '').split(' ')
        const date = new Date(lastLogin[0] + ' ' + lastLogin[1])
        const valid = lastLogin.length > 2 && Number.isFinite(date.getTime())
        return {
          id: u.id, username: u.username,
          loginDate: valid ? date.toLocaleDateString(locale) : '-',
          loginTime: valid ? date.toLocaleTimeString(locale) : '-',
          ip: lastLogin[2] || '-',
        }
      })
    }
  } catch {
    if (active && request === loadRequest) push.error({ title: i18n.global.t('failed') })
  } finally {
    endRequest()
  }
}

const editModal = ref({
  visible: false,
  user: {},
})

let editSession = 0
const showEditModal = (user: any) => {
  editSession++
  editModal.value.user = user
  editModal.value.visible = true
}
const closeEditModal = () => {
  editSession++
  editModal.value.visible = false
  editModal.value.user = {}
}
const saveEditModal = async (data:any) => {
  if (!active || saving) return
  saving = true
  const session = editSession
  const submitted = JSON.parse(JSON.stringify(data))
  beginRequest()
  try {
    const response = await HttpUtils.post('api/changePass', submitted)
    if (response.success && active) {
      const user = users.value.find(user => user.id === submitted.id)
      if (user) user.username = submitted.newUsername
      if (session === editSession) closeEditModal()
    }
  } catch {
    if (active) push.error({ title: i18n.global.t('failed') })
  } finally {
    saving = false
    endRequest()
  }
}

const changesModal = ref({
  visible: false,
  actor: '',
})
const showChangesModal = (actor: string) => {
  changesModal.value.actor = actor
  changesModal.value.visible = true
}
const closeChangesModal = () => {
  changesModal.value.visible = false
  changesModal.value.actor = ''
}

const tokenModal = ref({
  visible: false,
})
const showTokenModal = () => {
  tokenModal.value.visible = true
}
const closeTokenModal = () => {
  tokenModal.value.visible = false
}
</script>