import { reactive } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), error: vi.fn(), data: {} as any }))
vi.mock('@/plugins/httputil', () => ({ default: { get: mocks.get } }))
vi.mock('@/store/modules/data', () => ({ default: () => mocks.data }))
vi.mock('@/locales', () => ({ i18n: { global: { t: (key: string) => key } }, locale: 'en' }))
vi.mock('notivue', () => ({ push: { error: mocks.error } }))
import DateTime from './DateTime.vue'
import Headers from './Headers.vue'
import Dial from './Dial.vue'
import Listen from './Listen.vue'
import Multiplex from './Multiplex.vue'
import OutJson from './OutJson.vue'
import DnsRule from './DnsRule.vue'
import Rule from './Rule.vue'
import SubJsonExt from './SubJsonExt.vue'
import SubClashExt from './SubClashExt.vue'
import WgPeer from './WgPeer.vue'
import AnyTls from './protocols/AnyTls.vue'
import Direct from './protocols/Direct.vue'
import Http from './protocols/Http.vue'
import Hysteria from './protocols/Hysteria.vue'
import Hysteria2 from './protocols/Hysteria2.vue'
import ShadowTls from './protocols/ShadowTls.vue'
import Socks from './protocols/Socks.vue'
import Ssh from './protocols/Ssh.vue'
import Tailscale from './protocols/Tailscale.vue'
import Tor from './protocols/Tor.vue'
import Tun from './protocols/Tun.vue'
import Vless from './protocols/Vless.vue'
import Vmess from './protocols/Vmess.vue'
import Warp from './protocols/Warp.vue'
import Wireguard from './protocols/Wireguard.vue'
import Acme from './tls/Acme.vue'
import Ech from './tls/Ech.vue'
import HttpTransport from './transports/Http.vue'
import WebSocket from './transports/WebSocket.vue'
import { InTypes } from '@/types/inbounds'

// Run the actual Options API methods with reactive props/computed setters,
// without involving unrelated Vuetify rendering or application services.
function stateFor(component: any, props: any = {}) {
  const state: any = reactive({ $props: props, ...props, $emit: vi.fn() })
  Object.assign(state, component.data?.call(state) ?? {})
  for (const [name, method] of Object.entries(component.methods ?? {})) {
    state[name] = (method as Function).bind(state)
  }
  for (const [name, option] of Object.entries(component.computed ?? {}) as any[]) {
    Object.defineProperty(state, name, {
      configurable: true,
      get: () => (typeof option === 'function' ? option : option.get).call(state),
      set: option.set ? (value: any) => option.set.call(state, value) : undefined,
    })
  }
  return state
}

afterEach(() => { vi.resetAllMocks(); mocks.data = {} })

describe('HTTP header editor boundaries', () => {
  it('preserves repeated values even when the first value is empty', () => {
    const state = stateFor(Headers, { data: {} })
    state.hdrs = [{ name: 'X-Test', value: '' }, { name: 'X-Test', value: 'second' }]
    expect(state.data.headers['X-Test']).toEqual(['', 'second'])
  })
  it('preserves special names as own JSON properties', () => {
    const state = stateFor(Headers, { data: {} })
    state.hdrs = [{ name: '__proto__', value: 'own' }, { name: 'constructor', value: 'header' }]
    expect(Object.hasOwn(state.data.headers, '__proto__')).toBe(true)
    expect(JSON.parse(JSON.stringify(state.data.headers))).toEqual({ ['__proto__']: 'own', constructor: 'header' })
  })
  it('ignores stale invalid row events without removing a different header', () => {
    const state = stateFor(Headers, { data: { headers: { 'X-Test': 'keep' } } })
    for (const index of [-1, 8, NaN]) {
      expect(() => state.update_key(index, 'bad')).not.toThrow()
      expect(() => state.update_value(index, 'bad')).not.toThrow()
      state.del_header(index)
    }
    expect(state.data.headers).toEqual({ 'X-Test': 'keep' })
    state.update_value(0, null)
    expect(state.data.headers).toEqual({ 'X-Test': '' })
  })
})

