import { createApp, reactive, nextTick, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), store: {} as any }))
vi.mock('@/plugins/httputil', () => ({ default: { get: mocks.get, post: mocks.post } }))
vi.mock('@/store/modules/data', () => ({ default: () => mocks.store }))
vi.mock('notivue', () => ({ push: { error: vi.fn() } }))
vi.mock('@/locales', () => ({ i18n: { global: { t: (key: string) => key } } }))
vi.mock('@/layouts/modals/Logs.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Backup.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/UsageStats.vue', () => ({ default: {} }))
import Main from './Main.vue'
import SimpleDNS from './SimpleDNS.vue'
import Naive from './protocols/Naive.vue'
import OutShadowTls from './protocols/OutShadowTls.vue'
import Selector from './protocols/Selector.vue'
import Shadowsocks from './protocols/Shadowsocks.vue'
import Tuic from './protocols/Tuic.vue'
import UrlTest from './protocols/UrlTest.vue'
import Ccm from './services/Ccm.vue'
import Derp from './services/Derp.vue'
import Ocm from './services/Ocm.vue'
import SSMAPI from './services/SSMAPI.vue'
import Gauge from './tiles/Gauge.vue'
import History from './tiles/History.vue'
import HttpUpgrade from './transports/HttpUpgrade.vue'
import GRPC from './transports/gRPC.vue'
import { createOutbound } from '@/types/outbounds'
import { createInbound } from '@/types/inbounds'
import { createSrv } from '@/types/services'

