import axios from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  pushError: vi.fn(),
  pushSuccess: vi.fn(),
  routerPush: vi.fn(),
}))

vi.mock('@/plugins/api', () => ({
  default: {
    get: mocks.apiGet,
    post: mocks.apiPost,
  },
}))

vi.mock('notivue', () => ({
  push: {
    error: mocks.pushError,
    success: mocks.pushSuccess,
  },
}))

vi.mock('@/router', () => ({
  default: {
    push: mocks.routerPush,
  },
}))

vi.mock('@/locales', () => ({
  i18n: {
    global: {
      t: (key: string) => key,
    },
  },
}))

import HttpUtils from '@/plugins/httputil'

import {
  beginStatsRequest,
  finishStatsRequest,
  getStatsRecords,
  invalidateStatsRequests,
  stopStatsPolling,
  type StatsRequestState,
} from './Stats.request'
import { aggregateTrafficByBucket } from './Stats.traffic'

const createState = (): StatsRequestState => ({
  requestId: 0,
  loading: false,
  loaded: true,
  alert: false,
})

describe('aggregateTrafficByBucket', () => {
  const steps = [0, 10_000, 20_000]

  it('sums multiple records in the same bucket', () => {
    const result = aggregateTrafficByBucket([
      { dateTime: 1, direction: true, traffic: 120 },
      { dateTime: 5, direction: true, traffic: 80 },
      { dateTime: 8, direction: true, traffic: 50 },
    ], steps)

    expect(result.uplink).toEqual([250, null])
    expect(result.downlink).toEqual([null, null])
  })

  it('keeps upload and download totals independent', () => {
    const result = aggregateTrafficByBucket([
      { dateTime: 1, direction: true, traffic: 120 },
      { dateTime: 2, direction: false, traffic: 200 },
      { dateTime: 12, direction: true, traffic: 50 },
      { dateTime: 15, direction: false, traffic: 75 },
    ], steps)

    expect(result.uplink).toEqual([120, 50])
    expect(result.downlink).toEqual([200, 75])
  })

  it('includes each window start and excludes its end boundary', () => {
    const result = aggregateTrafficByBucket([
      { dateTime: 0, direction: true, traffic: 10 },
      { dateTime: 10, direction: true, traffic: 20 },
      { dateTime: 20, direction: true, traffic: 30 },
      { dateTime: -1, direction: true, traffic: 40 },
    ], steps)

    expect(result.uplink).toEqual([10, 20])
    expect(result.downlink).toEqual([null, null])
  })
})

describe('Stats and HttpUtils reliability', () => {
  beforeEach(() => {
    mocks.apiGet.mockReset()
    mocks.apiPost.mockReset()
    mocks.pushError.mockReset()
    mocks.pushSuccess.mockReset()
    mocks.routerPush.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns Axios cancellations silently without a failure notification', async () => {
    mocks.apiGet.mockRejectedValue(new axios.CanceledError('Duplicate request cancelled'))

    const result = await HttpUtils.get('api/stats')

    expect(result).toEqual({ success: false, msg: '', obj: null, cancelled: true })
    expect(mocks.pushError).not.toHaveBeenCalled()
    expect(mocks.pushSuccess).not.toHaveBeenCalled()
  })

  it('keeps the existing chart state when a Stats request is cancelled', () => {
    const state = createState()
    const requestId = beginStatsRequest(state)

    const records = getStatsRecords(state, requestId, {
      success: false,
      msg: '',
      obj: null,
      cancelled: true,
    })
    finishStatsRequest(state, requestId)

    expect(records).toBeNull()
    expect(state.loaded).toBe(true)
    expect(state.alert).toBe(false)
    expect(state.loading).toBe(false)
  })

  it('rejects a stale range response after a newer request starts', () => {
    const state = createState()
    const firstRequest = beginStatsRequest(state)
    const latestRequest = beginStatsRequest(state)
    const latestRecords = [{ dateTime: 3000, direction: true, traffic: 200 }]

    const appliedLatestRecords = getStatsRecords(state, latestRequest, {
      success: true,
      msg: '',
      obj: latestRecords,
    })
    const staleRecords = getStatsRecords(state, firstRequest, {
      success: true,
      msg: '',
      obj: [{ dateTime: 3000, direction: true, traffic: 100 }],
    })
    finishStatsRequest(state, firstRequest)
    finishStatsRequest(state, latestRequest)

    expect(appliedLatestRecords).toEqual(latestRecords)
    expect(staleRecords).toBeNull()
    expect(state.loading).toBe(false)
  })

  it('handles non-array Stats responses without leaving loading active', () => {
    const state = createState()
    const requestId = beginStatsRequest(state)

    const records = getStatsRecords(state, requestId, { success: true, msg: '', obj: {} })
    finishStatsRequest(state, requestId)

    expect(records).toBeNull()
    expect(state.loaded).toBe(false)
    expect(state.alert).toBe(true)
    expect(state.loading).toBe(false)
  })

  it('clears polling and invalidates pending requests when the dialog closes or unmounts', () => {
    vi.useFakeTimers()
    const state = createState()
    const loadData = vi.fn()
    const intervalId = setInterval(loadData, 10_000)

    expect(stopStatsPolling(intervalId)).toBe(0)
    invalidateStatsRequests(state)
    vi.advanceTimersByTime(20_000)

    expect(loadData).not.toHaveBeenCalled()
    expect(state.requestId).toBe(1)
    expect(state.loading).toBe(false)
  })
})