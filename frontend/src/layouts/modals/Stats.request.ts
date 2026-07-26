import type { Msg } from '@/plugins/httputil'

export interface StatsRequestState {
  requestId: number
  loading: boolean
  loaded: boolean
  alert: boolean
}

export function beginStatsRequest(state: StatsRequestState): number {
  state.requestId++
  state.loading = true
  return state.requestId
}

export function getStatsRecords(state: StatsRequestState, requestId: number, data: Msg): any[] | null {
  if (requestId !== state.requestId || data.cancelled) {
    return null
  }

  if (!data.success || !Array.isArray(data.obj)) {
    state.alert = true
    state.loaded = false
    return null
  }

  return data.obj
}

export function failStatsRequest(state: StatsRequestState, requestId: number): void {
  if (requestId === state.requestId) {
    state.alert = true
    state.loaded = false
  }
}

export function finishStatsRequest(state: StatsRequestState, requestId: number): void {
  if (requestId === state.requestId) {
    state.loading = false
  }
}

export function invalidateStatsRequests(state: StatsRequestState): void {
  state.requestId++
  state.loading = false
}

export function stopStatsPolling(intervalId: any): number {
  if (intervalId && intervalId != 0) {
    clearInterval(intervalId)
  }
  return 0
}