function stateFor(component: any, props: any = {}) {
  const state: any = reactive({ $props: props, ...props })
  Object.assign(state, component.data?.call(state) ?? {})
  for (const [name, method] of Object.entries(component.methods ?? {})) state[name] = (method as Function).bind(state)
  for (const [name, option] of Object.entries(component.computed ?? {}) as any[]) {
    Object.defineProperty(state, name, {
      configurable: true,
      get: () => (typeof option === 'function' ? option : option.get).call(state),
      set: option.set ? (value: any) => option.set.call(state, value) : undefined,
    })
  }
  return state
}
let app: App | undefined
function mountSetup(component: any, props = {}) {
  app = createApp({ ...component, render: () => null }, props)
  return (app.mount(document.createElement('div')).$ as any).setupState
}
function deferred() {
  let resolve!: (value: any) => void
  let reject!: (error: Error) => void
  const promise = new Promise((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
const flush = async () => { for (let i = 0; i < 8; i++) await Promise.resolve(); await nextTick() }
beforeEach(() => {
  mocks.store = { onlines: {}, inbounds: [], outbounds: [], clients: [] }
  mocks.get.mockResolvedValue({ success: true, obj: {} })
  mocks.post.mockResolvedValue({ success: true })
})
afterEach(() => { app?.unmount(); app = undefined; vi.useRealTimers(); vi.resetAllMocks() })

describe('round4 dashboard request lifecycle', () => {
  it('does not start a timer or apply results after unmount during initial load', async () => {
    vi.useFakeTimers()
    const status = deferred(), logs = deferred()
    mocks.get.mockImplementation((url: string) => url === 'api/status' ? status.promise : logs.promise)
    const state = mountSetup(Main)
    app!.unmount(); app = undefined
    status.resolve({ success: true, obj: { cpu: 51 } }); logs.resolve({ success: true, obj: ['late'] })
    await flush()
    expect(vi.getTimerCount()).toBe(0)
    expect(state.tilesData).toEqual({})
    expect(state.recentLogs).toEqual([])
  })
  it('keeps the latest status and logs when requests finish out of order', async () => {
    vi.useFakeTimers()
    const state = mountSetup(Main); await flush()
    const old = deferred(), latest = deferred()
    mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const first = state.reloadData(), second = state.reloadData()
    latest.resolve({ success: true, obj: { cpu: 12 } }); await second
    old.resolve({ success: true, obj: { cpu: 98 } }); await first
    expect(state.tilesData.cpu).toBe(12)
    const log1 = deferred(), log2 = deferred()
    mocks.get.mockReturnValueOnce(log1.promise).mockReturnValueOnce(log2.promise)
    const a = state.loadRecentLogs(), b = state.loadRecentLogs()
    log1.resolve({ success: true, obj: ['old'] }); await a
    expect(state.recentLogsLoading).toBe(true)
    log2.resolve({ success: true, obj: ['new'] }); await b
    expect(state.recentLogs).toEqual(['new'])
    expect(state.recentLogsLoading).toBe(false)
  })
  it('finishes rejected initial requests and prevents duplicate restart submissions', async () => {
    vi.useFakeTimers()
    mocks.get.mockRejectedValue(new Error('offline'))
    const state = mountSetup(Main); await flush()
    expect(state.loading).toBe(false)
    expect(state.recentLogsLoading).toBe(false)
    expect(vi.getTimerCount()).toBe(1)
    const post = deferred(); mocks.post.mockReturnValueOnce(post.promise)
    const a = state.restartSingbox(), b = state.restartSingbox()
    expect(mocks.post).toHaveBeenCalledTimes(1)
    post.reject(new Error('offline')); await Promise.all([a, b])
    expect(state.loading).toBe(false)
  })
  it('handles empty online state, malformed responses and bounded percentages', async () => {
    vi.useFakeTimers(); mocks.store.onlines = null
    const state = mountSetup(Main); await flush()
    expect(state.onlineUsersCount).toBe(0)
    expect(state.onlineInboundCount).toBe(0)
    expect(state.onlineOutboundCount).toBe(0)
    mocks.get.mockResolvedValueOnce({ success: true, obj: ['invalid'] })
    await state.reloadData(); expect(state.tilesData).toEqual({})
    mocks.get.mockResolvedValueOnce({ success: true, obj: 'invalid' })
    await state.loadRecentLogs(); expect(state.recentLogs).toEqual([])
    state.tilesData = { cpu: Infinity, mem: { current: -1, total: 10 }, dsk: { current: 15, total: 10 } }
    expect(state.resourceRows.map((r: any) => r.percent)).toEqual([0, 0, 100])
  })
})

describe('round4 protocol form invariants', () => {
  it('updates DNS type atomically and ignores cleared type selections', () => {
    const state = stateFor(SimpleDNS, { data: { type: 'udp', server: '1.1.1.1', server_port: 53 } })
    state.updateType('local')
    expect(state.data).toEqual({ type: 'local' })
    state.updateType('tls'); expect(state.data.type).toBe('tls')
    state.updateType(null); expect(state.data.type).toBe('tls')
  })
  it('matches the supported Naive inbound algorithms and retains outbound BBR2', () => {
    const state = stateFor(Naive, { data: createInbound('naive'), direction: 'in' })
    expect(state.inbCngs.map((x: any) => x.value)).toEqual(['bbr', 'cubic', 'reno'])
    expect(state.outCngs.map((x: any) => x.value)).toContain('bbr2')
    expect(state.udpOverTcp).toBe(false)
  })
  it('normalizes Naive concurrency and preserves UDP-over-TCP options', () => {
    const state = stateFor(Naive, { data: createOutbound('naive'), direction: 'out' })
    for (const v of [null, undefined, '', -2, 1.5, Infinity, 'invalid']) {
      state.insecure_concurrency = v
      expect(state.data.insecure_concurrency).toBeUndefined()
    }
    state.insecure_concurrency = '3'; expect(state.data.insecure_concurrency).toBe(3)
    state.data.udp_over_tcp = { enabled: true, version: 1 }; state.udpOverTcp = true
    expect(state.data.udp_over_tcp).toEqual({ enabled: true, version: 1 })
  })
  it('uses ShadowTLS v1 when omitted, v3 for new outbounds, and guards invalid versions', () => {
    const omitted = stateFor(OutShadowTls, { data: {} })
    expect(omitted.version).toBe(1); expect(omitted.data.version).toBeUndefined()
    const state = stateFor(OutShadowTls, { data: createOutbound('shadowtls') })
    expect(state.version).toBe(3)
    state.version = null; expect(state.version).toBe(3)
    state.version = 2; state.password = 'secret'; expect(state.data.password).toBe('secret')
    state.version = 1; state.password = 'late'; expect(state.data.password).toBeUndefined()
  })
  it('validates Selector defaults against the incoming list rather than stale model state', () => {
    const state = stateFor(Selector, { data: { outbounds: ['a'], default: 'a' } })
    state.updateDefault(['b']); expect(state.data.outbounds).toEqual(['b'])
    expect(state.data.default).toBeUndefined()
    state.defaultOutbound = 'b'; expect(state.data.default).toBe('b')
    state.defaultOutbound = 'unknown'; expect(state.data.default).toBe('b')
    state.defaultOutbound = null; expect(state.data.default).toBeUndefined()
    state.updateDefault(null); expect(state.data.outbounds).toEqual([])
  })
  it('ignores absent/unsupported Shadowsocks cipher events without changing credentials', () => {
    const state = stateFor(Shadowsocks, { data: { password: 'keep' }, direction: 'in' })
    for (const value of [null, undefined, 'unsupported']) expect(() => state.changeMethod(value)).not.toThrow()
    expect(state.data.password).toBe('keep')
    state.changeMethod('2022-blake3-aes-128-gcm'); expect(atob(state.data.password)).toHaveLength(16)
    state.changeMethod('2022-blake3-aes-256-gcm'); expect(atob(state.data.password)).toHaveLength(32)
    state.changeMethod('none'); expect(state.data.password).toBeUndefined()
  })
  it.each([[Tuic, 'auth_timeout'], [Tuic, 'heartbeat'], [GRPC, 'idle_timeout'], [GRPC, 'ping_timeout']])('round-trips duration units and clears invalid values for %s/%s', (component: any, field: any) => {
    const data: any = { [field]: '1m30s' }
    const state = stateFor(component, { data, transport: data })
    expect(state[field]).toBe(90)
    state[field] = 1.5; expect(data[field]).toBe('1.5s')
    for (const v of [null, undefined, '', -1, Infinity, 'bad']) {
      state[field] = v; expect(data[field]).toBeUndefined()
    }
    data[field] = '500ms'; expect(state[field]).toBe(0.5)
  })
  it('does not recreate disabled URLTest options on delayed field events', () => {
    const state = stateFor(UrlTest, { data: createOutbound('urltest') })
    for (const [toggle, field] of [['optionInterval', 'interval'], ['optionTolerance', 'tolerance'], ['optionIdle', 'idle_timeout']]) {
      state[toggle] = true; state[toggle] = false; state[field] = null
      expect(state.data[field]).toBeUndefined()
    }
    state.optionInterval = true; state.data.interval = '3m'; expect(state.interval).toBe(180)
    state.optionIdle = true; state.data.idle_timeout = '1h'; expect(state.idle_timeout).toBe(60)
    state.interval = Infinity; expect(state.data.interval).toBe('3m')
  })
})

describe('round4 service forms', () => {
  it.each([Ccm, Ocm])('supports missing tags/users and ignores invalid user indices', component => {
    const state = stateFor(component, { data: createSrv(component === Ccm ? 'ccm' : 'ocm') })
    expect(state.outTags).toEqual([])
    state.data.users = null; state.addUser(); state.addUser()
    state.data.users[0].name = 'first'
    for (const index of [-1, '', 'bad', 0.5, 8]) state.delUser(index)
    expect(state.data.users).toHaveLength(2)
    state.delUser('1'); expect(state.data.users).toEqual([{ name: 'first', token: '' }])
  })
  it('synchronizes DERP PSK mode after enabling mesh and ignores late list events', () => {
    const state = stateFor(Derp, { data: createSrv('derp') })
    expect(state.usePskText).toBe(1)
    state.optionMesh = true; expect(state.usePskText).toBe(0)
    state.usePskText = 1; expect(state.data.mesh_psk).toBeUndefined()
    expect(state.data.mesh_psk_file).toBe('')
    state.optionMesh = false
    expect(() => { state.addMesh(); state.delMesh(0); state.usePskText = 0 }).not.toThrow()
    expect(state.data.mesh_with).toBeUndefined(); expect(state.data.mesh_psk).toBeUndefined()
    state.optionVerifyCU = true; state.optionVerifyCU = false
    expect(() => { state.addClientUrl(); state.delClientUrl(0) }).not.toThrow()
    expect(state.data.verify_client_url).toBeUndefined()
  })
  it('keeps DERP repeated enabling idempotent', () => {
    const state = stateFor(Derp, { data: { mesh_with: [{ server: 'keep' }], mesh_psk_file: 'keep.key' } })
    state.optionMesh = true
    expect(state.data).toEqual({ mesh_with: [{ server: 'keep' }], mesh_psk_file: 'keep.key' })
    expect(state.usePskText).toBe(1)
  })
  it('adds SSM paths without collisions after deletions, even without tags', () => {
    const state = stateFor(SSMAPI, { data: { servers: { '/ss1': 'a' } } })
    state.add_server()
    expect(state.servers).toHaveLength(2)
    expect(state.servers.map((x: any) => x.name)).toEqual(['/ss1', '/ss0'])
    expect(state.data.servers['/ss1']).toBe('a')
  })
  it('rejects duplicate SSM paths, handles special keys and ignores stale indices', () => {
    const state = stateFor(SSMAPI, { data: { servers: { '/a': 'x', '/b': 'y' } }, ssTags: [] })
    state.update_key(1, '/a'); expect(state.data.servers).toEqual({ '/a': 'x', '/b': 'y' })
    state.update_key(1, '__proto__')
    expect(Object.hasOwn(state.data.servers, '__proto__')).toBe(true)
    expect(state.data.servers.__proto__).toBe('y')
    expect(() => { state.update_key(99, 'late'); state.update_value(99, null); state.del_server(-1) }).not.toThrow()
    expect(state.servers).toHaveLength(2)
    state.servers = null; expect(state.data.servers).toBeUndefined()
  })
})

describe('round4 metric tiles', () => {
  it.each([
    ['g-cpu', { cpu: 0 }, 0, '0%'],
    ['g-dsk', { dsk: { current: 1, total: 4 } }, 25, undefined],
    ['g-swp', { swp: { current: 1, total: 2 } }, 50, undefined],
    ['g-cpu', { cpu: Infinity }, 0, '-'],
    ['g-mem', { mem: { current: 20, total: 10 } }, 100, undefined],
  ])('reads %s independently with bounded finite percentages', (type, tilesData, percent, text) => {
    const state = mountSetup(Gauge, { tilesData, type })
    expect(state.data.percent).toBe(percent)
    if (text) expect(state.data.text).toBe(text)
    expect(parseFloat(state.cssTransformRotateValue)).toBeGreaterThanOrEqual(0)
    expect(parseFloat(state.cssTransformRotateValue)).toBeLessThanOrEqual(0.5)
  })
  it('keeps the first CPU sample and a rolling 20-sample window', () => {
    const state = stateFor(History, { type: 'h-cpu' })
    state.updateData1(1); expect(state.data.datasets[0].data).toEqual([1])
    for (let i = 2; i <= 25; i++) state.updateData1(i)
    expect(state.data.datasets[0].data).toEqual(Array.from({ length: 20 }, (_, i) => i + 6))
  })
  it.each(['h-cpu', 'h-mem', 'h-net', 'hp-net', 'h-dio'])('ignores incomplete %s samples safely', type => {
    const state = stateFor(History, { type })
    const update = (History as any).watch.tilesData.handler.bind(state)
    for (const value of [null, undefined, {}, { mem: { current: 2, total: 0 } }]) expect(() => update(value)).not.toThrow()
    expect(state.loaded).toBe(false)
  })
  it.each([
    ['h-net', 'net', 'sent', 'recv'], ['hp-net', 'net', 'psent', 'precv'], ['h-dio', 'dio', 'write', 'read'],
  ])('takes zero baselines and ignores counter resets for %s', (type, key, up, down) => {
    const state = stateFor(History, { type })
    const update = (History as any).watch.tilesData.handler.bind(state)
    update({ [key]: { [up]: 0, [down]: 0 } })
    update({ [key]: { [up]: 10, [down]: 20 } })
    expect(state.data.datasets[0].data).toEqual([5])
    expect(state.data.datasets[1].data).toEqual([10])
    update({ [key]: { [up]: 1, [down]: 1 } })
    expect(state.data.datasets[0].data).toEqual([5, 0])
    expect(state.data.datasets[1].data).toEqual([10, 0])
  })
  it('retains HTTPUpgrade scalar host/path and registers the header editor', () => {
    const transport = { type: 'httpupgrade', host: 'example.com', path: '/ws', headers: { Custom: 'value' } }
    const state = stateFor(HttpUpgrade, { transport })
    expect(state.transport).toEqual(transport)
    expect((HttpUpgrade as any).components.Headers).toBeDefined()
  })
})

describe('round4 additional timing and imported-config boundaries', () => {
  it('ignores pre-restart status and does not refresh after restart finishes on an unmounted page', async () => {
    vi.useFakeTimers()
    const state = mountSetup(Main); await flush()
    const old = deferred(), post = deferred()
    mocks.get.mockReturnValueOnce(old.promise)
    const status = state.reloadData()
    mocks.post.mockReturnValueOnce(post.promise)
    const restart = state.restartSingbox()
    old.resolve({ success: true, obj: { cpu: 88 } }); await status
    expect(state.tilesData.cpu).toBeUndefined()
    app!.unmount(); app = undefined
    const calls = mocks.get.mock.calls.length
    post.resolve({ success: true }); await restart
    expect(mocks.get).toHaveBeenCalledTimes(calls)
    expect(vi.getTimerCount()).toBe(0)
  })
  it.each([[Tuic, 'heartbeat'], [GRPC, 'idle_timeout']])('displays day/explicit-plus durations and serializes tiny values without exponent notation', (component: any, field: any) => {
    const data: any = { [field]: '+1d2h3m4.5s' }
    const state = stateFor(component, { data, transport: data })
    expect(state[field]).toBe(93784.5)
    state[field] = 1e-7; expect(data[field]).toBe('0.0000001s')
    state[field] = 1e20; expect(data[field]).toBeUndefined()
  })
  it('keeps URLTest custom options on repeated enable and ignores late URL updates', () => {
    const state = stateFor(UrlTest, { data: { url: 'https://custom', interval: '5m', idle_timeout: '1h', tolerance: 25 } })
    state.optionUrl = true; state.optionInterval = true; state.optionIdle = true; state.optionTolerance = true
    expect(state.data).toEqual({ url: 'https://custom', interval: '5m', idle_timeout: '1h', tolerance: 25 })
    state.optionUrl = false; state.url = 'late'
    expect(state.data.url).toBeUndefined()
    for (const v of [65536, Infinity, 0.5, -1]) { state.tolerance = v; expect(state.data.tolerance).toBe(25) }
    state.tolerance = 0; expect(state.data.tolerance).toBe(0)
  })
  it('normalizes legal DERP listable fields and numeric STUN without losing configuration', () => {
    const state = stateFor(Derp, { data: { verify_client_endpoint: 'ts', verify_client_url: ['https://a', { url: 'https://b', detour: 'proxy' }], mesh_with: { server: 'peer', tls: { enabled: true } }, stun: 3479 } })
    ;(Derp as any).watch.data.handler.call(state)
    expect(state.data.verify_client_endpoint).toEqual(['ts'])
    expect(state.data.verify_client_url).toEqual([{ url: 'https://a' }, { url: 'https://b', detour: 'proxy' }])
    expect(state.data.mesh_with).toEqual([{ server: 'peer', tls: { enabled: true } }])
    expect(state.data.stun).toEqual({ enabled: true, listen_port: 3479 })
    state.addClientUrl(); state.addMesh()
    expect(state.data.verify_client_url).toHaveLength(3)
    expect(state.data.mesh_with).toHaveLength(2)
  })
  it('reflects DERP STUN enabled state and preserves existing port when enabling', () => {
    const state = stateFor(Derp, { data: { stun: { enabled: false, listen_port: 3479 } } })
    expect(state.optionStun).toBe(false)
    state.optionStun = true; expect(state.data.stun).toEqual({ enabled: true, listen_port: 3479 })
    state.optionStun = true; expect(state.data.stun.listen_port).toBe(3479)
  })
  it('does not revive DERP PSK/home/endpoint fields after disabling or changing modes', () => {
    const state = stateFor(Derp, { data: {} })
    state.optionMesh = true; state.usePskText = 1; state.meshPsk = 'late'
    expect(state.data.mesh_psk).toBeUndefined()
    state.meshPskFile = 'key.txt'; expect(state.data.mesh_psk_file).toBe('key.txt')
    state.optionMesh = false; state.meshPskFile = 'late'
    expect(state.data.mesh_psk_file).toBeUndefined()
    state.optionHome = true; state.optionHome = false; state.home = 'late'
    expect(state.data.home).toBeUndefined()
    state.optionVerifyCE = true; state.optionVerifyCE = false; state.verifyClientEndpoint = ['late']
    expect(state.data.verify_client_endpoint).toBeUndefined()
  })
  it('does not mutate a cached SSM row after a rejected rename in a mounted Options component', async () => {
    const props = { data: { servers: { '/a': 'x', '/b': 'y' } }, ssTags: [] }
    app = createApp({ ...SSMAPI, render: () => null }, props)
    const vm = app.mount(document.createElement('div')) as any
    expect(vm.servers[1].name).toBe('/b')
    vm.update_key(1, '/a'); await nextTick()
    expect(vm.servers[1].name).toBe('/b')
  })
  it('uses immutable counter baselines and resets datasets on metric type changes', () => {
    const state = stateFor(History, { type: 'h-net' })
    const update = (History as any).watch.tilesData.handler.bind(state)
    const value = { net: { sent: 2, recv: 4 } }; update(value)
    value.net.sent = 12; value.net.recv = 24; update(value)
    expect(state.data.datasets[0].data).toEqual([5])
    expect(state.data.datasets[1].data).toEqual([10])
    ;(History as any).watch.type.call(state)
    expect(state.loaded).toBe(false); expect(state.data).toEqual({})
  })
})

// Additional checks use real Vue computed caching/watch hooks where relevant.
describe('round4 final regressions', () => {
  it('normalizes DERP imported scalar fields through the real immediate watcher', () => {
    const props = { data: { verify_client_url: 'https://verify', mesh_with: { server: 'peer' }, stun: 0 } }
    app = createApp({ ...Derp, render: () => null }, props)
    const vm = app.mount(document.createElement('div')) as any
    expect(vm.data.verify_client_url).toEqual([{ url: 'https://verify' }])
    expect(vm.data.mesh_with).toEqual([{ server: 'peer' }])
    expect(vm.data.stun).toEqual({ enabled: true, listen_port: 0 })
    expect(vm.optionStun).toBe(true)
  })
  it('lets legacy array-valued SSM mappings be repaired into valid string maps', () => {
    const state = stateFor(SSMAPI, { data: { servers: { '/ss': ['a', 'b'] } }, ssTags: ['a', 'b'] })
    state.update_key(1, '/other')
    expect(state.data.servers).toEqual({ '/ss': 'a', '/other': 'b' })
    state.update_value(0, null)
    expect(state.data.servers).toEqual({ '/ss': '', '/other': 'b' })
  })
  it('shows URLTest zero/default values exactly as the core resolves them', () => {
    const state = stateFor(UrlTest, { data: { interval: '0s', idle_timeout: '0m', tolerance: 0 } })
    expect(state.interval).toBe(180)
    expect(state.idle_timeout).toBe(30)
    expect(state.tolerance).toBe(50)
  })
  it('ignores missing transport/data on delayed duration setters', () => {
    const tuic = stateFor(Tuic, { data: null })
    const grpc = stateFor(GRPC, { transport: null })
    expect(() => { tuic.heartbeat = null; tuic.auth_timeout = 5; grpc.idle_timeout = null; grpc.ping_timeout = 5 }).not.toThrow()
    expect(tuic.data).toBeNull(); expect(grpc.transport).toBeNull()
  })
  it('consumes the initially supplied history CPU sample using the real immediate watcher', () => {
    app = createApp({ ...History, render: () => null }, { tilesData: { cpu: 0 }, type: 'h-cpu' })
    const vm = app.mount(document.createElement('div')) as any
    expect(vm.loaded).toBe(true)
    expect(vm.data.datasets[0].data).toEqual([0])
  })
})
