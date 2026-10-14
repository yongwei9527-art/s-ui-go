import { reactive } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), loadClients: vi.fn(), error: vi.fn() }))
vi.mock('@/plugins/httputil', () => ({ default: { get: mocks.get } }))
vi.mock('@/store/modules/data', () => ({ default: () => ({ loadClients: mocks.loadClients, subURI: 'https://example.test/sub/' }) }))
vi.mock('@/locales', () => ({ i18n: { global: { t: (key: string) => key, locale: { value: 'en' } } } }))
vi.mock('notivue', () => ({ push: { error: mocks.error, success: vi.fn() } }))
import Changes from './Changes.vue'
import QrCode from './QrCode.vue'
import WgQrCode from './WgQrCode.vue'

function stateFor(component: any, props: any = {}) {
  const state: any = reactive({ $props: props, ...props, $emit: vi.fn() })
  Object.assign(state, component.data?.call(state) ?? {})
  for (const [name, method] of Object.entries(component.methods ?? {})) {
    state[name] = (method as Function).bind(state)
  }
  for (const [name, option] of Object.entries(component.computed ?? {}) as any[]) {
    Object.defineProperty(state, name, {
      get: () => (typeof option === 'function' ? option : option.get).call(state),
    })
  }
  return state
}

function deferred() {
  let resolve!: (value: any) => void
  const promise = new Promise<any>(r => { resolve = r })
  return { promise, resolve }
}

afterEach(() => { vi.resetAllMocks() })

describe('changes modal async requests', () => {
  it('releases loading when the server rejects a request', async () => {
    mocks.get.mockResolvedValue({ success: false })
    const state = stateFor(Changes, { visible: true })
    await state.loadData()
    expect(state.loading).toBe(false)
  })
  it('does not overwrite a newer filter result with an older response', async () => {
    const old = deferred(), latest = deferred()
    mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const state = stateFor(Changes, { visible: true })
    const first = state.loadData()
    state.key = 'tls'
    const second = state.loadData()
    latest.resolve({ success: true, obj: [{ id: 2 }] })
    await second
    old.resolve({ success: true, obj: [{ id: 1 }] })
    await first
    expect(state.changes).toEqual([{ id: 2 }])
    expect(state.loading).toBe(false)
  })
  it('ignores a response after the dialog closes', async () => {
    const response = deferred()
    mocks.get.mockReturnValue(response.promise)
    const state = stateFor(Changes, { visible: true })
    const pending = state.loadData()
    state.$props.visible = false
    ;(Changes as any).watch.visible.call(state, false)
    response.resolve({ success: true, obj: [{ id: 1 }] })
    await pending
    expect(state.changes).toEqual([])
    expect(state.loading).toBe(false)
  })
  it('clears loading and reports an unexpected request exception', async () => {
    mocks.get.mockRejectedValue(new Error('failed request'))
    const state = stateFor(Changes, { visible: true })
    await state.loadData()
    expect(state.loading).toBe(false)
    expect(mocks.error).toHaveBeenCalled()
  })
})

describe('client QR modal request identity', () => {
  it('does not show the previous client while a new client is loading', async () => {
    const response = deferred()
    mocks.loadClients.mockReturnValue(response.promise)
    const state = stateFor(QrCode, { visible: true, id: 2 })
    state.client = { name: 'previous-secret' }
    const pending = state.load()
    expect(state.clientSub).toBe('')
    response.resolve({ name: 'current', links: [] })
    await pending
    expect(state.clientSub).toBe('https://example.test/sub/current')
  })
  it('keeps the newest client when an older request completes last', async () => {
    const old = deferred(), latest = deferred()
    mocks.loadClients.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const state = stateFor(QrCode, { visible: true, id: 1 })
    const first = state.load()
    state.$props.id = 2
    const second = state.load()
    latest.resolve({ name: 'second', links: [{ uri: 'new' }] })
    await second
    old.resolve({ name: 'first', links: [{ uri: 'old' }] })
    await first
    expect(state.client.name).toBe('second')
    expect(state.clientLinks).toEqual([{ uri: 'new' }])
  })
  it('does not publish subscription links for a failed or closed request', async () => {
    mocks.loadClients.mockResolvedValue({})
    const state = stateFor(QrCode, { visible: true, id: 1 })
    await state.load()
    expect(state.clientSub).toBe('')
    expect(state.singbox).toBe('')
    const response = deferred()
    mocks.loadClients.mockReturnValue(response.promise)
    const pending = state.load()
    state.$props.visible = false
    ;(QrCode as any).watch.visible.call(state, false)
    response.resolve({ name: 'closed-secret' })
    await pending
    expect(state.clientSub).toBe('')
    expect(state.loading).toBe(false)
  })
  it('recovers from a load exception without keeping old credentials', async () => {
    mocks.loadClients.mockRejectedValue(new Error('failed request'))
    const state = stateFor(QrCode, { visible: true, id: 1 })
    state.client = { name: 'old-secret' }
    await state.load()
    expect(state.clientSub).toBe('')
    expect(state.loading).toBe(false)
    expect(mocks.error).toHaveBeenCalled()
  })
})

describe('WireGuard QR export', () => {
  it('accepts an incomplete editor without crashing', async () => {
    const state = stateFor(WgQrCode, { data: {}, visible: true })
    await state.load()
    expect(state.wgLinks).toEqual([])
    expect(state.getWireguardLink(0, 'example.test')).toBe('')
  })
  it.each(['2001:db8::1', '[2001:db8::1]'])('brackets IPv6 endpoint %s once', address => {
    const state = stateFor(WgQrCode)
    state.wgData = {
      peers: [{ public_key: 'client-public', allowed_ips: ['10.0.0.2/32'] }],
      ext: { public_key: 'server-public', keys: [{ public_key: 'client-public', private_key: 'client-private' }] },
      listen_port: 51820, tag: 'wg',
    }
    expect(state.getWireguardLink(0, address)).toContain('Endpoint = [2001:db8::1]:51820')
    expect(state.getWireguardLink(0, '192.0.2.1')).toContain('Endpoint = 192.0.2.1:51820')
  })
})
