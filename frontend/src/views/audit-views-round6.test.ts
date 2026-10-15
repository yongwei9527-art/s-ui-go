/// <reference types="node" />
import * as Vue from 'vue'
import { createApp, reactive, nextTick, h, type App } from 'vue'
import { readFileSync } from 'node:fs'
import { parse, compileTemplate } from '@vue/compiler-sfc'
import ts from 'typescript'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), save: vi.fn(), loadInbounds: vi.fn(), route: vi.fn(), error: vi.fn(), store: {} as any }))
vi.mock('@/plugins/httputil', () => ({ default: { get: mocks.get, post: mocks.post } }))
vi.mock('@/store/modules/data', () => ({ default: () => mocks.store }))
vi.mock('@/locales', () => ({ i18n: { global: { t: (key: string) => key, locale: { value: 'en' } } }, languages: [], locale: 'en' }))
vi.mock('notivue', () => ({ push: { error: mocks.error, success: vi.fn() } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ replace: mocks.route, push: mocks.route }) }))
vi.mock('vuetify', () => ({ useTheme: () => ({ change: vi.fn() }), useLocale: () => ({ current: { value: 'en' } }), useDisplay: () => ({ smAndDown: { value: false } }) }))
vi.mock('@/composables/useOneTimeNotice', () => ({ useOneTimeNotice: () => ({ visible: false, show: vi.fn() }) }))
vi.mock('@/layouts/modals/Admin.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Changes.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Token.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Client.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/ClientAddBulk.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/ClientEditBulk.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/QrCode.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Stats.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Dns.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/DnsRule.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Endpoint.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/WgQrCode.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Inbound.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Outbound.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/OutboundBulk.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Rule.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Ruleset.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/RulesetImport.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/RuleImport.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Service.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Tls.vue', () => ({ default: {} }))
vi.mock('@/components/Main.vue', () => ({ default: {} }))
vi.mock('@/components/Dial.vue', () => ({ default: {} }))
vi.mock('@/components/SubJsonExt.vue', () => ({ default: {} }))
vi.mock('@/components/SubClashExt.vue', () => ({ default: {} }))
import Home from './Home.vue'
import Admins from './Admins.vue'
import Basics from './Basics.vue'
import Clients from './Clients.vue'
import Dns from './Dns.vue'
import Endpoints from './Endpoints.vue'
import Inbounds from './Inbounds.vue'
import Login from './Login.vue'
import Outbounds from './Outbounds.vue'
import Rules from './Rules.vue'
import Services from './Services.vue'
import Settings from './Settings.vue'
import Tls from './Tls.vue'