describe('null-safe component form setters', () => {
  const ruleFields = ['domain', 'domain_suffix', 'domain_keyword', 'domain_regex', 'ip_cidr', 'port', 'port_range', 'source_ip_cidr', 'source_port', 'source_port_range']
  const cases: [string, any, any, string[]][] = [
    ['DNS rule', DnsRule, { rule: {} }, ruleFields],
    ['route rule', Rule, { rule: {} }, ruleFields],
    ['AnyTLS', AnyTls, { data: {} }, ['padding_scheme']],
    ['HTTP', Http, { data: {} }, ['username', 'password']],
    ['SOCKS', Socks, { data: {} }, ['username', 'password']],
    ['Hysteria2', Hysteria2, { data: {} }, ['server_ports', 'masqueradeType']],
    ['SSH', Ssh, { data: {} }, ['private_key', 'host_key', 'algorithms']],
    ['Tailscale', Tailscale, { data: {} }, ['advertise_routes', 'relay_endpoints']],
    ['Tor', Tor, { data: {} }, ['extra_args']],
    ['TUN', Tun, { data: {} }, ['addrs']],
    ['Warp', Warp, { data: {} }, ['ifName']],
    ['WireGuard', Wireguard, { data: {} }, ['ifName', 'address', 'reserved', 'public_key']],
    ['WireGuard peer', WgPeer, { data: {}, ext: {} }, ['allowed_ips', 'reserved', 'address', 'privateKey', 'publicKey']],
    ['ACME', Acme, { tls: { acme: { domain: [] } } }, ['domains', 'caProvider']],
    ['ECH', Ech, { iTls: { ech: { enabled: true } }, oTls: { ech: {} } }, ['echKeyText', 'echConfigText']],
    ['HTTP transport', HttpTransport, { transport: {} }, ['hosts']],
    ['WebSocket', WebSocket, { transport: {} }, ['host']],
    ['outbound JSON', OutJson, { type: InTypes.VLESS, inData: { out_json: {} } }, ['server_ports', 'packet_encoding']],
    ['VLESS', Vless, { data: {} }, ['packet_encoding']],
    ['VMess', Vmess, { data: {} }, ['packet_encoding']],
  ]
  it.each(cases)('%s accepts cleared fields without throwing', (_name, component, props, fields) => {
    const state = stateFor(component, JSON.parse(JSON.stringify(props)))
    for (const field of fields) {
      expect(() => { state[field] = null }).not.toThrow()
      expect(() => { state[field] = undefined }).not.toThrow()
    }
  })
  it('does not emit an invalid expiration on clear or malformed input', () => {
    const state = stateFor(DateTime, { expiry: 0 })
    const previous = state.input
    for (const value of [null, undefined, '', 'invalid-date']) state.Input = value
    expect(state.$emit).not.toHaveBeenCalled()
    expect(state.input).toBe(previous)
    state.Input = '2026-10-13T12:00:00Z'
    expect(state.$emit).toHaveBeenCalledWith('submit', Date.parse('2026-10-13T12:00:00Z') / 1000)
  })
  it('handles absent tag lists and options with just one defined field', () => {
    const dial = stateFor(Dial, { dial: { tcp_multi_path: false } })
    expect(dial.outTags).toEqual([])
    expect(dial.optionTCP).toBe(true)
    dial.optionDetour = true
    expect(dial.dial.detour).toBe('')
    const listen = stateFor(Listen, { data: { tcp_fast_open: false, udp_timeout: '2m' } })
    expect(listen.optionTCP).toBe(true)
    expect(listen.optionUDP).toBe(true)
    listen.optionDetour = true
    expect(listen.data.detour).toBe('')
    listen.optionTCP = false
    listen.optionUDP = false
    expect(listen.optionTCP).toBe(false)
    expect(listen.optionUDP).toBe(false)
  })
  it('does not update disabled multiplex settings from a delayed field event', () => {
    const state = stateFor(Multiplex, { data: { multiplex: { enabled: true, brutal: { enabled: true } } } })
    state.down_mbps = '12'
    state.up_mbps = null
    expect(state.mux.brutal).toMatchObject({ down_mbps: 12, up_mbps: 0 })
    state.muxEnable = false
    expect(() => { state.down_mbps = null; state.up_mbps = null }).not.toThrow()
    expect(state.data.multiplex).toBeUndefined()
  })
})

