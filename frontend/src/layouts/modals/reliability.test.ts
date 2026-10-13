import { afterEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), loadInbounds: vi.fn() }))
vi.mock('@/plugins/httputil', () => ({ default: { get: mocks.get } }))
vi.mock('@/store/modules/data', () => ({ default: () => ({ loadInbounds: mocks.loadInbounds }) }))
vi.mock('@/locales', () => ({ i18n: { global: { t: (key: string) => key, locale: 'en' } } }))
vi.mock('notivue', () => ({ push: { error: vi.fn(), success: vi.fn() } }))
import Logs from './Logs.vue'
import Inbound from './Inbound.vue'
import Stats from './Stats.vue'

afterEach(() => { vi.useRealTimers(); vi.resetAllMocks() })
describe('modal loading reliability', () => {
  it('resets logs loading after a failed response', async () => {
    mocks.get.mockResolvedValue({ success: false })
    const state: any = { loading: false, lines: [], logCount: 10, logLevel: 'info' }
    await (Logs as any).methods.loadData.call(state)
    expect(state.loading).toBe(false)
  })
  it('closes an edit modal if the inbound no longer exists', async () => {
    mocks.loadInbounds.mockResolvedValue([])
    const state: any = { loading: false, HasInData: [], closeModal: vi.fn() }
    await (Inbound as any).methods.loadData.call(state, 99)
    expect(state.loading).toBe(false)
    expect(state.closeModal).toHaveBeenCalledOnce()
  })
  it('includes the newest stats interval and keeps labels aligned', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    const now = new Date('2026-10-13T12:00:00Z')
    vi.setSystemTime(now)
    mocks.get.mockResolvedValue({ success: true, obj: [{ dateTime: now.getTime()/1000-5, direction: true, traffic: 123 }] })
    const state: any = { requestId: 0, loading: false, loaded: false, alert: false, limit: 1, resource: 'client', tag: 'test', genLable: String }
    await (Stats as any).methods.loadData.call(state)
    expect(state.loading).toBe(false)
    expect(state.usage.labels).toHaveLength(360)
    expect(state.usage.datasets[0].data).toHaveLength(360)
    expect(state.usage.datasets[0].data[359]).toBe(123)
  })
})
