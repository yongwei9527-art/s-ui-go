import * as Vue from 'vue'
import { createApp, reactive, shallowRef, nextTick, h, type App } from 'vue'
import { readFileSync } from 'node:fs'
import { parse, compileTemplate } from '@vue/compiler-sfc'
import ts from 'typescript'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// All external actions are mocked before importing the real modal modules.
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), save: vi.fn(), loadClients: vi.fn(), checkName: vi.fn(), checkBulk: vi.fn(), checkTag: vi.fn(), error: vi.fn(), fetch: vi.fn() }))
vi.mock('@/plugins/httputil', () => ({ default: { get: mocks.get, post: mocks.post } }))
vi.mock('@/store/modules/data', () => ({ default: () => ({ save: mocks.save, loadClients: mocks.loadClients, checkClientName: mocks.checkName, checkBulkClientNames: mocks.checkBulk, checkTag: mocks.checkTag }) }))
vi.mock('@/locales', () => ({ locale: 'en', i18n: { global: { t: (key: string) => key, locale: { value: 'en' } } } }))
vi.mock('notivue', () => ({ push: { error: mocks.error, success: vi.fn() } }))
vi.mock('clipboard', () => ({ default: class { on() { return this }; destroy() {} } }))
vi.mock('@/plugins/utils', () => ({ HumanReadable: { sizeFormat: (value: number) => String(value) } }))
vi.mock('@/plugins/randomUtil', () => ({ default: { randomSeq: () => 'mock-random', randomIntRange: () => 12345, randomShortId: () => ['abcd'], randomUUID: () => 'mock-uuid', randomShadowsocksPassword: () => 'mock-password' } }))
vi.mock('@/components/DateTime.vue', () => ({ default: {} }))
vi.mock('@/components/Users.vue', () => ({ default: { render: () => null } }))
vi.mock('@/components/Dial.vue', () => ({ default: {} }))
vi.mock('@/components/Headers.vue', () => ({ default: {} }))
vi.mock('@/components/tls/OutTLS.vue', () => ({ default: {} }))
vi.mock('@/components/tls/InTLS.vue', () => ({ default: {} }))
vi.mock('@/components/tls/Acme.vue', () => ({ default: {} }))
vi.mock('@/components/tls/Ech.vue', () => ({ default: {} }))
vi.mock('@/components/protocols/Wireguard.vue', () => ({ default: {} }))
vi.mock('@/components/protocols/Warp.vue', () => ({ default: {} }))
vi.mock('@/components/protocols/Tailscale.vue', () => ({ default: {} }))
vi.mock('@/components/Listen.vue', () => ({ default: {} }))
vi.mock('@/components/services/Derp.vue', () => ({ default: {} }))
vi.mock('@/components/services/Ocm.vue', () => ({ default: {} }))
vi.mock('@/components/services/Ccm.vue', () => ({ default: {} }))
vi.mock('@/components/services/SSMAPI.vue', () => ({ default: {} }))
vi.mock('@/components/Rule.vue', () => ({ default: {} }))
vi.mock('@/components/DnsRule.vue', () => ({ default: {} }))
import Client from './Client.vue'
import ClientAddBulk from './ClientAddBulk.vue'
import ClientEditBulk from './ClientEditBulk.vue'
import Dns from './Dns.vue'
import DnsRule from './DnsRule.vue'
import Endpoint from './Endpoint.vue'
import OutboundBulk from './OutboundBulk.vue'
import Rule from './Rule.vue'
import RuleImport from './RuleImport.vue'
import Ruleset from './Ruleset.vue'
import RulesetImport from './RulesetImport.vue'
import Service from './Service.vue'
import Tls from './Tls.vue'
import Token from './Token.vue'
import UsageStats from './UsageStats.vue'
import { defaultInTls, defaultOutTls } from '@/types/tls'
import { createClient } from '@/types/clients'