describe('protocol patch semantics', () => {
  it('matches sing-tun DNSModeOrDefault and removes obsolete hijack fields on edit', () => {
    const state = stateFor(Tun, { data: { auto_route: true } })
    expect(state.dnsHijack).toBe(true)
    state.data.dns_mode = 'native'
    expect(state.dnsHijack).toBe(false)
    state.data.dns_hijack = ['any:53']
    state.data.dns_hijack_address = ['1.1.1.1']
    state.dnsHijack = false
    expect(state.data).toEqual({ auto_route: true, dns_mode: 'disabled' })
    state.dnsHijack = true
    expect(state.data.dns_mode).toBe('hijack')
    delete state.data.dns_mode
    state.data.auto_route = false
    expect(state.dnsHijack).toBe(false)
  })
  it.each([Vless, OutJson])('keeps VLESS omitted/default and explicit none distinct', component => {
    const state = stateFor(component, { data: {}, inData: { out_json: {} }, type: InTypes.VLESS })
    const output = component === Vless ? state.data : state.inData.out_json
    expect(state.packet_encoding).toBe('xudp')
    state.packet_encoding = 'none'
    expect(output.packet_encoding).toBe('')
    expect(JSON.stringify(output)).toContain('"packet_encoding":""')
    expect(state.packet_encoding).toBe('none')
    state.packet_encoding = 'packetaddr'
    expect(state.packet_encoding).toBe('packetaddr')
  })
  it('keeps VMess none omitted and offers the supported CFB cipher', () => {
    const state = stateFor(Vmess, { data: {} })
    state.packet_encoding = 'none'
    expect(state.data.packet_encoding).toBeUndefined()
    expect(state.securities).toContain('aes-128-cfb')
    expect(state.securities).not.toContain('aes-128-ctr')
    expect(stateFor(OutJson).vmessSecurities).toEqual(state.securities)
  })
  it('normalizes Hysteria bandwidth and removes overriding legacy values', () => {
    const state = stateFor(Hysteria, { data: { down: '100 Mbps', up: '100 Mbps' } })
    state.down_mbps = '12'
    state.up_mbps = 10
    expect(state.data).toEqual({ down_mbps: 12, up_mbps: 10 })
    state.down_mbps = null
    expect(state.data.down_mbps).toBe(0)
  })
  it('validates numeric ports and avoids mutating a ShadowTLS config on a read', () => {
    const direct = stateFor(Direct, { data: {} })
    const shadow = stateFor(ShadowTls, { data: { handshake: { server: 'example.com' } } })
    for (const value of [null, '', 0, -1, 65536, 1.5, 'invalid']) {
      direct.override_port = value
      shadow.server_port = value
      expect(direct.data.override_port).toBeUndefined()
      expect(shadow.data.handshake.server_port).toBe(443)
    }
    direct.override_port = 65535
    shadow.server_port = '8443'
    expect(direct.data.override_port).toBe(65535)
    expect(shadow.data.handshake.server_port).toBe(8443)
    expect(shadow.version).toBe(1)
    expect(shadow.data.version).toBeUndefined()
    shadow.handshake_server = '  '
    shadow.addHandshakeServer()
    expect(shadow.data.handshake_for_server_name).toBeUndefined()
    shadow.handshake_server = ' sni.example '
    shadow.addHandshakeServer()
    expect(shadow.data.handshake_for_server_name['sni.example']).toEqual({ server: '', server_port: 443 })
  })
  it('preserves the SSH user and round-trips multiline keys', () => {
    const state = stateFor(Ssh, { data: { user: 'alice', private_key: ['line1', 'line2'] } })
    expect(state.usePath).toBe(1)
    expect(state.private_key).toBe('line1\nline2')
    state.private_key = 'new1\nnew2'
    expect(state.data.private_key).toEqual(['new1', 'new2'])
    state.optionKey = true
    expect(state.data.user).toBe('alice')
    state.optionHostKey = true
    expect(state.host_key).toBe('')
    expect(state.data.host_key).toEqual([])
  })
  it('supports the one-second Tailscale timeout', () => {
    const state = stateFor(Tailscale, { data: {} })
    state.udpTimeout = 1
    expect(state.data.udp_timeout).toBe('1s')
  })
  it('preserves unrelated WebSocket headers when editing or clearing Host', () => {
    const state = stateFor(WebSocket, { transport: { headers: { host: 'old', Authorization: 'token', Custom: ['value'] } } })
    expect(state.host).toBe('old')
    state.host = 'new'
    expect(state.transport.headers).toEqual({ Host: 'new', Authorization: 'token', Custom: ['value'] })
    state.host = null
    expect(state.transport.headers).toEqual({ Authorization: 'token', Custom: ['value'] })
  })
  it('creates a missing peer key record and updates it without an index -1 write', () => {
    const state = stateFor(WgPeer, { data: { public_key: 'pub' }, ext: {} })
    expect(state.privateKey).toBe('')
    state.privateKey = 'private'
    state.publicKey = 'renamed'
    expect(state.ext.keys).toEqual([{ public_key: 'renamed', private_key: 'private' }])
    expect(state.privateKey).toBe('private')
    const withoutExt = stateFor(WgPeer, { data: {} })
    expect(() => { withoutExt.privateKey = 'ignored'; withoutExt.publicKey = 'pub' }).not.toThrow()
  })
})

