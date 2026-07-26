import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  pushError: vi.fn(),
  pushSuccess: vi.fn(),
}))

vi.mock('@/plugins/httputil', () => ({
  default: {
    get: mocks.get,
    post: mocks.post,
  },
}))

vi.mock('notivue', () => ({
  push: {
    error: mocks.pushError,
    success: mocks.pushSuccess,
  },
}))

vi.mock('@/locales', () => ({
  i18n: {
    global: {
      t: (key: string) => key,
    },
  },
}))

import Data from './data'

describe('Data change revision tracking', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mocks.get.mockReset()
    mocks.post.mockReset()
    mocks.pushError.mockReset()
    mocks.pushSuccess.mockReset()
    localStorage.clear()
  })

  it('stores the API revision losslessly and sends it as the next lu', async () => {
    const store = Data()
    const revision = '18446744073709551615'
    mocks.get.mockResolvedValue({
      success: true,
      obj: {
        lastUpdate: revision,
        onlines: { inbound: [], outbound: [], user: [] },
        config: {},
      },
    })

    await store.loadData()

    expect(store.lastLoad).toBe(revision)
    expect(mocks.get).toHaveBeenLastCalledWith('api/load', {})

    await store.loadData()

    expect(mocks.get).toHaveBeenLastCalledWith('api/load', { lu: revision })
  })

  it('updates the revision from a no-change load response rather than the browser clock', async () => {
    const store = Data()
    store.lastLoad = '41'
    mocks.get.mockResolvedValue({
      success: true,
      obj: {
        lastUpdate: '42',
        onlines: { inbound: [], outbound: [], user: [] },
      },
    })

    await store.loadData()

    expect(store.lastLoad).toBe('42')
    expect(mocks.get).toHaveBeenCalledWith('api/load', { lu: '41' })
  })

  it('does not replace a known server revision with an invalid or unsafe value', () => {
    const store = Data()
    store.lastLoad = '99'

    store.setLastLoadRevision('not-a-revision')
    store.setLastLoadRevision(Number.MAX_SAFE_INTEGER + 1)

    expect(store.lastLoad).toBe('99')
  })

  it('does not derive a revision while applying a save response without lastUpdate', () => {
    const store = Data()
    store.lastLoad = '77'

    store.setNewData({ config: { log: { level: 'info' } } })

    expect(store.lastLoad).toBe('77')
  })
})