const apps: App[] = []
function mount(component: any) {
  const app = createApp({ ...component, render: () => null })
  app.provide('loading', Vue.ref(false))
  apps.push(app)
  return (app.mount(document.createElement('div')).$ as any).setupState
}
function deferred() {
  let resolve!: (value: any) => void, reject!: (value: any) => void
  const promise = new Promise<any>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
const flush = async () => { for (let i = 0; i < 10; i++) await Promise.resolve(); await nextTick() }
const clone = (value: any) => JSON.parse(JSON.stringify(value))
beforeEach(() => {
  localStorage.clear()
  mocks.store = reactive({ lastLoad: '1', config: { log: { level: 'info' }, dns: { servers: [], rules: [] }, route: { rules: [], rule_set: [], final: 'direct' }, experimental: {} }, inbounds: [], endpoints: [], services: [], outbounds: [], clients: [], tlsConfigs: [], onlines: {}, save: mocks.save, loadInbounds: mocks.loadInbounds })
  mocks.get.mockResolvedValue({ success: false, obj: null })
  mocks.post.mockResolvedValue({ success: false })
  mocks.save.mockResolvedValue(false)
  mocks.loadInbounds.mockResolvedValue([])
  mocks.route.mockResolvedValue(undefined)
})
afterEach(() => { apps.splice(0).forEach(app => app.unmount()); vi.useRealTimers(); vi.resetAllMocks() })

describe('round6 configuration drafts', () => {
  it.each([Basics, Dns, Rules])('releases loading after a rejected save: %s', async component => {
    const state = mount(component); await flush()
    const section = component === Basics ? 'log' : component === Dns ? 'dns' : 'route'
    const field = component === Basics ? 'level' : 'final'
    state.appConfig[section][field] = 'draft'
    await state.saveConfig()
    expect(mocks.save).toHaveBeenCalledTimes(1)
    expect(state.loading).toBe(false)
    expect(state.appConfig[section][field]).toBe('draft')
  })
  it.each([Basics, Dns, Rules])('isolates drafts from background store replacement: %s', async component => {
    const state = mount(component); await flush()
    state.appConfig.log.level = 'debug'
    expect(mocks.store.config.log.level).toBe('info')
    mocks.store.config = { ...clone(mocks.store.config), log: { level: 'warn' } }
    mocks.store.lastLoad = '2'; await flush()
    expect(state.appConfig.log.level).toBe('debug')
  })
  it.each([Basics, Dns, Rules])('preserves edits made while saving and rejects double saves: %s', async component => {
    const state = mount(component); await flush()
    const section = component === Basics ? 'log' : component === Dns ? 'dns' : 'route'
    const field = component === Basics ? 'level' : 'final'
    state.appConfig[section][field] = 'first'
    const reply = deferred(); mocks.save.mockReturnValueOnce(reply.promise)
    const a = state.saveConfig(), b = state.saveConfig()
    expect(mocks.save).toHaveBeenCalledTimes(1)
    const submitted = clone(mocks.save.mock.calls[0][2])
    state.appConfig[section][field] = 'later'
    mocks.store.config = submitted
    reply.resolve(true); await Promise.all([a, b])
    expect(state.appConfig[section][field]).toBe('later')
    expect(state.stateChange).toBe(false)
  })
  it.each([Basics, Dns, Rules])('does not leak polling timers while initial data is unavailable: %s', async component => {
    vi.useFakeTimers(); mocks.store.lastLoad = ''; mocks.store.config = {}
    mount(component); await flush(); apps.pop()!.unmount()
    await vi.advanceTimersByTimeAsync(500)
    expect(vi.getTimerCount()).toBe(0)
  })
  it('accepts newly entered Clash CORS origins when the optional field was absent', async () => {
    mocks.store.config.experimental.clash_api = { external_controller: '127.0.0.1:9090' }
    const state = mount(Basics); await flush(); state.origin = 'https://example.test'
    expect(state.appConfig.experimental.clash_api.access_control_allow_origin).toEqual(['https://example.test'])
  })
  it('attaches route defaults and new rules to a config that omitted route', async () => {
    delete mocks.store.config.route
    const state = mount(Rules); await flush(); state.route.final = 'proxy'; state.saveRuleModal({ action: 'route', outbound: 'proxy' })
    expect(state.appConfig.route.final).toBe('proxy')
    expect(state.appConfig.route.rules).toHaveLength(1)
  })
  it('normalizes DNS only after the initial configuration arrives', async () => {
    mocks.store.lastLoad = ''; mocks.store.config = {}
    const state = mount(Dns)
    mocks.store.config = { log: {}, experimental: {}, route: {} }; mocks.store.lastLoad = '1'; await flush()
    expect(state.dns.servers).toEqual([]); expect(state.dnsRules).toEqual([])
  })
})

describe('round6 login lifecycle', () => {
  it('sends a single login and navigates without a late timer', async () => {
    vi.useFakeTimers(); const state = mount(Login); state.username = 'mock-user'; state.password = 'mock-password'
    const reply = deferred(); mocks.post.mockReturnValueOnce(reply.promise)
    const timer = vi.spyOn(globalThis, 'setTimeout')
    const a = state.login(), b = state.login()
    expect(mocks.post).toHaveBeenCalledTimes(1)
    reply.resolve({ success: true }); await Promise.all([a, b])
    expect(mocks.route).toHaveBeenCalledWith('/')
    expect(timer.mock.calls.some(([, delay]) => delay === 500)).toBe(false)
    timer.mockRestore(); expect(state.loading).toBe(false)
  })
  it('ignores authentication completion after the login page is unmounted', async () => {
    const state = mount(Login); state.username = 'mock-user'; state.password = 'mock-password'
    const reply = deferred(); mocks.post.mockReturnValueOnce(reply.promise); const pending = state.login()
    apps.pop()!.unmount(); reply.resolve({ success: true }); await pending
    expect(mocks.route).not.toHaveBeenCalled(); expect(localStorage.getItem('s-ui-authenticated')).toBeNull()
  })
  it('recovers from storage failure after the server accepted login', async () => {
    const state = mount(Login); state.username = 'mock-user'; state.password = 'mock-password'
    mocks.post.mockResolvedValue({ success: true }); const storage = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('mock storage blocked') })
    try { await state.login(); expect(state.loading).toBe(false) } finally { storage.mockRestore() }
  })
})