describe('route interface maps', () => {
  it('keeps incomplete JSON in a local draft without corrupting the last valid map', () => {
    const state = stateFor(Rule, { rule: { interface_address: { eth0: ['10.0.0.0/8'] } } })
    const original = JSON.stringify(state.rule)
    state.interface_addr = '{ "eth1": ['
    expect(state.interface_addr).toBe('{ "eth1": [')
    expect(JSON.stringify(state.rule)).toBe(original)
    state.interface_addr = '{ "eth1": ["192.168.0.0/16"] }'
    expect(state.rule.interface_address).toEqual({ eth1: ['192.168.0.0/16'] })
    state.interface_addr = '{ "eth1": 3 }'
    expect(state.rule.interface_address).toEqual({ eth1: ['192.168.0.0/16'] })
    state.interface_addr = null
    expect(state.rule.interface_address).toEqual({})
  })
  it('does not open the list editor for a map, and switches map/list shapes safely', () => {
    const state = stateFor(Rule, { rule: {} })
    state.optionInterface = true
    state.openExpTextarea('Interfaces', 'interface_address')
    expect(state.expTextarea.visible).toBe(false)
    state.updateInterfaceOption('default_interface_address')
    state.interface_addr = ' 10.0.0.0/8 \n\n192.168.0.0/16'
    state.openExpTextarea('Interfaces', 'default_interface_address')
    expect(state.expTextarea.content).toBe('10.0.0.0/8\n192.168.0.0/16')
    state.updateInterfaceOption('network_interface_address')
    expect(state.rule).toEqual({ network_interface_address: {} })
    expect(state.interface_addr).toBe('{}')
  })
})

