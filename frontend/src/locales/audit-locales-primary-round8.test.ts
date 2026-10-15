import { readFileSync, readdirSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'
import { parse } from '@vue/compiler-sfc'
import { baseParse, NodeTypes } from '@vue/compiler-dom'
import { baseCompile } from '@intlify/message-compiler'
import { createApp, nextTick, type App } from 'vue'
import { createI18n } from 'vue-i18n'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import en from './en'
import zhcn from './zhcn'
import zhtw from './zhtw'
import { i18n, languages } from './index'

// Vuetify auto-imports components directly, so global app.component stubs are insufficient.
const ui = await vi.hoisted(async () => {
  const { defineComponent, h } = await import('vue')
  return { module: (names: string[]) => Object.fromEntries(names.map(name => [name, defineComponent({
    name: name + 'AuditStub', inheritAttrs: false,
    props: ['label', 'title', 'subtitle', 'text', 'suffix', 'modelValue'],
    setup(props, { slots }) {
      return () => h('div', { 'data-ui': name, 'data-label': props.label, 'data-suffix': props.suffix }, [
        props.title, props.subtitle, props.text, slots.default?.(),
      ])
    },
  })])) }
})
vi.mock('vuetify/components/VAlert', () => ui.module(["VAlert"]))
vi.mock('vuetify/components/VApp', () => ui.module(["VApp"]))
vi.mock('vuetify/components/VAppBar', () => ui.module(["VAppBar","VAppBarTitle"]))
vi.mock('vuetify/components/VBadge', () => ui.module(["VBadge"]))
vi.mock('vuetify/components/VBtn', () => ui.module(["VBtn"]))
vi.mock('vuetify/components/VBtnToggle', () => ui.module(["VBtnToggle"]))
vi.mock('vuetify/components/VCard', () => ui.module(["VCard","VCardActions","VCardSubtitle","VCardText","VCardTitle"]))
vi.mock('vuetify/components/VCheckbox', () => ui.module(["VCheckbox"]))
vi.mock('vuetify/components/VChip', () => ui.module(["VChip"]))
vi.mock('vuetify/components/VCombobox', () => ui.module(["VCombobox"]))
vi.mock('vuetify/components/VDataTable', () => ui.module(["VDataTable"]))
vi.mock('vuetify/components/VDialog', () => ui.module(["VDialog"]))
vi.mock('vuetify/components/VDivider', () => ui.module(["VDivider"]))
vi.mock('vuetify/components/VExpansionPanel', () => ui.module(["VExpansionPanel","VExpansionPanelText","VExpansionPanelTitle","VExpansionPanels"]))
vi.mock('vuetify/components/VFileInput', () => ui.module(["VFileInput"]))
vi.mock('vuetify/components/VForm', () => ui.module(["VForm"]))
vi.mock('vuetify/components/VGrid', () => ui.module(["VCol","VContainer","VRow","VSpacer"]))
vi.mock('vuetify/components/VIcon', () => ui.module(["VIcon"]))
vi.mock('vuetify/components/VList', () => ui.module(["VList","VListItem","VListItemTitle"]))
vi.mock('vuetify/components/VMain', () => ui.module(["VMain"]))
vi.mock('vuetify/components/VMenu', () => ui.module(["VMenu"]))
vi.mock('vuetify/components/VNavigationDrawer', () => ui.module(["VNavigationDrawer"]))
vi.mock('vuetify/components/VOverlay', () => ui.module(["VOverlay"]))
vi.mock('vuetify/components/VProgressCircular', () => ui.module(["VProgressCircular"]))
vi.mock('vuetify/components/VProgressLinear', () => ui.module(["VProgressLinear"]))
vi.mock('vuetify/components/VRadio', () => ui.module(["VRadio"]))
vi.mock('vuetify/components/VRadioGroup', () => ui.module(["VRadioGroup"]))
vi.mock('vuetify/components/VSelect', () => ui.module(["VSelect"]))
vi.mock('vuetify/components/VSkeletonLoader', () => ui.module(["VSkeletonLoader"]))
vi.mock('vuetify/components/VSwitch', () => ui.module(["VSwitch"]))
vi.mock('vuetify/components/VTable', () => ui.module(["VTable"]))
vi.mock('vuetify/components/VTabs', () => ui.module(["VTab","VTabs"]))
vi.mock('vuetify/components/VTextField', () => ui.module(["VTextField"]))
vi.mock('vuetify/components/VTextarea', () => ui.module(["VTextarea"]))
vi.mock('vuetify/components/VTooltip', () => ui.module(["VTooltip"]))
vi.mock('vuetify/components/VWindow', () => ui.module(["VWindow","VWindowItem"]))

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/plugins/httputil', () => ({ default: { get: mocks.get, post: mocks.post } }))
vi.mock('@/store/modules/data', () => ({ default: () => ({ onlines: {}, inbounds: [], outbounds: [], clients: [] }) }))
vi.mock('notivue', () => ({ push: { error: vi.fn(), success: vi.fn() } }))
vi.mock('@/layouts/modals/Logs.vue', () => ({ default: { render: () => null } }))
vi.mock('@/layouts/modals/Backup.vue', () => ({ default: { render: () => null } }))
vi.mock('@/layouts/modals/UsageStats.vue', () => ({ default: { render: () => null } }))
vi.mock('@/components/Editor.vue', () => ({ default: { render: () => null } }))
import Headers from '@/components/Headers.vue'
import RuleOptions from '@/components/Rule.vue'
import RuleModal from '@/layouts/modals/Rule.vue'
import Tailscale from '@/components/protocols/Tailscale.vue'
import VMess from '@/components/protocols/Vmess.vue'
import Tun from '@/components/protocols/Tun.vue'
import Multiplex from '@/components/Multiplex.vue'
import Acme from '@/components/tls/Acme.vue'
import OutTLS from '@/components/tls/OutTLS.vue'
import Settings from '@/views/Settings.vue'
import Main from '@/components/Main.vue'

const root = dirname(fileURLToPath(import.meta.url))
const srcRoot = dirname(root)
const catalogs = { en, zhcn, zhtw }
const localeIds = { en: 'en', zhcn: 'zhHans', zhtw: 'zhHant' }
const configuredLocales = [...i18n.global.availableLocales]
const composer = i18n.global as any

function flatten(value: Record<string, any>, prefix = '', result: Record<string, string> = {}) {
  for (const [name, child] of Object.entries(value)) {
    const key = prefix ? `${prefix}.${name}` : name
    if (typeof child === 'string') result[key] = child
    else flatten(child, key, result)
  }
  return result
}
const flat = Object.fromEntries(Object.entries(catalogs).map(([locale, messages]) => [locale, flatten(messages)]))

// Inspect source ASTs: evaluating object literals would hide overwritten duplicate keys.
function objectIssues(file: string) {
  const source = ts.createSourceFile(file, readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  const issues: string[] = []
  const exported = source.statements.find(ts.isExportAssignment)
  if (!exported || !ts.isObjectLiteralExpression(exported.expression)) return ['default export must be an object']
  function visit(node: ts.ObjectLiteralExpression, prefix = '') {
    const seen = new Set<string>()
    for (const prop of node.properties) {
      if (!ts.isPropertyAssignment(prop) || !prop.name || ts.isComputedPropertyName(prop.name)) {
        issues.push(`${prefix}: expected a plain named property`)
        continue
      }
      const name = (prop.name as ts.Identifier | ts.StringLiteral).text
      const key = prefix ? `${prefix}.${name}` : name
      if (seen.has(name)) issues.push(`${key}: duplicate`)
      seen.add(name)
      if (ts.isObjectLiteralExpression(prop.initializer)) visit(prop.initializer, key)
      else if (!ts.isStringLiteral(prop.initializer) || !prop.initializer.text.trim()) issues.push(`${key}: expected non-empty string`)
    }
  }
  visit(exported.expression)
  return issues
}

// Parse actual SFC expressions (including directives), not regex matches in comments/tests.
const literalCalls: { key: string; file: string }[] = []
const routeKeys = new Set<string>()
function scanExpressions(code: string, file: string) {
  const ast = ts.createSourceFile(file, code, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  function visit(node: ts.Node) {
    if (ts.isCallExpression(node)) {
      const callee = node.expression
      const name = ts.isIdentifier(callee) ? callee.text : ts.isPropertyAccessExpression(callee) ? callee.name.text : ''
      const arg = node.arguments[0]
      if (['t', '$t', 'te', 'tm'].includes(name) && arg && (ts.isStringLiteral(arg) || ts.isNoSubstitutionTemplateLiteral(arg))) {
        literalCalls.push({ key: arg.text, file })
      }
    }
    if (file === 'router/index.ts' && ts.isPropertyAssignment(node) && node.name.getText(ast) === 'name' && ts.isStringLiteral(node.initializer)) {
      routeKeys.add(node.initializer.text)
    }
    ts.forEachChild(node, visit)
  }
  visit(ast)
}
function scanDirectory(directory: string) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const file = join(directory, entry.name)
    const name = relative(srcRoot, file).replaceAll('\\', '/')
    if (entry.isDirectory()) {
      if (name !== 'locales') scanDirectory(file)
    } else if (/\.(vue|ts)$/.test(name) && !name.endsWith('.test.ts')) {
      const source = readFileSync(file, 'utf8')
      if (name.endsWith('.ts')) scanExpressions(source, name)
      else {
        const sfc = parse(source).descriptor
        for (const script of [sfc.script, sfc.scriptSetup]) if (script) scanExpressions(script.content, name)
        if (sfc.template) {
          function visit(node: any) {
            if (node.type === NodeTypes.INTERPOLATION) scanExpressions(node.content.content, name)
            if (node.type === NodeTypes.ELEMENT) {
              for (const prop of node.props) if (prop.type === NodeTypes.DIRECTIVE && prop.exp) scanExpressions(prop.exp.content, name)
            }
            for (const child of node.children ?? []) visit(child)
          }
          visit(baseParse(sfc.template.content))
        }
      }
    }
  }
}
scanDirectory(srcRoot)

const namedContracts: Record<string, string[]> = {
  'rule.import.conflictDesc': ['rules', 'rulesets'],
  'rule.import.errJsonParse': ['message'],
  'rule.import.errFetch': ['message'],
}
function namedVariables(node: any, names = new Set<string>()) {
  if (!node || typeof node !== 'object') return names
  if (node.type === 4 && typeof node.key === 'string') names.add(node.key)
  for (const [key, child] of Object.entries(node)) {
    if (key === 'loc') continue
    if (Array.isArray(child)) child.forEach(item => namedVariables(item, names))
    else if (child && typeof child === 'object') namedVariables(child, names)
  }
  return names
}

const apps: App[] = []
const hosts: HTMLElement[] = []
function mount(component: any, props: Record<string, any> = {}) {
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp(component, props)
  app.use(i18n)
  app.provide('loading', undefined)
  app.directive('tooltip', {})
  apps.push(app)
  hosts.push(host)
  const vm = app.mount(host) as any
  return { host, vm }
}
function labels(host: HTMLElement) {
  return [...host.querySelectorAll('[data-label]')].map(node => node.getAttribute('data-label'))
}
const flush = async () => { for (let i = 0; i < 8; i++) await Promise.resolve(); await nextTick() }
beforeEach(() => {
  vi.useFakeTimers()
  composer.locale.value = 'en'
  composer.fallbackLocale.value = false
  mocks.get.mockImplementation(async (url: string) => ({ success: true, obj: url === 'api/logs' ? [] : {} }))
})
afterEach(() => {
  apps.splice(0).forEach(app => app.unmount())
  hosts.splice(0).forEach(host => host.remove())
  composer.locale.value = 'en'
  composer.fallbackLocale.value = 'en'
  vi.useRealTimers()
  vi.clearAllMocks()
})

describe('round8 primary locale schema and real Vue I18n contracts', () => {
  it('keeps the production language registry unchanged; Traditional Chinese is tested independently', () => {
    expect(configuredLocales.sort()).toEqual(['en', 'zhHans'])
    expect(languages.map(language => language.value)).toEqual(['zhHans', 'en'])
  })
  it('uses correct Traditional Chinese characters for copying and not dropping', () => {
    const instance = createI18n({ legacy: false, locale: 'zhHant', fallbackLocale: false, messages: { zhHant: zhtw } })
    expect(instance.global.t('copyToClipboard')).toBe('複製到剪貼板')
    expect(instance.global.t('rule.noDrop')).toBe('不丟棄')
  })
  for (const [locale, messages] of Object.entries(catalogs)) {
    it(`${locale}: has no overwritten/empty/non-string leaves and matches the primary schema`, () => {
      expect(objectIssues(join(root, `${locale}.ts`))).toEqual([])
      expect(Object.keys(flat[locale]).sort()).toEqual(Object.keys(flat.en).sort())
    })
    it(`${locale}: compiles and renders every leaf without fallback, missing keys or interpolation drift`, () => {
      const errors: string[] = []
      const missing = vi.fn()
      const instance = createI18n({ legacy: false, locale, fallbackLocale: false, messages: { [locale]: messages }, missing })
      for (const [key, message] of Object.entries(flat[locale])) {
        const compiled = baseCompile(message, { onError: error => errors.push(`${key}: ${error.message}`) })
        expect([...namedVariables(compiled.ast)].sort(), key).toEqual(namedContracts[key] ?? [])
        const rendered = instance.global.t(key, { rules: 17, rulesets: 29, message: 'UPSTREAM_MARKER' })
        expect(typeof rendered, key).toBe('string')
        expect(rendered, key).not.toMatch(/\{(?:rules|rulesets|message)\}/)
      }
      expect(errors).toEqual([])
      expect(missing).not.toHaveBeenCalled()
    })
    it(`${locale}: supplies all literal UI keys and route-name titles found in the actual sources`, () => {
      expect(literalCalls.length).toBeGreaterThan(1000)
      expect(routeKeys.size).toBeGreaterThan(10)
      expect(literalCalls.filter(call => !Object.hasOwn(flat[locale], call.key))).toEqual([])
      expect([...routeKeys].filter(key => !Object.hasOwn(flat[locale], key))).toEqual([])
    })
    it(`${locale}: covers ACME's actual provider parameter names and closed dynamic key families`, () => {
      const providers = (Acme as any).data().dnsProviders
      const keys = [
        ...providers.flatMap((provider: any) => provider.params.map((param: string) => `tls.acme.dns01Params.${param}`)),
        ...['light', 'dark', 'system'].map(theme => `theme.${theme}`),
        ...['add', 'edit', 'clone', 'del', 'set'].map(action => `actions.${action}`),
        ...['client', 'inbound', 'outbound'].map(resource => `objects.${resource}`),
        ...['external', 'sub'].map(type => `client.${type}`),
        ...['local', 'remote'].map(type => `ruleset.${type}`),
      ]
      expect(keys.filter(key => !Object.hasOwn(flat[locale], key))).toEqual([])
    })
    it(`${locale}: interpolates real import counters/errors while preserving literal JSON braces`, () => {
      const instance = createI18n({ legacy: false, locale, fallbackLocale: false, messages: { [locale]: messages } })
      const t = instance.global.t
      const conflict = t('rule.import.conflictDesc', { rules: 17, rulesets: 29 })
      expect(conflict).toContain('17')
      expect(conflict).toContain('29')
      for (const key of ['rule.import.errJsonParse', 'rule.import.errFetch']) expect(t(key, { message: 'UPSTREAM_MARKER' })).toContain('UPSTREAM_MARKER')
      expect(t('rule.import.jsonHint')).toContain('{...}')
      expect(t('rule.import.jsonHint')).not.toContain("{'{'}")
    })
  }
})

const expected = {
  en: {
    privateIp: 'Match non-public IPs', sourceIp: 'Match non-public source IPs', dns: 'DNS Server',
    headers: ['Headers', 'Key', 'Value'], session: 'Session Maximum Age', trafficAge: 'Traffic Statistics Retention',
    total: 'Total Traffic', insecure: 'Skip server certificate verification (insecure)',
    fragment: ['Fragment', 'Fragment Fallback Delay', 'Multiple records Fragmentation'],
    routes: ['Advertise Routes', 'Advertise Exit Node'], mux: ['Enable Multiplex', 'Enable Brutal'],
    authLen: 'Authenticated Length', secret: 'Access Key Secret', defaultOut: 'Default Outbound', yes: 'yes', no: 'no',
    verification: 'skips server certificate verification',
  },
  zhcn: {
    privateIp: '匹配非公开 IP', sourceIp: '匹配非公开源 IP', dns: 'DNS 服务器',
    headers: ['标头', '键', '值'], session: '会话超时时限', trafficAge: '流量统计保留时限',
    total: '累计流量', insecure: '跳过服务器证书验证（insecure）',
    fragment: ['TLS 分片', '分片回退延迟', 'TLS 记录分片'],
    routes: ['通告路由', '通告出口节点'], mux: ['启用多路复用', '启用 TCP Brutal'],
    authLen: '认证长度', secret: '访问密钥 Secret', defaultOut: '默认出站', yes: '是', no: '否',
    verification: '跳过服务器证书验证',
  },
  zhtw: {
    privateIp: '匹配非公開 IP', sourceIp: '匹配非公開來源 IP', dns: 'DNS 伺服器',
    headers: ['標頭', '鍵', '值'], session: '工作階段最長有效時間', trafficAge: '流量統計保留期限',
    total: '累計流量', insecure: '略過伺服器憑證驗證（insecure）',
    fragment: ['分段', '分段回退延遲', '多筆記錄分段'],
    routes: ['宣告路由', '宣告出口節點'], mux: ['啟用多路復用', '啟用 TCP Brutal'],
    authLen: '驗證長度', secret: '存取金鑰 Secret', defaultOut: '預設出站', yes: '是', no: '否',
    verification: '略過伺服器憑證驗證',
  },
}

for (const locale of Object.keys(catalogs) as (keyof typeof catalogs)[]) {
  describe(`round8 ${locale} real component translation regressions`, () => {
    beforeEach(() => {
      const id = localeIds[locale]
      if (locale === 'zhtw') composer.setLocaleMessage(id, zhtw) // Test-only registration, not a production change.
      composer.locale.value = id
    })
    const want = expected[locale]
    it('labels private-address routing predicates, not invalid-address detection', () => {
      const { host } = mount(RuleOptions, { rule: { ip_is_private: true, source_ip_is_private: true } })
      expect(labels(host)).toContain(want.privateIp)
      expect(labels(host)).toContain(want.sourceIp)
    })
    it('renders the real resolve-action DNS label instead of a raw missing key', () => {
      const { host } = mount(RuleModal, { visible: true, index: 0, data: JSON.stringify({ action: 'resolve', server: 'dns-local' }) })
      expect(labels(host)).toContain(want.dns)
      expect(labels(host)).not.toContain('basic.dns.server')
    })
    it('uses dictionary/header terminology rather than method, physical key or monetary value', () => {
      const { host } = mount(Headers, { data: { headers: { Host: 'example.test' } } })
      expect(host.textContent).toContain(want.headers[0])
      expect(labels(host)).toEqual(want.headers.slice(1))
      expect(composer.t('objects.listen')).toBe(locale === 'en' ? 'Listen' : locale === 'zhcn' ? '监听' : '監聽')
    })
    it('shows session duration in minutes and statistics retention in days', async () => {
      mocks.get.mockResolvedValue({ success: true, obj: {
        webListen: '', webDomain: '', webPort: '2095', webCertFile: '', webKeyFile: '', webPath: '/app/', webURI: '',
        sessionMaxAge: '60', trafficAge: '30', timeLocation: 'UTC',
        subListen: '', subPort: '2096', subPath: '/sub/', subDomain: '', subCertFile: '', subKeyFile: '',
        subUpdates: '12', subEncode: 'true', subShowInfo: 'false', subURI: '', subJsonExt: '', subClashExt: '',
      } })
      const { host } = mount(Settings)
      await flush()
      const fields = [...host.querySelectorAll('[data-label]')]
      expect(fields.find(node => node.getAttribute('data-label') === want.session)?.getAttribute('data-suffix')).toBe(locale === 'en' ? 'm' : '分')
      expect(fields.find(node => node.getAttribute('data-label') === want.trafficAge)?.getAttribute('data-suffix')).toBe(locale === 'en' ? 'd' : '天')
    })
    it('describes aggregate usage and renders the server identity keys on the dashboard', async () => {
      mocks.get.mockImplementation(async (url: string) => ({ success: true, obj: url === 'api/status' ? {
        db: { clientUp: 1024, clientDown: 1024 }, sys: { ipv4: ['192.0.2.10/24'], systemVersion: 'Mock OS' },
      } : [] }))
      const { host } = mount(Main)
      await flush()
      expect(host.querySelectorAll('.stat-content')[2].textContent).toContain(want.total)
      expect(host.textContent).toContain('192.0.2.10')
      expect(host.textContent).toContain(locale === 'en' ? 'Server IP' : locale === 'zhcn' ? '服务器 IP' : '伺服器 IP')
      expect(host.textContent).toContain(locale === 'en' ? 'System Version' : locale === 'zhcn' ? '系统版本' : '系統版本')
    })
    it('names certificate-verification bypass and distinct TLS fragmentation controls', () => {
      const { host } = mount(OutTLS, { outbound: { type: 'vless', tls: { enabled: true, fragment: true, record_fragment: true } } })
      expect(labels(host)).toContain(want.insecure)
      for (const label of want.fragment) expect(labels(host)).toContain(label)
      expect(new Set(want.fragment).size).toBe(3)
      expect(composer.t('tls.cs')).toBe(locale === 'en' ? 'Cipher suites' : locale === 'zhcn' ? '密码套件' : '加密套件')
    })
    it('preserves Tailscale route announcement and multiplex/Brutal protocol meaning', () => {
      const tailscale = mount(Tailscale, { data: { advertise_routes: ['192.0.2.0/24'], advertise_exit_node: true } })
      for (const label of want.routes) expect(labels(tailscale.host).some(value => value?.startsWith(label))).toBe(true)
      const mux = mount(Multiplex, { direction: 'out', data: { multiplex: { enabled: true } } })
      for (const label of want.mux) expect(labels(mux.host)).toContain(label)
      expect(composer.t('types.lb.defaultOut')).toBe(want.defaultOut)
      expect(composer.t('basic.routing.defaultOut')).toBe(want.defaultOut)
    })
    it('names authenticated_length and the AliDNS secret credential without implying ciphertext', () => {
      const vmess = mount(VMess, { data: { authenticated_length: true } })
      expect(labels(vmess.host)).toContain(want.authLen)
      expect(composer.t('types.vless.flow')).toBe(locale === 'en' ? 'Flow' : '流控')
      const acme = mount(Acme, { tls: { acme: { domain: [], dns01_challenge: { provider: 'alidns' } } } })
      expect(labels(acme.host)).toContain(want.secret)
    })
    it('supplies the Windows NIC warning to the actual TUN form', () => {
      const { host } = mount(Tun, { data: {} })
      expect(host.textContent).toContain('Windows')
      expect(host.textContent).toContain('Strict Route')
      expect(host.textContent).not.toContain('types.tun.windowsMultiNicHint')
    })
    it('renders boolean values as yes/no and warns that bootstrap insecure must be disabled for public use', () => {
      expect(composer.t('yes')).toBe(want.yes)
      expect(composer.t('no')).toBe(want.no)
      for (const key of ['inbounds.defaultProtocolNoticeDesc', 'client.defaultUserNoticeDesc', 'tls.selfSignedNotice']) {
        const text = composer.t(key)
        expect(text, key).toContain('insecure=true')
        expect(text, key).toContain('insecure=false')
        expect(text, key).toContain(want.verification)
      }
    })
  })
}
