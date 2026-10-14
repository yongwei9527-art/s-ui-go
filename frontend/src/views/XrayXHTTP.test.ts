import { createApp, type App } from 'vue'
import { createVuetify } from 'vuetify'
import { afterEach, describe, expect, it, vi } from 'vitest'
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), error: vi.fn() }))
vi.mock('@/plugins/httputil', () => ({ default: mocks }))
vi.mock('@/store/modules/data', () => ({ default: () => ({ clients: [] }) }))
vi.mock('notivue', () => ({ push: { error: mocks.error } }))
import XrayXHTTP from './XrayXHTTP.vue'
let app: App | undefined
const settle = async () => { for (let i = 0; i < 10; i++) await Promise.resolve() }
const mount = async () => {
  mocks.get.mockResolvedValue({ success: true, obj: {} })
  mocks.post.mockResolvedValue({ success: true, obj: {} })
  vi.spyOn(console, 'warn').mockImplementation(() => {})
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  vi.stubGlobal('IntersectionObserver', class { observe() {} unobserve() {} disconnect() {} })
  window.matchMedia = vi.fn().mockReturnValue({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() })
  app = createApp(XrayXHTTP).use(createVuetify())
  const vm = app.mount(document.createElement('div'))
  await settle()
  return (vm.$ as any).setupState
}
afterEach(() => { app?.unmount(); app = undefined; vi.restoreAllMocks(); vi.clearAllMocks(); vi.unstubAllGlobals() })
describe('Xray form lifecycle', () => {
  it('does not partially replace an existing Reality key pair', async () => {
    const state = await mount()
    state.edit({ ...state.form, id: 7, publicKey: 'existing-public', shortId: '0123456789abcdef' })
    mocks.get.mockResolvedValue({ success: true, obj: ['PrivateKey: new-private'] })
    await state.generateRealityCredentials()
    expect(state.form.privateKey).toBe('')
    expect(state.form.publicKey).toBe('existing-public')
    expect(state.form.shortId).toBe('0123456789abcdef')
    expect(state.loading).toBe(false)
    expect(mocks.error).toHaveBeenCalled()
  })
  it('removes the edited id when starting a new node', async () => {
    const state = await mount()
    state.edit({ ...state.form, id: 7 })
    expect(state.editing).toBe(true)
    state.newInbound()
    expect(state.form.id).toBeUndefined()
    expect(state.editing).toBe(false)
  })
  it('does not switch to an existing node while new credentials are pending', async () => {
    const state = await mount()
    let resolve!: (value: unknown) => void
    mocks.get.mockImplementation(() => new Promise(r => { resolve = r }))
    const saving = state.save()
    await settle()
    state.edit({ ...state.form, id: 7 })
    state.newInbound()
    await state.load()
    expect(state.form.id).toBeUndefined()
    expect(state.loading).toBe(true)
    resolve({ success: true, obj: ['PrivateKey: private', 'PublicKey: public'] })
    await saving
    expect(JSON.parse(mocks.post.mock.calls[0][1].data).inbound.id).toBeUndefined()
    expect(state.form.id).toBeUndefined()
    expect(state.loading).toBe(false)
  })
})