describe('subscription defaults and editor validation', () => {
  it.each(['enableLog', 'enableDns', 'enableInb', 'enableExp'])('%s restores a fresh deep copy after edits', field => {
    const state = stateFor(SubJsonExt, { settings: {} })
    const defaults = JSON.stringify([state.defaultLog, state.defaultDns, state.defaultInb, state.defaultExp])
    state[field] = true
    if (field === 'enableLog') state.subJsonExt.log.level = 'debug'
    if (field === 'enableDns') state.dns.servers[0].server = 'changed'
    if (field === 'enableInb') state.inbounds[0].address.push('changed')
    if (field === 'enableExp') state.subJsonExt.experimental.clash_api.secret = 'changed'
    state[field] = false
    state[field] = true
    expect(JSON.stringify([state.defaultLog, state.defaultDns, state.defaultInb, state.defaultExp])).toBe(defaults)
    expect(JSON.stringify(state.subJsonExt)).not.toContain('changed')
  })
  it('clones platform defaults and ignores delayed toggles after disabling inbounds', () => {
    const state = stateFor(SubJsonExt, { settings: {} })
    state.enableInb = true
    state.platformProxy = false
    state.platformProxy = true
    state.inbounds[0].platform.http_proxy.server = 'changed'
    expect(state.defaultInb[0].platform.http_proxy.server).toBe('127.0.0.1')
    state.enableInb = false
    expect(state.platformProxy).toBe(false)
    expect(() => { state.platformProxy = true; state.final = null }).not.toThrow()
  })
  it.each(['null', '[]', 'false', '{"dns":false}', '{"dns":""}', '{"dns":null}', '{"dns":{"servers":[null]}}', '{"dns":{"rules":[null]}}', '{"log":"info"}', '{"experimental":[]}', '{"inbounds":[null]}', '{"rules":null}', '{"rules":[{"rule_set":4}]}', '{"rules":[{"rules":[null]}]}', '{"rule_set":[null]}'])('rejects unsafe editor input %s without changing the existing config', text => {
    const state = stateFor(SubJsonExt, { settings: {} })
    state.subJsonExt = { log: { level: 'info' } }
    state.enableEditor = true
    state.saveEditor(text)
    expect(state.subJsonExt).toEqual({ log: { level: 'info' } })
    expect(state.enableEditor).toBe(true)
    expect(mocks.error).toHaveBeenCalledOnce()
  })
  it('loads empty/malformed settings without throwing and fills missing DNS entries independently', () => {
    const state = stateFor(SubJsonExt, { settings: { subJsonExt: '  ' } })
    state.loadData()
    expect(state.subJsonExt).toEqual({})
    state.settings.subJsonExt = 'not json'
    state.loadData()
    expect(state.subJsonExt).toEqual({})
    const config = state.parseConfig('{"dns":{"servers":[{"tag":"proxy-dns","server":"custom"}]}}')
    expect(config.dns.servers).toHaveLength(3)
    expect(config.dns.servers[0].server).toBe('custom')
    config.dns.servers[1].tag = 'changed'
    expect(state.defaultDns.servers[1].tag).toBe('direct-dns')
  })
  it('preserves custom rule sets and logical/scalar references while cloning new defaults', () => {
    const state = stateFor(SubJsonExt, { settings: {} })
    state.subJsonExt = state.parseConfig(JSON.stringify({
      rules: [{ type: 'logical', rules: [{ rule_set: 'geosite-cn' }], action: 'route', outbound: 'proxy' }],
      rule_set: [{ tag: 'custom', url: 'custom.srs' }, { tag: 'geosite-cn', url: 'override.srs' }],
    }))
    state.ruleToDirect = ['geosite-private']
    expect(state.subJsonExt.rule_set).toContainEqual({ tag: 'custom', url: 'custom.srs' })
    expect(state.subJsonExt.rule_set).toContainEqual({ tag: 'geosite-cn', url: 'override.srs' })
    state.subJsonExt.rule_set.find((r: any) => r.tag === 'geosite-private').url = 'changed'
    expect(state.geo.find((r: any) => r.tag === 'geosite-private').url).not.toBe('changed')
    state.ruleToDirect = null
    expect(state.subJsonExt.rule_set.map((r: any) => r.tag)).toEqual(['custom', 'geosite-cn'])
    state.ruleToBlock = null
    state.dnsToDirect = null
    expect(state.subJsonExt.rules).toHaveLength(1)
  })
  it('normalizes scalar rule-set selections and handles clearing an empty config', () => {
    const state = stateFor(SubJsonExt, { settings: {} })
    state.ruleToDirect = null
    state.ruleToBlock = null
    expect(state.subJsonExt).toEqual({})
    state.subJsonExt = state.parseConfig('{"rules":[{"outbound":"direct","rule_set":"geosite-cn"}]}')
    expect(state.ruleToDirect).toEqual(['geosite-cn'])
  })
  it('keeps Clash rules safe for absent/scalar YAML and rejects null saves', () => {
    const state = stateFor(SubClashExt, { settings: { subClashExt: '' } })
    for (const value of ['', 'null', 'true', '[]', 'rules: invalid']) {
      state.settings.subClashExt = value
      expect(state.rules).toEqual([])
    }
    state.enableEditor = true
    state.saveEditor('null')
    expect(state.enableEditor).toBe(true)
    state.rules = null
    expect(state.metaJson.rules).toEqual(['MATCH,Proxy'])
  })
})

