import { describe, expect, it } from 'vitest'

import RandomUtil from './randomUtil'

describe('RandomUtil', () => {
  it('keeps generated integers inside the inclusive range', () => {
    for (let index = 0; index < 64; index += 1) {
      const value = RandomUtil.randomIntRange(-3, 5)
      expect(value).toBeGreaterThanOrEqual(-3)
      expect(value).toBeLessThanOrEqual(5)
    }
  })

  it('returns empty strings for non-positive sequence lengths', () => {
    expect(RandomUtil.randomSeq(0)).toBe('')
    expect(RandomUtil.randomLowerAndNum(-1)).toBe('')
  })

  it('uses the documented alphabets for generated sequences', () => {
    expect(RandomUtil.randomSeq(48)).toMatch(/^[0-9A-Za-z]{48}$/)
    expect(RandomUtil.randomLowerAndNum(48)).toMatch(/^[0-9a-z]{48}$/)
  })

  it('creates RFC 4122 version 4 UUIDs', () => {
    expect(RandomUtil.randomUUID()).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    )
  })
})