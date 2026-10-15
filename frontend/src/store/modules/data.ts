import HttpUtils from '@/plugins/httputil'
import { defineStore } from 'pinia'
import { push } from 'notivue'
import { i18n } from '@/locales'
import { Inbound } from '@/types/inbounds'
import { Client } from '@/types/clients'


// Keep synchronization metadata per Pinia instance and out of persisted/public state.
// Reads are ordered per returned field, not globally: independent partials can merge.
const partialObjects = ['config', 'clients', 'inbounds', 'outbounds', 'endpoints', 'services', 'tls'] as const
type PartialObject = typeof partialObjects[number]
const cacheFields = ['subURI', 'enableTraffic', ...partialObjects] as const
type CacheField = typeof cacheFields[number]
type Resource = CacheField | 'onlines'
const resources: readonly Resource[] = [...cacheFields, 'onlines']
interface ResourceState {
  epoch: number
  pendingWrites: number
  latestWrite: number
  appliedRequest: number
  version: number
  revision?: string
}
interface SyncState {
  sequence: number
  writeEpoch: number
  pendingWrites: number
  controlRequest: number
  needsFullLoad: boolean
  fields: Record<Resource, ResourceState>
  details: Map<string, number>
}
const synchronizers = new WeakMap<object, SyncState>()
function synchronization(store: object): SyncState {
  let sync = synchronizers.get(store)
  if (!sync) {
    sync = {
      sequence: 0, writeEpoch: 0, pendingWrites: 0, controlRequest: 0, needsFullLoad: false,
      fields: Object.fromEntries(resources.map(key => [key, {
        epoch: 0, pendingWrites: 0, latestWrite: 0, appliedRequest: 0, version: 0,
      }])) as Record<Resource, ResourceState>,
      details: new Map(),
    }
    synchronizers.set(store, sync)
  }
  return sync
}
function normalizeRevision(value: unknown): string | undefined {
  const text = typeof value === 'string' ? value
    : typeof value === 'number' && Number.isSafeInteger(value) ? String(value) : ''
  return /^(0|[1-9]\d*)$/.test(text) ? text : undefined
}
// Compare decimal strings losslessly, including revisions beyond Number.MAX_SAFE_INTEGER.
function compareRevision(a: string, b: string): number {
  return a.length === b.length ? (a === b ? 0 : a > b ? 1 : -1) : a.length > b.length ? 1 : -1
}
const record = (value: any): value is Record<string, any> => value !== null && typeof value === 'object' && !Array.isArray(value)
function validField(key: Resource, value: any): boolean {
  if (key === 'config') return record(value)
  if (key === 'onlines') return value == null || record(value)
  if (key === 'subURI') return value == null || typeof value === 'string'
  if (key === 'enableTraffic') return value == null || typeof value === 'boolean'
  return value == null || Array.isArray(value)
}
function applyField(store: any, key: Resource, value: any) {
  if (key === 'config') store.config = value
  else if (key === 'onlines') store.onlines = value ?? { inbound: [], outbound: [], user: [] }
  else if (key === 'subURI') store.subURI = value ?? ''
  else if (key === 'enableTraffic') store.enableTraffic = value ?? false
  else store[key === 'tls' ? 'tlsConfigs' : key] = value ?? []
}
function startRead(sync: SyncState) {
  return {
    id: ++sync.sequence, writeEpoch: sync.writeEpoch, pendingWrites: sync.pendingWrites,
    fields: Object.fromEntries(resources.map(key => [key, { ...sync.fields[key] }])) as Record<Resource, ResourceState>,
  }
}
type ReadTicket = ReturnType<typeof startRead>
function currentRead(sync: SyncState, ticket: ReadTicket, key: Resource, revision?: string): boolean {
  const current = sync.fields[key], start = ticket.fields[key]
  if (start.epoch !== current.epoch || start.pendingWrites > 0 || current.pendingWrites > 0) return false
  if (revision && current.revision) {
    const order = compareRevision(revision, current.revision)
    if (order !== 0) return order > 0
  }
  return ticket.id >= current.appliedRequest
}
function appliedRead(sync: SyncState, ticket: ReadTicket, key: Resource, revision?: string) {
  const field = sync.fields[key]
  field.appliedRequest = Math.max(field.appliedRequest, ticket.id)
  field.version++
  if (revision && (!field.revision || compareRevision(revision, field.revision) > 0)) field.revision = revision
}
// Mirrors the related objects returned by ConfigService.Save. A write may affect
// more than its primary object (e.g. TLS changes also update clients/inbounds).
const writeFields: Record<string, readonly CacheField[]> = {
  config: ['config'], clients: ['clients', 'inbounds'], tls: ['tls', 'clients', 'inbounds'],
  inbounds: ['inbounds', 'clients'], outbounds: ['outbounds'], endpoints: ['endpoints'],
  services: ['services'], settings: ['subURI', 'enableTraffic'],
}
function startWrite(sync: SyncState, object: string) {
  const ticket = { id: ++sync.sequence, fields: writeFields[object] ?? cacheFields }
  sync.writeEpoch++; sync.pendingWrites++; sync.needsFullLoad = true
  for (const key of ticket.fields) {
    const field = sync.fields[key]
    field.epoch++; field.pendingWrites++; field.latestWrite = ticket.id
  }
  return ticket
}
function finishWrite(sync: SyncState, ticket: ReturnType<typeof startWrite>) {
  // Invalidate reads begun DURING the write too, even if they finish after its ACK.
  sync.writeEpoch++; sync.pendingWrites--
  for (const key of ticket.fields) { sync.fields[key].epoch++; sync.fields[key].pendingWrites-- }
}
function applyWrite(store: object, sync: SyncState, id: number, data: any) {
  if (!record(data)) return
  for (const key of cacheFields) {
    if (!Object.hasOwn(data, key) || !validField(key, data[key])) continue
    const field = sync.fields[key]
    if (field.latestWrite > id || field.appliedRequest > id) continue
    applyField(store, key, data[key])
    field.epoch++; field.version++; field.latestWrite = id; field.appliedRequest = id
  }
}
function startDetail(sync: SyncState, key: PartialObject, query: string) {
  const ticket = { ...startRead(sync), query: key + ':' + query }
  sync.details.set(ticket.query, ticket.id)
  return ticket
}
function currentDetail(sync: SyncState, ticket: ReturnType<typeof startDetail>, key: PartialObject) {
  return sync.details.get(ticket.query) === ticket.id && currentRead(sync, ticket, key)
    && sync.fields[key].version === ticket.fields[key].version
}
function finishDetail(sync: SyncState, ticket: ReturnType<typeof startDetail>) {
  if (sync.details.get(ticket.query) === ticket.id) sync.details.delete(ticket.query)
}