describe('round6 list and request identity', () => {
  it('replaces administrator results instead of accumulating duplicates', async () => {
    mocks.get.mockResolvedValue({ success: true, obj: [{ id: 1, username: 'mock-admin', lastLogin: '2026-01-02 15:04:05 192.0.2.1' }] })
    const state = mount(Admins); await flush(); await state.loadData()
    expect(state.users).toHaveLength(1)
    expect(state.users[0].loginTime).toBe(new Date('2026-01-02 15:04:05').toLocaleTimeString('en'))
  })
  it('keeps newer administrator data when an older response finishes last', async () => {
    const old = deferred(), latest = deferred(); mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const state = mount(Admins); const pending = state.loadData()
    latest.resolve({ success: true, obj: [{ id: 2, username: 'latest', lastLogin: '' }] }); await pending
    old.resolve({ success: true, obj: [{ id: 1, username: 'old', lastLogin: '' }] }); await flush()
    expect(state.users.map((u: any) => u.id)).toEqual([2])
  })
  it('releases inbound clone loading for an empty load result', async () => {
    const state = mount(Inbounds)
    await state.clone(1); expect(state.cloneLoading).toBe(false); expect(mocks.save).not.toHaveBeenCalled()
  })
  it('does not create two clones or save after the page is unmounted', async () => {
    const reply = deferred(); mocks.loadInbounds.mockReturnValue(reply.promise)
    const state = mount(Inbounds); const a = state.clone(1), b = state.clone(1)
    expect(mocks.loadInbounds).toHaveBeenCalledTimes(1)
    apps.pop()!.unmount(); reply.resolve([{ id: 1, type: 'vless', tag: 'mock', tls_id: 0 }]); await Promise.all([a, b])
    expect(mocks.save).not.toHaveBeenCalled()
  })
  it('prevents overlapping checks of the same outbound, including all-checks', async () => {
    mocks.store.outbounds = [{ tag: 'a' }]; const reply = deferred(); mocks.get.mockReturnValue(reply.promise)
    const state = mount(Outbounds); const a = state.checkOutbound('a'), b = state.checkAllOutbounds(), c = state.checkAllOutbounds()
    expect(mocks.get).toHaveBeenCalledTimes(1)
    reply.resolve({ success: true, obj: { OK: true, Delay: 10 } }); await Promise.all([a, b, c])
    expect(state.checkResults.a.loading).toBe(false); expect(state.testingAll).toBe(false)
  })
  it('continues bounded all-check workers after one transport throws', async () => {
    mocks.store.outbounds = Array.from({ length: 10 }, (_, i) => ({ tag: String(i) }))
    mocks.get.mockRejectedValueOnce(new Error('mock offline')).mockResolvedValue({ success: true, obj: { OK: true, Delay: 1 } })
    const state = mount(Outbounds); await state.checkAllOutbounds()
    expect(mocks.get).toHaveBeenCalledTimes(10); expect(state.checkResults['0'].loading).toBe(false)
  })
  it('refreshes an applied client filter when the store replaces/deletes clients', async () => {
    mocks.store.clients = [{ id: 1, name: 'a', group: 'one' }, { id: 2, name: 'b', group: 'two' }]
    const state = mount(Clients); state.filterSettings.group = 'one'; state.doFilter()
    expect(state.groups).toEqual(['one', 'two'])
    mocks.store.clients = [{ id: 3, name: 'new', group: 'one' }]; await flush()
    expect(state.filterSettings.filteredClients.map((c: any) => c.id)).toEqual([3])
  })
  it.each([[Inbounds, 'delInbound'], [Services, 'delSrv']])('ignores deletion for an already removed row: %s', async (component, method) => {
    const state = mount(component); await state[method](999); expect(mocks.save).not.toHaveBeenCalled()
  })
  it.each([[Endpoints, 'endpoints', 'delEndpoint', 'tag'], [Outbounds, 'outbounds', 'delOutbound', 'tag'], [Services, 'services', 'delSrv', 1], [Inbounds, 'inbounds', 'delInbound', 1], [Clients, 'clients', 'delClient', 1]])('does not move an open delete confirmation to another row: %s', async (component, collection, method, key) => {
    mocks.store[collection as string] = [{ id: 1, tag: 'tag', name: 'one' }, { id: 2, tag: 'other', name: 'two' }]
    const state = mount(component); state.delOverlay[1] = true
    mocks.save.mockImplementation(async () => { mocks.store[collection as string].shift(); return true })
    await state[method as string](key); await flush()
    expect(Object.values(state.delOverlay).some(Boolean)).toBe(false)
  })
})

