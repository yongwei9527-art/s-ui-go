import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/plugins/httputil', () => ({ default: { get: mocks.get, post: mocks.post } }))
vi.mock('@/locales', () => ({ i18n: { global: { t: (key: string) => key } } }))
vi.mock('notivue', () => ({ push: { success: vi.fn(), error: vi.fn() } }))
import Data from '@/store/modules/data'

function deferred() {
  let resolve!: (value: any) => void
  const promise = new Promise<any>(res => { resolve = res })
  return { promise, resolve }
}
beforeEach(() => { localStorage.clear(); setActivePinia(createPinia()) })
afterEach(() => vi.resetAllMocks())

// Regression expectations for the two previously reproduced store defects.
// Passing now means stale responses were rejected, not that a defect was observed.
describe('round7 repaired store boundaries', () => {
  it('rejects an old background read after a configuration save', async () => {
    const state = Data(); state.lastLoad = '10'
    const old = deferred(); mocks.get.mockReturnValue(old.promise)
    const read = state.loadData()
    const saved = { route: { final: 'new' } }
    mocks.post.mockResolvedValue({ success: true, obj: { config: saved } })
    await state.save('config', 'set', saved)
    expect(state.config.route.final).toBe('new')
    old.resolve({ success: true, obj: { lastUpdate: '10', onlines: {}, config: { route: { final: 'old' } } } })
    await read
    expect(state.config.route.final).toBe('new')
  })
  it('keeps the newer revision and configuration when reads finish out of order', async () => {
    const state = Data(), old = deferred(), latest = deferred()
    mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(latest.promise)
    const a = state.loadData(), b = state.loadData()
    latest.resolve({ success: true, obj: { lastUpdate: '11', onlines: {}, config: { route: { final: 'latest' } } } }); await b
    old.resolve({ success: true, obj: { lastUpdate: '10', onlines: {}, config: { route: { final: 'stale' } } } }); await a
    expect(state.lastLoad).toBe('11')
    expect(state.config.route.final).toBe('latest')
  })
})