const Data = defineStore('Data', {
  state: () => ({
    // Revisions are opaque decimal strings so uint64 values retain their exact value in JavaScript.
    lastLoad: <string>"",
    reloadItems: localStorage.getItem("reloadItems")?.split(',')?? <string[]>[],
    subURI: "",
    enableTraffic: false,
    onlines: {inbound: <string[]>[], outbound: <string[]>[], user: <string[]>[]},
    config: <any>{},
    inbounds: <any[]>[],
    outbounds: <any[]>[],
    services: <any[]>[],
    endpoints: <any[]>[],
    clients: <any>[],
    tlsConfigs: <any[]>[],
  }),
  actions: {
    async loadData(object?: PartialObject) {
      const sync = synchronization(this), ticket = startRead(sync)
      const options = !object && this.lastLoad && !sync.needsFullLoad ? { lu: this.lastLoad } : {}
      const msg = await HttpUtils.get(object ? 'api/' + object : 'api/load', options)
      if (!msg.success || !record(msg.obj)) return
      const data = msg.obj, revision = normalizeRevision(data.lastUpdate)
      const knownRevision = normalizeRevision(this.lastLoad)
      if (revision && knownRevision && compareRevision(revision, knownRevision) < 0) return
      const stableWrite = ticket.writeEpoch === sync.writeEpoch && ticket.pendingWrites === 0 && sync.pendingWrites === 0
      let rejectedField = false
      for (const key of cacheFields) {
        if (!Object.hasOwn(data, key)) continue
        if (!validField(key, data[key]) || !currentRead(sync, ticket, key, revision)) {
          rejectedField = true
          continue
        }
        applyField(this, key, data[key]); appliedRead(sync, ticket, key, revision)
      }
      if (Object.hasOwn(data, 'onlines') && stableWrite && validField('onlines', data.onlines) && currentRead(sync, ticket, 'onlines', revision)) {
        applyField(this, 'onlines', data.onlines); appliedRead(sync, ticket, 'onlines', revision)
      }
      // A partition-only response cannot certify a global polling cursor. Nor can
      // a poll that skipped protected fields: the next poll must request a full snapshot.
      if (!object && rejectedField) sync.needsFullLoad = true
      const newerRevision = revision && (!knownRevision || compareRevision(revision, knownRevision) > 0)
      if (!object && stableWrite && !rejectedField && (ticket.id >= sync.controlRequest || newerRevision)) {
        if (!sync.needsFullLoad || record(data.config)) {
          this.setLastLoadRevision(data.lastUpdate)
          if (revision && record(data.config)) sync.needsFullLoad = false
        }
        sync.controlRequest = Math.max(sync.controlRequest, ticket.id)
        if (data.lastLog) push.error({ title: i18n.global.t('error.core'), duration: 5000, message: data.lastLog })
      }
    },
    setLastLoadRevision(revision: unknown) {
      const next = normalizeRevision(revision), current = normalizeRevision(this.lastLoad)
      if (next && (!current || compareRevision(next, current) >= 0)) this.lastLoad = next
    },
    setNewData(data: any) {
      if (!record(data)) return
      const sync = synchronization(this)
      // Public authoritative updates obey the same barriers as save ACKs, but do
      // not invent a revision. Internal read application does not enter this path.
      sync.writeEpoch++; sync.needsFullLoad = true
      applyWrite(this, sync, ++sync.sequence, data)
    },
    async loadInbounds(ids: number[]): Promise<Inbound[]> {
      const sync = synchronization(this)
      const ticket = startDetail(sync, 'inbounds', [...ids].sort((a, b) => a - b).join(','))
      try {
        const options = ids.length > 0 ? { id: ids.join(',') } : {}
        const msg = await HttpUtils.get('api/inbounds', options)
        return msg.success && Array.isArray(msg.obj?.inbounds) && currentDetail(sync, ticket, 'inbounds') ? msg.obj.inbounds : []
      } finally { finishDetail(sync, ticket) }
    },
    async loadClients(id: number): Promise<Client> {
      const sync = synchronization(this), ticket = startDetail(sync, 'clients', id > 0 ? String(id) : '')
      try {
        const options = id > 0 ? { id } : {}
        const msg = await HttpUtils.get('api/clients', options)
        const client = Array.isArray(msg.obj?.clients) ? msg.obj.clients[0] : undefined
        return msg.success && record(client) && currentDetail(sync, ticket, 'clients') ? client as Client : {} as Client
      } finally { finishDetail(sync, ticket) }
    },
    async save (object: string, action: string, data: any, initUsers?: number[]): Promise<boolean> {
      const postData = { object, action, data: JSON.stringify(data, null, 2), initUsers: initUsers?.join(',') ?? undefined }
      const sync = synchronization(this), ticket = startWrite(sync, object)
      try {
        const msg = await HttpUtils.post('api/save', postData)
        if (msg.success) {
          const objectName = ['tls', 'config'].includes(object) ? object : object.substring(0, object.length - 1)
          push.success({
            title: i18n.global.t('success'), duration: 5000,
            message: i18n.global.t('actions.' + action) + ' ' + i18n.global.t('objects.' + objectName),
          })
          applyWrite(this, sync, ticket.id, msg.obj)
        }
        return msg.success
      } finally { finishWrite(sync, ticket) }
    },
    // Check duplicate client name
    checkClientName (id: number, newName: string): boolean {
      const oldName = id > 0 ? this.clients.findLast((i: any) => i.id == id)?.name : null
      if (newName != oldName && this.clients.findIndex((c: any) => c.name == newName) != -1) {
        push.error({
          message: i18n.global.t('error.dplData') + ": " + i18n.global.t('client.name')
        })
        return true
      }
      return false
    },
    // Check bulk client names
    checkBulkClientNames (names: string[]): boolean {
      const newNames = new Set(names)
      const oldNames = new Set(this.clients.map((c: any) => c.name))
      const allNames = new Set([...oldNames, ...newNames])
      if (newNames.size != names.length || oldNames.size + newNames.size != allNames.size) {
        push.error({
          message: i18n.global.t('error.dplData') + ": " + i18n.global.t('client.name')
        })
        return true
      }
      return false
    },
    // check duplicate tag
    checkTag (object: string, id: number, tag: string): boolean {
      let objects = <any[]>[]
      switch (object) {
        case 'inbound':
          objects = this.inbounds
          break
        case 'outbound':
          objects = this.outbounds
          break
        case 'service':
          objects = this.services
          break
        case 'endpoint':
          objects = this.endpoints
          break
        default:
          return false
      }
      const oldObject = id > 0 ? objects.findLast((i: any) => i.id == id) : null
      if (tag != oldObject?.tag && objects.findIndex((i: any) => i.tag == tag) != -1) {
        push.error({
          message: i18n.global.t('error.dplData') + ": " + i18n.global.t('objects.tag')
        })
        return true
      }
      return false
    },
  }
})

export default Data