const apps: App[] = []
const flush = async () => { for (let i = 0; i < 12; i++) await Promise.resolve(); await nextTick() }
function deferred() {
  let resolve!: (value: any) => void, reject!: (reason: any) => void
  const promise = new Promise<any>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
function renderFor(filename: string) {
  const source = parse(readFileSync(new URL(filename, import.meta.url), 'utf8')).descriptor.template!.content
  const compiled = compileTemplate({ source, filename, id: 'round7', compilerOptions: { expressionPlugins: ['typescript'] } })
  expect(compiled.errors).toEqual([])
  const code = ts.transpileModule(compiled.code, { compilerOptions: { target: ts.ScriptTarget.ESNext, module: ts.ModuleKind.ESNext } }).outputText
    .replace(/import \{([\s\S]*?)\} from ["']vue["'];?/, (_, names) => 'const {' + names.replace(/ as /g, ': ') + '} = Vue')
    .replace('export function render', 'function render')
  return { render: new Function('Vue', code + '; return render')(Vue), source }
}
function mount(component: any, values: any = {}, template?: string) {
  const props = reactive({ visible: true, id: 1, index: -1, data: '', clients: [], inboundTags: [], groups: [], outTags: [], serverTags: [], rsTags: [], ...values })
  const save = vi.fn(), close = vi.fn(), updateVisible = vi.fn(), instance = shallowRef<any>()
  const compiled = template ? renderFor(template) : null
  const child = { ...component, render: compiled?.render ?? (() => null) }
  const app = createApp({ render: () => h(child, { ...props, ref: instance, onSave: save, onClose: close, 'onUpdate:visible': updateVisible }) })
  app.config.globalProperties.$t = (key: string) => key
  if (compiled) {
    const stub = { inheritAttrs: false, setup: (_: any, ctx: any) => () => h('div', ctx.attrs, ctx.slots.default?.()) }
    for (const tag of new Set([...compiled.source.matchAll(/<([A-Z][\w-]*|v-[\w-]+)/g)].map(m => m[1]))) {
      if (tag !== 'v-btn') app.component(tag, stub)
    }
    app.component('v-btn', { inheritAttrs: false, setup: (_: any, ctx: any) => () => h('button', ctx.attrs, ctx.slots.default?.()) })
    app.directive('tooltip', {})
  }
  apps.push(app)
  const container = document.createElement('div'); app.mount(container)
  return { state: instance.value as any, props, save, close, updateVisible, container, unmount: () => { app.unmount(); apps.splice(apps.indexOf(app), 1) } }
}
const fullClient = (id = 1) => createClient({ id, name: 'mock-client-' + id, volume: 1024 ** 3, expiry: 2000000000, inbounds: [10, 2], delayStart: true, autoReset: true, resetDays: 7, nextReset: 1900000000, totalUp: 123, totalDown: 456, up: 10, down: 20 })
const wgData = () => ({ id: 1, type: 'wireguard', tag: 'mock-wg', private_key: 'mock-old', peers: [], ext: { public_key: 'mock-old-pub', keys: [] } })
const keyReply = (suffix = '') => ({ success: true, obj: ['PrivateKey: mock-private' + suffix, 'PublicKey: mock-public' + suffix] })
const pemReply = () => ({ success: true, obj: ['-----BEGIN PRIVATE KEY-----', 'mock-key-body', '-----END PRIVATE KEY-----', '-----BEGIN CERTIFICATE-----', 'mock-cert-body', '-----END CERTIFICATE-----'] })
beforeEach(() => {
  mocks.get.mockResolvedValue({ success: false }); mocks.post.mockResolvedValue({ success: false }); mocks.save.mockResolvedValue(false)
  mocks.loadClients.mockImplementation(async (id: number) => fullClient(id))
  mocks.checkName.mockReturnValue(false); mocks.checkBulk.mockReturnValue(false); mocks.checkTag.mockReturnValue(false)
  mocks.fetch.mockRejectedValue(new Error('unconfigured mock fetch')); vi.stubGlobal('fetch', mocks.fetch)
})
afterEach(() => { apps.splice(0).forEach(app => app.unmount()); vi.unstubAllGlobals(); vi.resetAllMocks() })

const eventModals = [
  ['Dns', Dns, 'save', 'dnsServer', 'tag'], ['DnsRule', DnsRule, 'saveChanges', 'ruleData', 'server'],
  ['Rule', Rule, 'saveChanges', 'ruleData', 'outbound'], ['Ruleset', Ruleset, 'saveChanges', 'rule_set', 'tag'], ['Tls', Tls, 'saveChanges', 'tls', 'name'],
] as const

describe('round7 synchronous event contracts', () => {
  it.each(eventModals)('%s initializes already-visible and emits an isolated draft', (_name, component, method, field, key) => {
    const { state, save } = mount(component, { id: 0 })
    state[field][key] = 'mock-submitted'; state[method]()
    expect(save).toHaveBeenCalledOnce(); const payload = save.mock.calls[0][0]
    state[field][key] = 'later'; expect(payload[key]).toBe('mock-submitted')
  })
  it.each(eventModals)('%s closes malformed JSON and refuses saving', (_name, component, method) => {
    const { state, close, save } = mount(component, { id: 1, index: 0, data: '[null]' })
    expect(close).toHaveBeenCalledOnce(); state[method](); expect(save).not.toHaveBeenCalled()
  })
  it.each([['Rule', Rule], ['DnsRule', DnsRule]] as const)('%s rejects invalid logical rule children', (_name, component) => {
    const { close, save, state } = mount(component, { index: 0, data: JSON.stringify({ type: 'logical', action: 'route', rules: [null, 5, []] }) })
    expect(close).toHaveBeenCalledOnce(); state.saveChanges(); expect(save).not.toHaveBeenCalled()
  })
  it.each([['Rule', Rule], ['DnsRule', DnsRule]] as const)('%s preserves simple match fields while filtering stale action options', (_name, component) => {
    const { state, save } = mount(component, { index: 0, data: JSON.stringify({ action: 'reject', domain: ['example.test'], method: 'drop', outbound: 'old', server: 'old' }) })
    state.saveChanges(); expect(save.mock.calls[0][0]).toMatchObject({ action: 'reject', domain: ['example.test'], method: 'drop' })
    expect(save.mock.calls[0][0].server).toBeUndefined(); if (_name === 'Rule') expect(save.mock.calls[0][0].outbound).toBeUndefined()
  })
  it('keeps zero DNS rewrite TTL and clears comma list null inputs', () => {
    const { state, save } = mount(DnsRule); state.ruleData.rewrite_ttl = 0; state.answer = ' a, ,b '; expect(state.ruleData.answer).toEqual(['a', 'b'])
    state.answer = null; state.saveChanges(); expect(save.mock.calls[0][0].rewrite_ttl).toBe(0); expect(state.ruleData.answer).toBeUndefined()
  })
  it('serializes hosts paths/maps and prototype-like hostnames as ordinary JSON keys', () => {
    const { state, save } = mount(Dns); state.changeType('hosts'); state.hostsPath = ' /mock/a, ,/mock/b '
    state.hostsPredefined = [{ name: '__proto__', value: ' 192.0.2.1, ,::1 ' }]; state.save()
    expect(save.mock.calls[0][0].path).toEqual(['/mock/a', '/mock/b']); expect(Object.hasOwn(save.mock.calls[0][0].predefined, '__proto__')).toBe(true)
    state.update_pds_value(99, 'ignored'); state.hostsPath = null; expect(state.dnsServer.path).toBeUndefined()
  })
  it.each([['1h30m', 90 / 1440], ['500ms', 0.5 / 86400], ['2d', 2], ['invalid', 0]])('displays ruleset duration %s without rewriting untouched data', (duration, days) => {
    const { state, save } = mount(Ruleset, { index: 0, data: JSON.stringify({ type: 'remote', tag: 'mock', update_interval: duration }) })
    expect(state.update_intervals).toBeCloseTo(Number(days), 12); state.saveChanges(); expect(save.mock.calls[0][0].update_interval).toBe(duration)
    state.updateType('local'); expect(state.rule_set.update_interval).toBeUndefined()
  })
})

describe('round7 client drafts and bulk contracts', () => {
  it('cannot save fallback defaults after an edit load throws', async () => {
    mocks.loadClients.mockRejectedValue(new Error('mock offline')); const { state, close } = mount(Client); await flush(); await state.saveChanges()
    expect(close).toHaveBeenCalledOnce(); expect(mocks.save).not.toHaveBeenCalled(); expect(state.loading).toBe(false)
  })
  it('keeps the newest loaded client and does not alias store data', async () => {
    const old = deferred(), latest = deferred(); mocks.loadClients.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const { state, props } = mount(Client); props.id = 2; await nextTick(); const source = fullClient(2)
    latest.resolve(source); await flush(); old.resolve(fullClient(1)); await flush()
    expect(state.client.id).toBe(2); state.client.config.vless.uuid = 'later'; expect(source.config!.vless.uuid).toBe('mock-uuid')
  })
  it('sorts clearable inbound IDs numerically without mutating source and preserves reset-day invariants', async () => {
    const { state } = mount(Client); await flush(); const ids = [10, 2]; state.clientInbounds = ids
    expect(state.client.inbounds).toEqual([2, 10]); expect(ids).toEqual([10, 2]); state.clientInbounds = null; expect(state.client.inbounds).toEqual([])
    state.delayStart = false; expect(state.client.resetDays).toBe(7); state.autoReset = false; expect(state.client.resetDays).toBe(0)
    state.autoReset = true; state.resetDays = 2.9; expect(state.client.resetDays).toBe(2); expect(Number.isInteger(state.client.nextReset)).toBe(true)
  })
  it('submits a detached client config with renamed users and excludes local links', async () => {
    const { state } = mount(Client); await flush(); state.client.name = 'renamed'; state.extLinks = [{ type: 'external', uri: 'mock://link' }]
    await state.saveChanges(); const payload = mocks.save.mock.calls[0][2]
    expect(payload.config.vless.name).toBe('renamed'); expect(state.clientConfig.vless.name).toBe('mock-client-1')
    state.extLinks[0].uri = 'later'; expect(payload.links[0].uri).toBe('mock://link')
  })
  it('releases add-bulk loading after duplicate names and validates integer count/reset days', async () => {
    const { state } = mount(ClientAddBulk); mocks.checkBulk.mockReturnValue(true); await state.saveChanges(); expect(state.loading).toBe(false); expect(mocks.save).not.toHaveBeenCalled()
    mocks.checkBulk.mockReturnValue(false); state.count = 1.5; await state.saveChanges(); state.count = 1; state.bulkData.autoReset = true; state.bulkData.resetDays = 0; await state.saveChanges(); expect(mocks.save).not.toHaveBeenCalled()
  })
  it('makes independent numerically sorted bulk payloads with bytes/seconds units', async () => {
    const { state } = mount(ClientAddBulk); state.count = 2; state.bulkData.clientInbounds = [10, 2]; state.bulkData.Volume = 1.5
    await state.saveChanges(); const saved = mocks.save.mock.calls[0][2]; expect(saved[0].volume).toBe(1.5 * 1024 ** 3); expect(saved[0].inbounds).toEqual([2, 10])
    saved[0].inbounds.push(99); expect(saved[1].inbounds).toEqual([2, 10]); expect(state.bulkData.clientInbounds).toEqual([10, 2])
  })
  it('enables the rendered save button for all-mode with no explicit values', async () => {
    const { state, container } = mount(ClientEditBulk, { clients: [{ id: 1 }] }, './ClientEditBulk.vue'); state.selectedClients.model = 'all'; await nextTick()
    const button = [...container.querySelectorAll('button')].find(b => b.textContent?.includes('actions.save'))!
    expect(button).toBeDefined(); expect(button.disabled).toBe(false)
  })
  it('hydrates summary rows before bulk edits and preserves reset settings/lifetime counters', async () => {
    const rows = [{ id: 1, name: 'summary', enable: true, volume: 10, expiry: 20, inbounds: [], up: 0, down: 0 }]
    const { state } = mount(ClientEditBulk, { clients: rows }); state.selectedClients = { model: 'all', values: [] }; state.actionMode = 'add_inbounds'; state.editData.inboundTags = [5]
    await state.saveChanges(); expect(mocks.loadClients).toHaveBeenCalledWith(1); const saved = mocks.save.mock.calls[0][2][0]
    expect(saved).toMatchObject({ totalUp: 123, totalDown: 456, autoReset: true, delayStart: true, resetDays: 7, nextReset: 1900000000, inbounds: [2, 5, 10] })
    expect(rows[0].inbounds).toEqual([])
  })
  it('does not save a partial bulk edit if any full client fails to load', async () => {
    mocks.loadClients.mockResolvedValueOnce(fullClient(1)).mockResolvedValueOnce({}); const { state } = mount(ClientEditBulk, { clients: [{ id: 1 }, { id: 2 }] })
    state.selectedClients = { model: 'all', values: [] }; await state.saveChanges(); expect(mocks.save).not.toHaveBeenCalled(); expect(state.loading).toBe(false)
  })
  it('snapshots bulk action and stops hydration after close', async () => {
    const reply = deferred(); mocks.loadClients.mockReturnValue(reply.promise); const { state } = mount(ClientEditBulk, { clients: [{ id: 1 }, { id: 2 }] })
    state.selectedClients = { model: 'all', values: [] }; const pending = state.saveChanges(); state.actionMode = 'delete_bulk'; state.closeModal(); reply.resolve(fullClient(1)); await pending
    expect(mocks.loadClients).toHaveBeenCalledTimes(1); expect(mocks.save).not.toHaveBeenCalled()
  })
  it('deletes selected IDs without loading credentials', async () => {
    const { state } = mount(ClientEditBulk, { clients: [{ id: 1, group: 'a' }, { id: 2, group: 'b' }] }); state.selectedClients = { model: 'group', values: ['a'] }; state.actionMode = 'delete_bulk'
    await state.saveChanges(); expect(mocks.loadClients).not.toHaveBeenCalled(); expect(mocks.save).toHaveBeenCalledWith('clients', 'delbulk', [1])
  })
})

describe('round7 saves and WireGuard async ownership', () => {
  it.each([['Client', Client], ['ClientAddBulk', ClientAddBulk], ['ClientEditBulk', ClientEditBulk], ['Endpoint', Endpoint], ['Service', Service]] as const)('%s blocks duplicate saves and ignores success after close/reopen', async (name, component) => {
    const { state, props, close } = mount(component, { data: JSON.stringify(name === 'Endpoint' ? wgData() : { id: 1, type: 'derp', tag: 'mock' }), clients: [fullClient()] }); await flush()
    if (name === 'ClientEditBulk') state.selectedClients = { model: 'all', values: [] }
    const reply = deferred(); mocks.save.mockReturnValue(reply.promise); const first = state.saveChanges(), second = state.saveChanges(); await flush()
    expect(mocks.save).toHaveBeenCalledOnce(); state.closeModal(); props.visible = false; await nextTick(); props.visible = true; await nextTick(); const closes = close.mock.calls.length
    reply.resolve(true); await Promise.all([first, second]); expect(close).toHaveBeenCalledTimes(closes); expect(state.closed).toBe(false)
  })
  it.each([['Client', Client], ['ClientAddBulk', ClientAddBulk], ['ClientEditBulk', ClientEditBulk], ['Endpoint', Endpoint], ['Service', Service]] as const)('%s retains draft and releases loading after a thrown save', async (name, component) => {
    const { state } = mount(component, { data: JSON.stringify(name === 'Endpoint' ? wgData() : { id: 1, type: 'derp', tag: 'mock' }), clients: [fullClient()] }); await flush()
    if (name === 'ClientEditBulk') state.selectedClients = { model: 'all', values: [] }
    mocks.save.mockRejectedValue(new Error('mock offline')); await state.saveChanges(); expect(state.loading).toBe(false); expect(state.closed).toBe(false); expect(mocks.error).toHaveBeenCalled()
  })
  it('does not replace a manually edited WG private key with a pending generated pair', async () => {
    const { state } = mount(Endpoint, { data: JSON.stringify(wgData()) }); const reply = deferred(); mocks.get.mockReturnValue(reply.promise)
    const pending = state.newWgKey(); state.endpoint.private_key = 'mock-manual'; reply.resolve(keyReply()); await pending
    expect(state.endpoint.private_key).toBe('mock-manual'); expect(state.endpoint.ext.public_key).toBe('mock-old-pub'); expect(state.loading).toBe(false)
  })
  it('does not start WG requests or unlock a save while a save is pending', async () => {
    const { state } = mount(Endpoint, { data: JSON.stringify(wgData()) }); const reply = deferred(); mocks.save.mockReturnValue(reply.promise)
    const pending = state.saveChanges(); await state.newWgKey(); expect(mocks.get).not.toHaveBeenCalled(); expect(state.loading).toBe(true)
    await state.saveChanges(); expect(mocks.save).toHaveBeenCalledOnce(); reply.resolve(false); await pending; expect(state.loading).toBe(false)
  })
  it('uses latest main-key request and parses exact backend key labels', async () => {
    const { state } = mount(Endpoint, { data: JSON.stringify(wgData()) }); const old = deferred(), latest = deferred(); mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const a = state.newWgKey(), b = state.newWgKey(); latest.resolve(keyReply('-latest')); await b; expect(state.loading).toBe(true)
    old.resolve(keyReply('-old')); await a; expect(state.endpoint.private_key).toBe('mock-private-latest'); expect(state.pendingRequests).toBe(0)
  })
  it('ignores a refreshed peer key after the peer was deleted, not the shifted peer index', async () => {
    const data: any = wgData(); data.peers = [{ public_key: 'a' }, { public_key: 'b' }]
    const { state } = mount(Endpoint, { data: JSON.stringify(data) }); const reply = deferred(); mocks.get.mockReturnValue(reply.promise)
    const pending = state.refreshWgPeerKey(0); state.delWgPeer(0); reply.resolve(keyReply()); await pending; expect(state.endpoint.peers).toEqual([{ public_key: 'b' }]); expect(state.endpoint.ext.keys).toEqual([])
  })
  it('does not append a partial peer when all assigned IPs are occupied', async () => {
    const data: any = wgData(); data.peers = Array.from({ length: 253 }, (_, i) => ({ public_key: String(i), allowed_ips: ['10.0.1.' + (i + 2) + '/32'] }))
    const { state } = mount(Endpoint, { data: JSON.stringify(data) }); mocks.get.mockResolvedValue(keyReply()); await state.addWgPeer()
    expect(state.endpoint.peers).toHaveLength(253); expect(state.endpoint.ext.keys).toEqual([]); expect(state.loading).toBe(false); expect(mocks.error).toHaveBeenCalled()
  })
})

describe('round7 import source ownership and retry contracts', () => {
  it('converts unique outbound tags and creates matching URLTest members without mutating response', async () => {
    const input = [{ type: 'direct', tag: 'node' }, { type: 'direct', tag: 'node' }]; mocks.post.mockResolvedValue({ success: true, obj: input })
    const { state } = mount(OutboundBulk, { outboundTags: ['node', 'node-1'] }); state.addUrlTest = true; await state.linkCheck()
    expect(state.outbounds.map((o: any) => o.tag)).toEqual(['node-2', 'node-3', 'urltest-mock-random']); expect(state.outbounds[2].outbounds).toEqual(['node-2', 'node-3']); expect(input[0].tag).toBe('node')
  })
  it('awaits sequential outbound saves, retries failures only, and never duplicates successes', async () => {
    const { state } = mount(OutboundBulk); state.outbounds = [{ type: 'direct', tag: 'a' }, { type: 'direct', tag: 'b' }]; state.outChecks = [0, 0]
    const reply = deferred(); mocks.save.mockReturnValueOnce(reply.promise).mockResolvedValueOnce(false)
    const pending = state.saveChanges(); expect(mocks.save).toHaveBeenCalledTimes(1); expect(state.loading).toBe(true); reply.resolve(true); await pending
    expect(state.outChecks).toEqual([1, 2]); mocks.save.mockResolvedValue(true); await state.saveChanges(); expect(mocks.save).toHaveBeenCalledTimes(3); expect(mocks.save.mock.calls[2][2].tag).toBe('b')
  })
  it('stops starting subsequent outbound saves immediately after close', async () => {
    const { state } = mount(OutboundBulk); state.outbounds = [{ tag: 'a' }, { tag: 'b' }]; const reply = deferred(); mocks.save.mockReturnValue(reply.promise)
    const pending = state.saveChanges(); state.closeModal(); reply.resolve(true); await pending; expect(mocks.save).toHaveBeenCalledTimes(1); expect(state.outChecks).toEqual([])
  })
  it('invalidates parsed route import on synchronous source edits and preserves event argument contract', () => {
    const { state, save } = mount(RuleImport, { existingRulesCount: 1, existingRulesetsCount: 0, existingRulesetTags: [] })
    state.rawJson = JSON.stringify({ route: { rules: [{ action: 'route', outbound: 'a' }], final: 'a' } }); state.parseJson(); state.applyFinal = true; state.save()
    expect(save.mock.calls[0].slice(1)).toEqual(['merge', true]); state.parsed.rules[0].outbound = 'later'; expect(save.mock.calls[0][0].rules[0].outbound).toBe('a')
    state.rawJson = '{}'; expect(state.parsed).toBeNull(); state.save(); expect(save).toHaveBeenCalledOnce()
  })
  it.each([{ rules: [null] }, { rules: [{ type: 'logical', rules: [3] }] }, { rules: [], rule_set: 'bad' }, { rules: [], final: 3 }])('rejects malformed route blocks %j', value => {
    const { state } = mount(RuleImport); expect(state.extractRouteBlock(value)).toBeNull()
  })
  it('ignores older URL responses and cleared file reads', async () => {
    const { state } = mount(RuleImport); const old = deferred(), latest = deferred(); mocks.fetch.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    state.fetchUrl = 'https://one.test'; const a = state.fetchFromUrl(); state.fetchUrl = 'https://two.test'; const b = state.fetchFromUrl()
    latest.resolve({ ok: true, json: async () => ({ rules: [{ outbound: 'latest' }] }) }); await b; old.resolve({ ok: true, json: async () => ({ rules: [{ outbound: 'old' }] }) }); await a
    expect(state.parsed.rules[0].outbound).toBe('latest'); const read = deferred(); const pending = state.onFileUpload({ text: () => read.promise }); state.onFileUpload(null); read.resolve('{"rules":[]}'); await pending; expect(state.parsed).toBeNull()
  })
  it('parses uploaded ruleset text with real sync watchers, skips duplicate tags and unsafe protocols', async () => {
    const { state, save } = mount(RulesetImport, { rsTags: ['existing'] })
    const urls = 'https://example.test/a.srs\nhttps://other.test/a.srs\nhttps://example.test/existing.srs\njavascript:alert(1)'
    await state.onFileUpload([{ text: async () => urls }]); expect(state.importPreview).toHaveLength(3); expect(state.newCount).toBe(1)
    state.importFormat = 'source'; state.importDetour = 'direct'; state.importInterval = 0.5; state.save()
    expect(save.mock.calls[0][0]).toEqual([{ type: 'remote', tag: 'a', url: 'https://example.test/a.srs', format: 'source', download_detour: 'direct', update_interval: '0.5d' }])
    state.importRawText = 'changed'; expect(state.importPreview).toEqual([])
  })
  it('ignores a ruleset upload after close/reopen and reports a current file failure', async () => {
    const { state, props } = mount(RulesetImport); const read = deferred(); const pending = state.onFileUpload({ text: () => read.promise })
    state.close(); props.visible = false; await nextTick(); props.visible = true; await nextTick(); read.resolve('https://example.test/a.srs'); await pending; expect(state.importPreview).toEqual([])
    await state.onFileUpload({ text: async () => { throw new Error('mock read failure') } }); expect(mocks.error).toHaveBeenCalled()
  })
})

describe('round7 TLS, one-time tokens and usage', () => {
  it('does not apply a self-signed certificate over paths edited during generation', async () => {
    const { state } = mount(Tls, { id: 0 }); state.inTls.key_path = '/mock/old.key'; const reply = deferred(); mocks.get.mockReturnValue(reply.promise)
    const pending = state.genSelfSigned(); state.inTls.key_path = '/mock/new.key'; reply.resolve(pemReply()); await pending
    expect(state.inTls.key_path).toBe('/mock/new.key'); expect(state.inTls.key).toBeUndefined(); expect(state.loading).toBe(false)
  })
  it('requires complete PEM pairs and updates certificate/key atomically', async () => {
    const { state } = mount(Tls, { id: 0 }); state.inTls.key = ['mock-existing']; mocks.get.mockResolvedValue({ success: true, obj: ['-----BEGIN PRIVATE KEY-----', 'incomplete'] })
    await state.genSelfSigned(); expect(state.inTls.key).toEqual(['mock-existing']); expect(mocks.error).toHaveBeenCalled()
    mocks.get.mockResolvedValue(pemReply()); await state.genSelfSigned(); expect(state.inTls.certificate[0]).toBe('-----BEGIN CERTIFICATE-----'); expect(state.usePath).toBe(1)
  })
  it('clones TLS defaults across instances and creates enabled Reality client options', () => {
    const first = mount(Tls, { id: 0 }).state, second = mount(Tls, { id: 0 }).state
    first.optionALPN = true; first.inTls.alpn.push('custom'); expect(defaultInTls.alpn).not.toContain('custom')
    first.tlsType = 1; first.changeTlsType(); second.tlsType = 1; second.changeTlsType(); expect(first.outTls.reality.enabled).toBe(true)
    first.outTls.utls.fingerprint = 'firefox'; expect(second.outTls.utls.fingerprint).toBe('chrome'); expect(defaultOutTls.utls!.fingerprint).toBe('chrome')
  })
  it('does not apply a Reality key response after changing TLS type', async () => {
    const { state } = mount(Tls, { id: 0 }); state.tlsType = 1; state.changeTlsType(); const reply = deferred(); mocks.get.mockReturnValue(reply.promise)
    const pending = state.genRealityKey(); state.tlsType = 0; state.changeTlsType(); reply.resolve(keyReply()); await pending; expect(state.inTls.reality).toBeUndefined(); expect(state.loading).toBe(false)
  })
  it('retains one-time token plaintext if only the nested add dialog was closed during creation', async () => {
    const { state } = mount(Token); await flush(); state.showAddToken(); const reply = deferred(); mocks.post.mockReturnValue(reply.promise)
    const pending = state.addToken(); state.showNewToken = false; reply.resolve({ success: true, obj: 'mock-one-time-token' }); await pending
    expect(state.newToken.token).toBe('mock-one-time-token'); expect(state.loading).toBe(false)
  })
  it('invalidates tokens on identity changes, keeping stale plaintext out of the new session', async () => {
    const { state, props } = mount(Token, { user: 'first' }); await flush(); state.showAddToken(); const reply = deferred(); mocks.post.mockReturnValue(reply.promise)
    const pending = state.addToken(); props.user = 'second'; await nextTick(); reply.resolve({ success: true, obj: 'mock-old-session-token' }); await pending
    expect(state.newToken.token).toBe(''); expect(state.showNewToken).toBe(false)
  })
  it('rejects invalid expiry but accepts zero, blocks duplicate token mutation and waits for refresh', async () => {
    const { state } = mount(Token); await flush(); state.showAddToken(); state.newToken.expiry = -1; await state.addToken(); expect(mocks.post).not.toHaveBeenCalled()
    state.newToken.expiry = 0; const reply = deferred(), refresh = deferred(); mocks.post.mockReturnValue(reply.promise); mocks.get.mockReturnValue(refresh.promise)
    const a = state.addToken(), b = state.addToken(); expect(mocks.post).toHaveBeenCalledWith('api/addToken', { desc: '', expiry: 0 }); expect(mocks.post).toHaveBeenCalledOnce()
    reply.resolve({ success: true, obj: 'mock-new' }); await flush(); expect(state.loading).toBe(true); refresh.resolve({ success: true, obj: [] }); await Promise.all([a, b]); expect(state.loading).toBe(false)
  })
  it('does not publish a token list after unmount or stale refresh', async () => {
    const old = deferred(), latest = deferred(); mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const { state, unmount } = mount(Token); const pending = state.loadData(); latest.resolve({ success: true, obj: [{ id: 2 }] }); await pending
    old.resolve({ success: true, obj: [{ id: 1 }] }); await flush(); expect(state.tokens[0].id).toBe(2)
    const after = deferred(); mocks.get.mockReturnValue(after.promise); const work = state.loadData(); unmount(); after.resolve({ success: true, obj: [{ id: 3 }] }); await work; expect(state.tokens).toEqual([])
  })
  it('initially refreshes usage, keeps latest result and clears data synchronously on local close', async () => {
    const old = deferred(), latest = deferred(); mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const { state, updateVisible } = mount(UsageStats); const pending = state.refresh(); latest.resolve({ success: true, obj: { db: { clientUp: 12, clientDown: 34, clients: 2 } } }); await pending
    old.resolve({ success: true, obj: { clients: 99 } }); await flush(); expect(state.totalUsage).toBe('46'); expect(state.info.clients).toBe(2)
    state.setVisible(false); expect(updateVisible).toHaveBeenCalledWith(false); expect(state.info).toEqual({}); await state.refresh(); expect(mocks.get).toHaveBeenCalledTimes(2)
  })
  it('releases usage loading on throws and suppresses a late response after unmount', async () => {
    mocks.get.mockRejectedValue(new Error('mock offline')); const { state, unmount } = mount(UsageStats); await flush(); expect(state.loading).toBe(false)
    const reply = deferred(); mocks.get.mockReturnValue(reply.promise); const pending = state.refresh(); unmount(); reply.resolve({ success: true, obj: { clients: 7 } }); await pending; expect(state.info.clients).toBeUndefined()
  })
})

describe('round7 final boundary checks', () => {
  it.each([NaN, Infinity, -Infinity])('rejects non-finite bulk limits %s without turning them into zero in a snapshot', async value => {
    const { state } = mount(ClientEditBulk, { clients: [{ id: 1 }] }); state.selectedClients = { model: 'all', values: [] }; state.editData.addVolume = value
    await state.saveChanges(); expect(mocks.save).not.toHaveBeenCalled(); expect(mocks.loadClients).not.toHaveBeenCalled(); expect(state.loading).toBe(false)
  })
  it('uses the submitted action and inbound tags even if the editor changes during hydration', async () => {
    const { state } = mount(ClientEditBulk, { clients: [{ id: 1 }] }); state.selectedClients = { model: 'all', values: [] }; state.actionMode = 'add_inbounds'; state.editData.inboundTags = [5]
    const reply = deferred(); mocks.loadClients.mockReturnValue(reply.promise); const pending = state.saveChanges()
    state.actionMode = 'delete_bulk'; state.editData.inboundTags.push(8); reply.resolve(fullClient()); await pending
    expect(mocks.save).toHaveBeenCalledOnce(); expect(mocks.save.mock.calls[0][1]).toBe('editbulk'); expect(mocks.save.mock.calls[0][2][0].inbounds).toEqual([2, 5, 10])
  })
  it('uses GiB/day increments on full rows without mutating the read response', async () => {
    const source = fullClient(); mocks.loadClients.mockResolvedValue(source); const { state } = mount(ClientEditBulk, { clients: [{ id: 1 }] })
    state.selectedClients = { model: 'all', values: [] }; state.editData.addVolume = 0.5; state.editData.addDays = 0.5; await state.saveChanges()
    const saved = mocks.save.mock.calls[0][2][0]; expect(saved.volume).toBe(1.5 * 1024 ** 3); expect(saved.expiry).toBe(2000043200)
    expect(source.volume).toBe(1024 ** 3); expect(source.expiry).toBe(2000000000); expect(saved.totalUp).toBe(123)
  })
  it('rejects a mismatched full-client identity and does not continue reads after unmount', async () => {
    mocks.loadClients.mockResolvedValue(fullClient(2)); const { state, unmount } = mount(ClientEditBulk, { clients: [{ id: 1 }, { id: 3 }] }); state.selectedClients = { model: 'all', values: [] }
    await state.saveChanges(); expect(mocks.save).not.toHaveBeenCalled(); expect(mocks.loadClients).toHaveBeenCalledTimes(1)
    const reply = deferred(); mocks.loadClients.mockReturnValue(reply.promise); const pending = state.saveChanges(); unmount(); reply.resolve(fullClient()); await pending
    expect(mocks.save).not.toHaveBeenCalled(); expect(mocks.loadClients).toHaveBeenCalledTimes(2)
  })
  it('preserves edited service identity and listen fields across type changes', () => {
    const { state } = mount(Service, { data: JSON.stringify({ id: 5, type: 'derp', tag: 'mock-service', listen: '::1', listen_port: 8123 }) })
    state.srv.type = 'ocm'; state.changeType(); expect(state.srv).toMatchObject({ id: 5, type: 'ocm', tag: 'mock-service', listen: '::1', listen_port: 8123 })
  })
  it('does not generate any WireGuard keys while closing an existing endpoint', async () => {
    const { state } = mount(Endpoint, { data: JSON.stringify(wgData()) }); state.closeModal(); await state.newWgKey(); await state.addWgPeer(); expect(mocks.get).not.toHaveBeenCalled()
  })
  it.each(['certificate_path', 'usePath'])('preserves changed TLS %s while generation is pending', async field => {
    const { state } = mount(Tls, { id: 0 }); const reply = deferred(); mocks.get.mockReturnValue(reply.promise); const pending = state.genSelfSigned()
    if (field === 'usePath') state.usePath = 1; else state.inTls.certificate_path = '/mock/new.crt'
    reply.resolve(pemReply()); await pending; expect(state.inTls.key).toBeUndefined()
    if (field !== 'usePath') expect(state.inTls.certificate_path).toBe('/mock/new.crt')
  })
  it('clears one-time plaintext on parent close and ignores late creation responses', async () => {
    const { state } = mount(Token); await flush(); state.showAddToken(); const reply = deferred(); mocks.post.mockReturnValue(reply.promise); const pending = state.addToken()
    state.closeModal(); reply.resolve({ success: true, obj: 'mock-stale-token' }); await pending; expect(state.newToken.token).toBe(''); expect(state.tokens).toEqual([])
  })
  it('releases token mutation state on a thrown delete and permits retry', async () => {
    const { state } = mount(Token); await flush(); mocks.post.mockRejectedValueOnce(new Error('mock offline')); await state.deleteToken(1)
    expect(state.loading).toBe(false); expect(state.mutating).toBe(false); await state.deleteToken(1); expect(mocks.post).toHaveBeenCalledTimes(2)
  })
  it('makes no usage request while initially hidden', async () => {
    const { state } = mount(UsageStats, { visible: false }); await state.refresh(); expect(mocks.get).not.toHaveBeenCalled(); expect(state.info).toEqual({})
  })
  it.each([
    ['Client', Client, 'saveChanges', 'closeModal'], ['ClientAddBulk', ClientAddBulk, 'saveChanges', 'closeModal'],
    ['ClientEditBulk', ClientEditBulk, 'saveChanges', 'closeModal'], ['Dns', Dns, 'save', 'close'], ['DnsRule', DnsRule, 'saveChanges', 'closeModal'],
    ['Endpoint', Endpoint, 'saveChanges', 'closeModal'], ['OutboundBulk', OutboundBulk, 'saveChanges', 'closeModal'],
    ['Rule', Rule, 'saveChanges', 'closeModal'], ['RuleImport', RuleImport, 'save', 'close'], ['Ruleset', Ruleset, 'saveChanges', 'closeModal'],
    ['RulesetImport', RulesetImport, 'save', 'close'], ['Service', Service, 'saveChanges', 'closeModal'], ['Tls', Tls, 'saveChanges', 'closeModal'], ['Token', Token, 'addToken', 'closeModal'],
  ] as const)('%s does not submit after local close even before parent visible changes', async (_name, component, method, closeMethod) => {
    const { state, save } = mount(component, { id: 0 }); await flush(); state[closeMethod](); mocks.save.mockClear(); mocks.post.mockClear(); save.mockClear()
    await state[method](); expect(mocks.save).not.toHaveBeenCalled(); expect(mocks.post).not.toHaveBeenCalled(); expect(save).not.toHaveBeenCalled()
  })
})