describe('ECH async generation lifecycle', () => {
  const response = { success: true, obj: ['-----BEGIN ECH CONFIGS-----', 'config', '-----END ECH CONFIGS-----', '-----BEGIN ECH KEYS-----', 'key', '-----END ECH KEYS-----'] }
  const echState = () => stateFor(Ech, { iTls: { server_name: 'example.com', ech: { enabled: true, key_path: 'existing.pem' } }, oTls: { ech: { config: ['existing'] } } })
  function pending() {
    let resolve!: (value: any) => void
    const promise = new Promise(r => { resolve = r })
    mocks.get.mockReturnValueOnce(promise)
    return resolve
  }
  it('applies complete key/config results and releases the loading state', async () => {
    const state = echState()
    mocks.get.mockResolvedValue(response)
    await state.genECH()
    expect(state.loading).toBe(false)
    expect(state.useEchPath).toBe(1)
    expect(state.iTls.ech.key_path).toBeUndefined()
    expect(state.iTls.ech.key).toEqual(response.obj.slice(3))
    expect(state.oTls.ech.config).toEqual(response.obj.slice(0, 3))
  })
  it.each([null, [], 'text', ['-----BEGIN ECH KEYS-----', 'partial']])('preserves existing data on malformed success response %j', async obj => {
    const state = echState()
    mocks.get.mockResolvedValue({ success: true, obj })
    await state.genECH()
    expect(state.loading).toBe(false)
    expect(state.iTls.ech.key_path).toBe('existing.pem')
    expect(state.oTls.ech.config).toEqual(['existing'])
    expect(mocks.error).toHaveBeenCalledOnce()
  })
  it('resets loading after a rejected request', async () => {
    const state = echState()
    mocks.get.mockRejectedValue(new Error('network failure'))
    await state.genECH()
    expect(state.loading).toBe(false)
    expect(state.iTls.ech.key_path).toBe('existing.pem')
  })
  it('does not overwrite a re-enabled ECH config or clear a newer request loading flag', async () => {
    const state = echState()
    const resolveOld = pending()
    const oldRequest = state.genECH()
    await state.genECH()
    expect(mocks.get).toHaveBeenCalledOnce()
    state.enabled = false
    state.enabled = true
    const resolveNew = pending()
    const newRequest = state.genECH()
    resolveOld(response)
    await oldRequest
    expect(state.loading).toBe(true)
    expect(state.iTls.ech.key).toBeUndefined()
    resolveNew(response)
    await newRequest
    expect(state.loading).toBe(false)
    expect(state.iTls.ech.key).toEqual(response.obj.slice(3))
  })
  it.each(['replace', 'server-name', 'unmount'])('ignores results after %s', async change => {
    const state = echState()
    const resolve = pending()
    const request = state.genECH()
    if (change === 'replace') state.iTls = { ech: { enabled: true, key_path: 'new.pem' } }
    if (change === 'server-name') state.iTls.server_name = 'other.example'
    if (change === 'unmount') (Ech as any).beforeUnmount.call(state)
    resolve(response)
    await request
    expect(state.iTls.ech.key).toBeUndefined()
    expect(state.oTls.ech.config).toEqual(['existing'])
    expect(state.loading).toBe(false)
  })
  it('ignores delayed text updates after ECH is disabled', () => {
    const state = echState()
    state.enabled = false
    state.echKeyText = null
    state.echConfigText = null
    expect(state.iTls.ech).toBeUndefined()
    expect(state.oTls.ech).toBeUndefined()
  })
})