describe('round6 TLS and settings boundaries', () => {
  it('blocks TLS deletion if a service uses the selected profile', async () => {
    mocks.store.tlsConfigs = [{ id: 1, name: 'used' }]; mocks.store.services = [{ tag: 'derp', tls_id: 1 }]
    const state = mount(Tls); await state.delTls(1); expect(mocks.save).not.toHaveBeenCalled()
  })
  it('does not close another TLS editor when a prior save completes', async () => {
    mocks.store.tlsConfigs = [{ id: 1 }, { id: 2 }]; const reply = deferred(); mocks.save.mockReturnValue(reply.promise)
    const state = mount(Tls); state.showModal(1); const pending = state.saveModal({ id: 1, name: 'one' }); state.closeModal(); state.showModal(2)
    reply.resolve(true); await pending; expect(state.modal.visible).toBe(true); expect(state.modal.id).toBe(2)
  })
  it('builds a valid IPv6 restart URL with normalized base path', () => {
    const state = mount(Settings)
    expect(state.buildURL('2001:db8::1', '443', true, 'panel')).toBe('https://[2001:db8::1]/panel/settings')
    expect(state.buildURL('example.test', '80', false, '/')).toBe('http://example.test/settings')
  })
  it('does not allow saving fallback defaults after initial settings loading fails', async () => {
    const state = mount(Settings); await flush(); await state.save(); expect(mocks.post).not.toHaveBeenCalled()
  })
  it('preserves settings edits made during an accepted save', async () => {
    const state = mount(Settings); await flush(); state.setData(clone(state.settings))
    state.settings.webDomain = 'submitted.test'; const reply = deferred(); mocks.post.mockReturnValue(reply.promise)
    const submitted = clone(state.settings); const pending = state.save(); state.settings.webDomain = 'later.test'
    reply.resolve({ success: true, obj: { settings: submitted } }); await pending
    expect(state.settings.webDomain).toBe('later.test'); expect(state.stateChange).toBe(true)
  })
  it('allows zero subscription update hours instead of silently saving twelve', () => {
    const state = mount(Settings); state.subUpdates = 0; expect(state.settings.subUpdates).toBe('0')
  })
})

