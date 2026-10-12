import { describe, expect, it } from 'vitest'
import { createXrayDefaults } from './xrayDefaults'

describe('Xray Reality defaults', () => {
  it('supplies a consistent disabled XHTTP preset without shared credentials', () => {
    const preset = createXrayDefaults()
    expect(preset.enable).toBe(false)
    expect(preset.dest).toBe(`${preset.serverName}:443`)
    expect(preset.port).toBe(2088)
    expect(preset.transport).toBe('xhttp')
    expect(preset.path).toBe('/xhttp')
    expect(preset.mode).toBe('auto')
    expect([preset.privateKey, preset.publicKey, preset.shortId]).toEqual(['', '', ''])
  })
  it('does not retain edits when creating another node', () => {
    const first = createXrayDefaults()
    first.privateKey = 'changed'
    expect(createXrayDefaults().privateKey).toBe('')
  })
})