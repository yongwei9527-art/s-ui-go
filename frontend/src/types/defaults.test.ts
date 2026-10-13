import { describe, expect, it } from 'vitest'
import { createOutbound } from './outbounds'
import { createInbound } from './inbounds'
import { createEndpoint } from './endpoints'
import { createSrv } from './services'
import { createClient } from './clients'
import OutTLS from '@/components/tls/OutTLS.vue'

describe('independent default configurations', () => {
  it('does not share TLS, transport, peer or user arrays between forms', () => {
    const first: any = createOutbound('hysteria')
    first.tls.server_name = 'edited.example'
    expect((createOutbound('hysteria') as any).tls.server_name).toBeUndefined()
    const inbound: any = createInbound('vless')
    inbound.transport.type = 'ws'
    expect((createInbound('vless') as any).transport.type).toBeUndefined()
    const endpoint: any = createEndpoint('warp')
    endpoint.peers[0].public_key = 'edited'
    expect((createEndpoint('warp') as any).peers[0].public_key).toBe('')
    const service: any = createSrv('ocm')
    service.users.push({ username: 'edited' })
    expect((createSrv('ocm') as any).users).toHaveLength(0)
    createClient().inbounds.push(99)
    expect(createClient().inbounds).toHaveLength(0)
  })
  it('does not share a Reality key through TLS option defaults', () => {
    const setter = (OutTLS as any).computed.optionReality.set
    const first: any = { $props: { outbound: { tls: {} } } }
    setter.call(first, true)
    first.$props.outbound.tls.reality.public_key = 'node-A-key'
    const second: any = { $props: { outbound: { tls: {} } } }
    setter.call(second, true)
    expect(second.$props.outbound.tls.reality.public_key).not.toBe('node-A-key')
  })
})
