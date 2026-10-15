import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ post: vi.fn(), error: vi.fn() }))
vi.mock('@/plugins/httputil', () => ({ default: { post: mocks.post } }))
vi.mock('notivue', () => ({ push: { error: mocks.error } }))
import Backup from './Backup.vue'

function stateFor() {
  const state: any = { control: { visible: true } }
  Object.assign(state, (Backup as any).data.call(state))
  for (const [name, method] of Object.entries((Backup as any).methods)) {
    state[name] = (method as Function).bind(state)
  }
  return state
}

const file = () => new File(['backup'], 'backup.db')

beforeEach(() => {
  vi.stubGlobal('location', { reload: vi.fn() })
  localStorage.setItem('s-ui-authenticated', 'true')
})
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.resetAllMocks()
  localStorage.clear()
})

describe('database restore readiness', () => {
  it('probes readiness, not database import, across busy and listener restarts', async () => {
    vi.useFakeTimers()
    const fetch = vi.fn()
      .mockResolvedValueOnce({ status: 503 })
      .mockRejectedValueOnce(new Error('listener restarting'))
      .mockResolvedValueOnce({ status: 204 })
    vi.stubGlobal('fetch', fetch)
    const pending = stateFor().waitForRecovery()
    await vi.advanceTimersByTimeAsync(2000)
    expect(await pending).toBe(true)
    expect(fetch).toHaveBeenCalledTimes(3)
    expect(fetch.mock.calls.every(([url, options]) => url === 'api/recovery-ready' && options.cache === 'no-store')).toBe(true)
    expect(mocks.post).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('stops when the restored backup changed the panel route', async () => {
    const fetch = vi.fn().mockResolvedValue({ status: 404 })
    vi.stubGlobal('fetch', fetch)
    expect(await stateFor().waitForRecovery()).toBe(false)
    expect(fetch).toHaveBeenCalledOnce()
  })

  it('bounds retries even if the service stays unavailable', async () => {
    vi.useFakeTimers()
    const fetch = vi.fn().mockResolvedValue({ status: 503 })
    vi.stubGlobal('fetch', fetch)
    const pending = stateFor().waitForRecovery()
    await vi.advanceTimersByTimeAsync(45_000)
    expect(await pending).toBe(false)
    expect(fetch).toHaveBeenCalledTimes(45)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('aborts a hanging readiness request and retries without re-importing', async () => {
    vi.useFakeTimers()
    const fetch = vi.fn()
      .mockImplementationOnce((_url, options) => new Promise((_resolve, reject) => {
        options.signal.addEventListener('abort', () => reject(new Error('aborted')))
      }))
      .mockResolvedValueOnce({ status: 204 })
    vi.stubGlobal('fetch', fetch)
    const pending = stateFor().waitForRecovery()
    await vi.advanceTimersByTimeAsync(3000)
    expect(await pending).toBe(true)
    expect(mocks.post).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('reloads only after readiness and invalidates the old local login marker', async () => {
    mocks.post.mockResolvedValue({ success: true })
    const state = stateFor()
    let ready!: (value: boolean) => void
    state.waitForRecovery = vi.fn(() => new Promise<boolean>(resolve => { ready = resolve }))
    const pending = state.uploadDatabase(file())
    await Promise.resolve()
    expect(state.restoring).toBe(true)
    expect(state.control.visible).toBe(false)
    expect(localStorage.getItem('s-ui-authenticated')).toBeNull()
    expect(location.reload).not.toHaveBeenCalled()
    ready(true)
    await pending
    expect(location.reload).toHaveBeenCalledOnce()
    expect(state.restoring).toBe(false)
    const [url, body] = mocks.post.mock.calls[0]
    expect(url).toBe('api/importdb')
    expect(body.get('db').name).toBe('backup.db')
  })

  it('rejects double-clicks and keeps the dialog open after rejected imports', async () => {
    let finish!: (value: any) => void
    mocks.post.mockReturnValue(new Promise(resolve => { finish = resolve }))
    const state = stateFor()
    state.waitForRecovery = vi.fn()
    const first = state.uploadDatabase(file())
    await state.uploadDatabase(file())
    expect(mocks.post).toHaveBeenCalledOnce()
    finish({ success: false })
    await first
    expect(state.control.visible).toBe(true)
    expect(state.restoring).toBe(false)
    expect(localStorage.getItem('s-ui-authenticated')).toBe('true')
    expect(state.waitForRecovery).not.toHaveBeenCalled()
    expect(location.reload).not.toHaveBeenCalled()
  })

  it('does not auto-reload or re-import if recovery cannot be confirmed', async () => {
    mocks.post.mockResolvedValue({ success: true })
    const state = stateFor()
    state.waitForRecovery = vi.fn().mockResolvedValue(false)
    await state.uploadDatabase(file())
    expect(mocks.post).toHaveBeenCalledOnce()
    expect(location.reload).not.toHaveBeenCalled()
    expect(mocks.error.mock.calls[0][0].message).toContain('不要重复导入')
    expect(state.restoring).toBe(false)
  })

  it('clears busy state after an unexpected exception and warns against re-importing', async () => {
    mocks.post.mockRejectedValue(new Error('unexpected request failure'))
    const state = stateFor()
    await state.uploadDatabase(file())
    expect(state.restoring).toBe(false)
    expect(mocks.error.mock.calls[0][0].message).toContain('不要直接重复导入')
    expect(location.reload).not.toHaveBeenCalled()
  })
})
