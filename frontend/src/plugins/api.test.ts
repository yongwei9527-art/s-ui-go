import { describe, expect, it } from 'vitest'
import api from './api'

describe('request key cleanup', () => {
  it('does not retain a completed POST after Axios transforms its body', async () => {
    const adapter = async (config: any) => ({ data: {}, status: 200, statusText: 'OK', headers: {}, config })
    const body = { object: 'config', action: 'edit', data: '{}' }
    const first = await api.post('api/save', body, { adapter })
    expect(typeof first.config.data).toBe('string')
    await api.post('api/save', body, { adapter })
    expect(first.config.cancelToken?.reason).toBeUndefined()
  })
})