// Compile the real template with lightweight presentation stubs; setup stays real.
function renderView(component: any, filename: string) {
  const source = readFileSync(new URL(filename, import.meta.url), 'utf8')
  const template = parse(source).descriptor.template!.content
  const compiled = compileTemplate({ source: template, filename, id: 'round6', compilerOptions: { expressionPlugins: ['typescript'] } })
  expect(compiled.errors).toEqual([])
  const code = ts.transpileModule(compiled.code, { compilerOptions: { target: ts.ScriptTarget.ESNext, module: ts.ModuleKind.ESNext } }).outputText
    .replace(/import \{([\s\S]*?)\} from ["']vue["'];?/, (_, names) => 'const {' + names.replace(/ as /g, ': ') + '} = Vue')
    .replace('export function render', 'function render')
  const render = new Function('Vue', code + '; return render')(Vue)
  const app = createApp({ ...component, render: function(this: any, context: any, ...args: any[]) {
    const setup = (Vue.getCurrentInstance() as any).setupState
    const proxy = new Proxy(context, { get: (target, key) => key in setup ? setup[key] : target[key] })
    return render(proxy, ...args)
  } })
  app.config.globalProperties.$t = (key: string) => key
  app.config.globalProperties.$i18n = { locale: 'en', fallbackLocale: 'en', availableLocales: ['en'] }
  app.provide('loading', Vue.ref(false))
  const stub = { inheritAttrs: false, setup: (_: any, context: any) => () => h('div', { icon: context.attrs.icon }, context.slots.default?.()) }
  for (const tag of new Set([...template.matchAll(/<([A-Z][\w-]*|v-[\w-]+)/g)].map(match => match[1]))) app.component(tag, stub)
  app.directive('tooltip', {})
  apps.push(app)
  const container = document.createElement('div'); app.mount(container)
  return container
}

describe('round6 additional error and safety regressions', () => {
  it.each([Basics, Dns, Rules])('handles thrown saves without losing a draft: %s', async component => {
    const state = mount(component); await flush()
    const section = component === Basics ? 'log' : component === Dns ? 'dns' : 'route'
    const field = component === Basics ? 'level' : 'final'
    state.appConfig[section][field] = 'draft'
    mocks.save.mockRejectedValue(new Error('mock offline'))
    await state.saveConfig(); expect(state.loading).toBe(false); expect(state.appConfig[section][field]).toBe('draft')
  })
  it.each([Basics, Dns, Rules])('does not overwrite unrelated current configuration: %s', async component => {
    const state = mount(component); await flush()
    const section = component === Basics ? 'log' : component === Dns ? 'dns' : 'route'
    const field = component === Basics ? 'level' : 'final'
    state.appConfig[section][field] = 'draft'
    mocks.store.config.other_section = { keep: 'latest' }
    await state.saveConfig(); expect(mocks.save.mock.calls[0][2].other_section).toEqual({ keep: 'latest' })
  })
  it.each([Basics, Dns, Rules])('refuses a same-section background conflict while retaining the draft: %s', async component => {
    const state = mount(component); await flush()
    const section = component === Basics ? 'log' : component === Dns ? 'dns' : 'route'
    const field = component === Basics ? 'level' : 'final'
    state.appConfig[section][field] = 'draft'; mocks.store.config[section][field] = 'external'
    await state.saveConfig(); expect(mocks.save).not.toHaveBeenCalled(); expect(state.appConfig[section][field]).toBe('draft')
  })
  it('keeps administrator payloads immutable and updates a successfully renamed user', async () => {
    mocks.get.mockResolvedValue({ success: true, obj: [{ id: 1, username: 'before', lastLogin: '' }] })
    const state = mount(Admins); await flush(); state.showEditModal(state.users[0])
    const data = { id: 1, oldPass: 'mock-old', newPass: 'mock-new', newUsername: 'after' }
    const reply = deferred(); mocks.post.mockReturnValue(reply.promise); const a = state.saveEditModal(data), b = state.saveEditModal(data)
    data.newUsername = ''; expect(mocks.post).toHaveBeenCalledTimes(1); expect(mocks.post.mock.calls[0][1].newUsername).toBe('after')
    reply.resolve({ success: true }); await Promise.all([a, b]); expect(state.users[0].username).toBe('after'); expect(state.loading).toBe(false)
  })
  it('releases administrator loading on thrown reads and password changes', async () => {
    mocks.get.mockRejectedValue(new Error('mock offline')); const state = mount(Admins); await flush(); expect(state.loading).toBe(false)
    mocks.post.mockRejectedValue(new Error('mock offline')); await state.saveEditModal({}); expect(state.loading).toBe(false)
  })
  it('keeps new administrator editors open when old saves complete', async () => {
    const state = mount(Admins); await flush(); state.showEditModal({ id: 1 })
    const reply = deferred(); mocks.post.mockReturnValue(reply.promise); const pending = state.saveEditModal({ id: 1 }); state.closeEditModal(); state.showEditModal({ id: 2 })
    reply.resolve({ success: true }); await pending; expect(state.editModal.visible).toBe(true); expect(state.editModal.user.id).toBe(2)
  })
  it('keeps password-change drafts open on failure without delayed timers', async () => {
    const state = mount(Admins); await flush(); state.showEditModal({ id: 1 }); await state.saveEditModal({ id: 1 }); expect(state.editModal.visible).toBe(true)
  })
  it.each([Endpoints, Outbounds, Services, Tls])('ignores edits of rows removed by a concurrent refresh: %s', component => {
    const state = mount(component); state.showModal(999); expect(state.modal.visible).toBe(false)
  })
  it.each([Endpoints, Outbounds, Inbounds])('handles absent online data: %s', component => {
    mocks.store.onlines = null; const state = mount(component); expect(state.onlines).toEqual([])
  })
  it('maintains at most eight concurrent outbound checks', async () => {
    mocks.store.outbounds = Array.from({ length: 17 }, (_, id) => ({ tag: String(id) }))
    let running = 0, maximum = 0
    mocks.get.mockImplementation(async () => { running++; maximum = Math.max(running, maximum); await Promise.resolve(); running--; return { success: true, obj: { OK: true, Delay: 1 } } })
    await mount(Outbounds).checkAllOutbounds(); expect(maximum).toBeLessThanOrEqual(8); expect(mocks.get).toHaveBeenCalledTimes(17)
  })
  it('allows another outbound check after a synchronous adapter failure', async () => {
    mocks.get.mockImplementationOnce(() => { throw new Error('mock synchronous error') })
    const state = mount(Outbounds); await state.checkOutbound('a'); await state.checkOutbound('a'); expect(mocks.get).toHaveBeenCalledTimes(2)
  })
  it('stops all-check workers after unmount rather than starting further requests', async () => {
    mocks.store.outbounds = Array.from({ length: 17 }, (_, id) => ({ tag: String(id) })); const reply = deferred(); mocks.get.mockReturnValue(reply.promise)
    const state = mount(Outbounds); const pending = state.checkAllOutbounds(); expect(mocks.get).toHaveBeenCalledTimes(8)
    apps.pop()!.unmount(); reply.resolve({ success: true, obj: { OK: true, Delay: 1 } }); await pending; expect(mocks.get).toHaveBeenCalledTimes(8)
  })
  it('uses TLS profile enabled state rather than only a nonzero reference', () => {
    mocks.store.tlsConfigs = [{ id: 1, server: { enabled: false } }, { id: 2, server: { enabled: true } }]
    for (const component of [Inbounds, Services]) { const state = mount(component); expect(state.tlsEnabled(1)).toBe(false); expect(state.tlsEnabled(2)).toBe(true); expect(state.tlsEnabled(999)).toBe(false) }
  })
  it.each(['tls', 'https', 'quic', 'h3'])('shows implicit DNS TLS as enabled for %s', type => {
    const state = mount(Dns); expect(state.dnsTlsStatus({ type })).toBe('enable')
  })
  it('renders disabled ECH and Reality flags as no, and hides service-used TLS deletion', () => {
    mocks.store.tlsConfigs = [{ id: 1, name: 'profile', server: { enabled: true, ech: { enabled: false }, reality: { enabled: false } } }]
    mocks.store.services = [{ tls_id: 1, tag: 'service' }]
    const dom = renderView(Tls, './Tls.vue')
    expect(dom.textContent).toContain('ECHno'); expect(dom.textContent).toContain('Realityno')
    expect(dom.querySelector('[icon="mdi-file-remove"]')).toBeNull()
  })
  it.each([Basics, Dns, Rules])('renders safely before any initial configuration exists: %s', component => {
    mocks.store.lastLoad = ''; mocks.store.config = {}
    const filename = component === Basics ? './Basics.vue' : component === Dns ? './Dns.vue' : './Rules.vue'
    expect(() => renderView(component, filename)).not.toThrow()
  })
  it('keeps custom optional Basics configuration on repeated enable events', async () => {
    mocks.store.config.ntp = { enabled: false, server: 'ntp.example.test', interval: '15m' }
    mocks.store.config.experimental = { cache_file: { enabled: false, path: 'mock-cache.db' }, clash_api: { external_controller: '127.0.0.1:9191', secret: 'mock-secret' }, v2ray_api: { listen: '127.0.0.1:8181', stats: { enabled: true } } }
    const state = mount(Basics); await flush(); state.enableNtp = true; state.enableCacheFile = true; state.enableClashApi = true; state.enableV2rayApi = true
    expect(state.appConfig.ntp.server).toBe('ntp.example.test'); expect(state.appConfig.experimental.cache_file.path).toBe('mock-cache.db')
    expect(state.appConfig.experimental.clash_api.secret).toBe('mock-secret'); expect(state.appConfig.experimental.v2ray_api.listen).toBe('127.0.0.1:8181')
  })
  it.each(['javascript:alert(1)', 'data:text/html,mock', 'https://mock-user:mock-password@example.test/'])('blocks unsafe saved restart URI %s before posting a restart', async webURI => {
    const state = mount(Settings); await flush(); state.setData({ ...clone(state.settings), webURI }); await state.restartApp()
    expect(mocks.post).not.toHaveBeenCalled(); expect(state.loading).toBe(false); expect(mocks.error).toHaveBeenCalled()
  })
  it('does not restart with unsaved settings', async () => {
    const state = mount(Settings); await flush(); state.setData(clone(state.settings)); state.settings.webPort = '8888'; await state.restartApp(); expect(mocks.post).not.toHaveBeenCalled()
  })
  it('uses the last saved settings while edits happen during a restart request, then cancels the redirect on unmount', async () => {
    const state = mount(Settings); await flush(); state.setData(clone(state.settings)); const reply = deferred(); mocks.post.mockReturnValue(reply.promise)
    vi.useFakeTimers(); const pending = state.restartApp(); state.settings.webURI = 'javascript:alert(1)'; reply.resolve({ success: true }); await flush()
    apps.pop()!.unmount(); await pending; expect(state.loading).toBe(false); expect(vi.getTimerCount()).toBe(0)
  })
  it('ignores an older settings response and recovers from thrown saves', async () => {
    const old = deferred(), latest = deferred(); mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const state = mount(Settings), pending = state.loadData(); const data = clone(state.settings)
    latest.resolve({ success: true, obj: { ...data, webDomain: 'new.test' } }); await pending
    old.resolve({ success: true, obj: { ...data, webDomain: 'old.test' } }); await flush(); expect(state.settings.webDomain).toBe('new.test')
    mocks.post.mockRejectedValue(new Error('mock offline')); await state.save(); expect(state.loading).toBe(false)
  })
})


describe('round6 stale rule drag protection', () => {
  it.each([Dns, Rules])('does not insert an undefined rule after the dragged rule was removed: %s', component => {
    mocks.store.config.dns.rules = [{ action: 'route', server: 'a' }, { action: 'route', server: 'b' }]
    mocks.store.config.route.rules = [{ action: 'route', outbound: 'a' }, { action: 'route', outbound: 'b' }]
    const state = mount(component), list = component === Dns ? state.dnsRules : state.rules
    state.onDragStart(1); list.splice(1, 1); state.onDrop(0)
    expect(list).toHaveLength(1); expect(list[0]).toBeDefined()
  })
  it.each([Dns, Rules])('does not apply canceled drags to later unrelated drops: %s', component => {
    const state = mount(component)
    const list = component === Dns ? state.dnsRules : state.rules
    list.push({ action: 'first' }, { action: 'second' })
    state.onDragStart(0); state.onDragEnd(); state.onDrop(1)
    expect(list.map((rule: any) => rule.action)).toEqual(['first', 'second'])
  })
})


describe('round6 explicit defaults and TLS preservation', () => {
  it.each([['1h30m', 90], ['30s', 0.5], ['500ms', 1 / 120], ['+2m', 2]])('displays NTP interval %s in minutes', (interval, minutes) => {
    mocks.store.config.ntp = { enabled: true, server: 'ntp.example.test', interval }
    expect(mount(Basics).ntpInterval).toBe(minutes)
  })
  it('saves finite NTP minutes and clears invalid/empty inputs without producing invalid duration strings', () => {
    mocks.store.config.ntp = { enabled: true, server: 'ntp.example.test', interval: '30m' }
    const state = mount(Basics); state.ntpInterval = 0.5; expect(state.appConfig.ntp.interval).toBe('30s')
    for (const input of [null, '', 'invalid', Infinity, -1]) { state.ntpInterval = input; expect(state.appConfig.ntp.interval).toBeUndefined() }
  })
  it('does not assign the first outbound as a default and omits a cleared selection from saved JSON', async () => {
    delete mocks.store.config.route.final; mocks.store.outbounds = [{ tag: 'direct' }, { tag: 'proxy' }]
    const state = mount(Rules); expect(state.finalOutbound).toBe(''); expect(state.route.final).toBeUndefined()
    state.finalOutbound = 'proxy'; await state.saveConfig(); expect(mocks.save.mock.calls[0][2].route.final).toBe('proxy')
    state.finalOutbound = null; expect(Object.hasOwn(state.route, 'final')).toBe(false)
  })
  it('keeps the first-DNS-server sentinel as an omitted final field', () => {
    const state = mount(Dns); expect(state.finalDns).toBe(''); state.finalDns = 'mock-resolver'; expect(state.dns.final).toBe('mock-resolver')
    state.finalDns = null; expect(JSON.stringify(state.dns)).not.toContain('"final"')
  })
  it('clones full inbound TLS references without mutating the original object', async () => {
    const source = { id: 5, type: 'vless', tag: 'source', listen_port: 443, tls_id: 7, transport: { type: 'ws', path: '/mock' } }
    mocks.loadInbounds.mockResolvedValue([source]); await mount(Inbounds).clone(5)
    const saved = mocks.save.mock.calls[0][2]; expect(saved.tls_id).toBe(7); expect(saved.transport).toEqual(source.transport); expect(source.id).toBe(5); expect(saved.id).toBe(0)
  })
  it('clones independent TLS client/server options and chooses a unique profile name', async () => {
    const source = { id: 1, name: 'profile', server: { enabled: true, reality: { enabled: true, private_key: 'mock-private' } }, client: { reality: { enabled: true, public_key: 'mock-public' } } }
    mocks.store.tlsConfigs = [source, { id: 2, name: 'profile-copy' }]
    const state = mount(Tls); state.clone(source); await flush()
    const saved = mocks.save.mock.calls[0][2]; expect(saved.name).toBe('profile-copy-copy'); expect(saved.id).toBe(0); expect(saved.server).toEqual(source.server)
    saved.server.reality.private_key = 'changed'; expect(source.server.reality.private_key).toBe('mock-private')
  })
})

// Template/setup smoke only: presentation components remain lightweight stubs.
describe('round7 complete page template smoke', () => {
  it.each([['Admins', Admins], ['Basics', Basics], ['Clients', Clients], ['Dns', Dns], ['Endpoints', Endpoints], ['Home', Home], ['Inbounds', Inbounds], ['Login', Login], ['Outbounds', Outbounds], ['Rules', Rules], ['Services', Services], ['Settings', Settings], ['Tls', Tls]])('compiles and mounts %s with mocked boundaries', (name, component) => {
    expect(() => renderView(component, './' + name + '.vue')).not.toThrow()
  })
})
