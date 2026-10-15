import { readFileSync } from 'node:fs'
import ts from 'typescript'
import { baseCompile } from '@intlify/message-compiler'
import { createI18n } from 'vue-i18n'
import { createApp, h, nextTick, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import en from './en'
import fa from './fa'
import ru from './ru'
import viMessages from './vi'
import RuleImport from '../layouts/modals/RuleImport.vue'

// Never import app i18n/router/store: secondary catalogs are not registered there.
const catalogs = [{ locale: 'fa', messages: fa }, { locale: 'ru', messages: ru }, { locale: 'vi', messages: viMessages }]
const readSource = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')
type CatalogNode = string | { [key: string]: CatalogNode }
function flatten(node: CatalogNode, path = '', result: Record<string, string> = {}) {
  if (typeof node === 'string') result[path] = node
  else for (const [key, value] of Object.entries(node)) flatten(value, path ? `${path}.${key}` : key, result)
  return result
}
const english = flatten(en)
// Deliberately retain English fallback instead of adding 28 cosmetic translations.
const dashboardFallback = Object.keys(english).filter(key => key.startsWith('main.dashboard.')).sort()
function inspectSource(source: string) {
  const file = ts.createSourceFile('locale.ts', source, ts.ScriptTarget.Latest, true)
  const diagnostics = ts.transpileModule(source, { reportDiagnostics: true, compilerOptions: { target: ts.ScriptTarget.ESNext } }).diagnostics ?? []
  const exported = file.statements.find(ts.isExportAssignment)
  const duplicates: string[] = [], invalid: string[] = [], shape: Record<string, string> = {}
  const walk = (node: ts.Expression, path: string) => {
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
      shape[path] = 'string'; if (!node.text.trim()) invalid.push(path); return
    }
    if (!ts.isObjectLiteralExpression(node)) { invalid.push(path); return }
    shape[path] = 'object'
    const seen = new Set<string>()
    for (const property of node.properties) {
      if (!ts.isPropertyAssignment(property) || ts.isComputedPropertyName(property.name)) { invalid.push(`${path}.<non-static-property>`); continue }
      const name = property.name
      const key = ts.isIdentifier(name) || ts.isStringLiteral(name) || ts.isNumericLiteral(name) ? name.text : ''
      const child = path ? `${path}.${key}` : key
      if (seen.has(key)) duplicates.push(child)
      seen.add(key); walk(property.initializer, child)
    }
  }
  if (exported) walk(exported.expression, '')
  else invalid.push('<missing-default-export>')
  return { duplicates, invalid, shape, diagnostics }
}
function parameters(message: string) {
  const errors: string[] = []
  const ast = baseCompile(message, { onError: error => errors.push(error.message) }).ast
  const names = new Set<string>(), indexes = new Set<number>()
  const walk = (value: unknown) => {
    if (!value || typeof value !== 'object') return
    const node = value as Record<string, unknown>
    if (node.type === 4 && typeof node.key === 'string') names.add(node.key)
    if (node.type === 5 && typeof node.index === 'number') indexes.add(node.index)
    for (const child of Object.values(node)) {
      if (Array.isArray(child)) child.forEach(walk)
      else if (child && typeof child === 'object') walk(child)
    }
  }
  walk(ast)
  return { errors, names: [...names].sort(), indexes: [...indexes].sort() }
}
function isolatedI18n(locale: string, messages: typeof fa | typeof ru | typeof viMessages) {
  return createI18n({ legacy: false, locale, fallbackLocale: 'en', messages: { en, [locale]: messages }, missingWarn: false, fallbackWarn: false, warnHtmlMessage: false })
}
const apps: App[] = []
afterEach(() => { apps.splice(0).forEach(app => app.unmount()); vi.unstubAllGlobals() })
it('detects sibling duplicates before JavaScript silently overwrites them', () => {
  const result = inspectSource('export default { a: { key: "first", "key": "second" }, b: { key: "valid elsewhere" } }')
  expect(result.duplicates).toEqual(['a.key']); expect(result.invalid).toEqual([])
})
for (const { locale, messages } of catalogs) {
  describe(`round8 isolated ${locale} catalog`, () => {
    const flat = flatten(messages)
    it('has static nonempty strings, no duplicates, and English-compatible structure', () => {
      const source = inspectSource(readSource(`./${locale}.ts`))
      expect(source.diagnostics).toEqual([]); expect(source.duplicates).toEqual([]); expect(source.invalid).toEqual([])
      const expected = Object.fromEntries(Object.entries(inspectSource(readSource('./en.ts')).shape).filter(([key]) => key !== 'main.dashboard' && !key.startsWith('main.dashboard.')))
      expect(source.shape).toEqual(expected)
    })
    it('resolves all English keys with only the explicit dashboard fallback gap', () => {
      expect(Object.keys(english).filter(key => !(key in flat)).sort()).toEqual(dashboardFallback)
      expect(Object.keys(flat).filter(key => !(key in english))).toEqual([])
      const { global } = isolatedI18n(locale, messages)
      for (const key of Object.keys(english)) {
        const translated = global.t(key, { rules: 731, rulesets: 947, message: 'audit-message' })
        expect(translated, key).not.toBe(key); expect(translated.trim(), key).not.toBe('')
      }
      for (const key of dashboardFallback) expect(global.t(key)).toBe(english[key])
    })
    it('compiles every message and preserves parameters and literal JSON braces', () => {
      for (const [key, value] of Object.entries(flat)) {
        const actual = parameters(value), baseline = parameters(english[key])
        expect(actual.errors, key).toEqual([])
        expect(actual.names, key).toEqual(baseline.names); expect(actual.indexes, key).toEqual(baseline.indexes)
      }
      const { global } = isolatedI18n(locale, messages)
      expect(global.t('rule.import.jsonHint')).toContain('{...}')
      const conflict = global.t('rule.import.conflictDesc', { rules: 731, rulesets: 947 })
      expect(conflict).toContain('731'); expect(conflict).toContain('947'); expect(conflict).not.toMatch(/\{(?:rules|rulesets)\}/)
    })
    it('locally resolves router names and dynamic action/object/theme labels', () => {
      const routes = [...readSource('../router/index.ts').matchAll(/name:\s*['"](pages\.[^'"]+)['"]/g)].map(match => match[1])
      expect(routes.length).toBeGreaterThan(10)
      const { global } = isolatedI18n(locale, messages)
      const keys = [...routes, ...Object.keys(en.actions).map(key => `actions.${key}`), ...Object.keys(en.objects).map(key => `objects.${key}`), 'theme.light', 'theme.dark', 'theme.system']
      for (const key of keys) expect(global.te(key, locale), key).toBe(true)
    })
    it('limits insecure TLS to bootstrap and requires verified regenerated production exports', () => {
      for (const value of [messages.inbounds.defaultProtocolNoticeDesc, messages.client.defaultUserNoticeDesc, messages.tls.selfSignedNotice]) {
        expect(value).toMatch(/bootstrap/i); expect(value).toContain('insecure=true'); expect(value).toContain('insecure=false')
        expect(value).toMatch(/certificate verification/i); expect(value).toMatch(/production/i); expect(value).toMatch(/regenerate subscriptions\/links/i)
        expect(value).not.toContain('Clients need insecure=true')
      }
      expect(messages.inbounds.defaultProtocolNoticeDesc).toContain('UDP'); expect(messages.inbounds.defaultProtocolNoticeDesc).toContain('Mixed/SOCKS/HTTP')
      expect(messages.tls.realityNotice).toContain('public key, short_id, and fingerprint')
      expect(messages.inbounds.hysteria2Notice).toContain('UDP'); expect(messages.inbounds.tuicNotice).toContain('auth_timeout 3s, heartbeat 10s')
      expect(messages.types.tun).toHaveProperty('windowsMultiNicHint', expect.stringContaining('Strict Route'))
      expect(messages.setting.clashSubCompatibilityNotice).toContain('Naive'); expect(messages.setting.clashSubCompatibilityNotice).toContain('ShadowTLS')
    })
    it('uses real RuleImport methods with plain-text localized parser/fetch errors', async () => {
      const source = readSource('../layouts/modals/RuleImport.vue')
      expect(source).toMatch(/<v-alert\b[^>]*v-if="error"[^>]*>\s*\{\{\s*error\s*\}\}/); expect(source).not.toContain('v-html')
      // Exercise the real methods with the same text sink, not a full Vuetify mount.
      const app = createApp({ ...RuleImport, render(this: { error: string }) { return h('p', this.error) } }, { visible: true, existingRulesCount: 0, existingRulesetsCount: 0, existingRulesetTags: [] })
      const i18n = isolatedI18n(locale, messages); app.use(i18n); apps.push(app)
      const container = document.createElement('div')
      const state = app.mount(container) as unknown as { rawJson: string; error: string; fetchUrl: string; parseJson(): void; fetchFromUrl(): Promise<void> }
      state.rawJson = '{'; state.parseJson(); await nextTick()
      expect(state.error).not.toBe(''); expect(state.error).not.toMatch(/<\/?p\b|dir=/); expect(container.textContent).toBe(state.error)
      const message = 'audit-fetch-error <img src=x onerror=alert(1)>'
      vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error(message)))
      state.fetchUrl = 'https://audit.invalid/rules.json'; await state.fetchFromUrl(); await nextTick()
      expect(state.error).toBe(i18n.global.t('rule.import.errFetch', { message })); expect(state.error).not.toMatch(/<\/?p\b|dir=/)
      expect(container.textContent).toBe(state.error); expect(container.querySelector('img')).toBeNull()
      expect(flat['rule.import.errJsonParse']).not.toMatch(/<[^>]+>/); expect(flat['rule.import.errFetch']).not.toMatch(/<[^>]+>/)
    })
  })
}
const correctedLabels = {
  fa: {
    'basic.dns.server': 'سرور DNS',
    'rule.privateIp': 'محدوده‌های IP خصوصی', 'rule.srcPrivateIp': 'آدرس‌های IP مبدا خصوصی',
    'tls.insecure': 'نادیده‌گرفتن اعتبارسنجی گواهی (insecure)',
  },
  ru: {
    'basic.dns.server': 'DNS-сервер',
    'pages.services': 'Сервисы', 'objects.service': 'Сервис', 'rule.privateIp': 'Частные диапазоны IP', 'rule.srcPrivateIp': 'Частные исходные IP',
    'basic.routing.defaultRm': 'Метка маршрутизации по умолчанию', 'tls.insecure': 'Отключить проверку сертификата (insecure)',
  },
  vi: {
    'basic.dns.server': 'Máy chủ DNS',
    editor: 'Trình chỉnh sửa', 'error.invalidData': 'Dữ liệu không hợp lệ',
    'theme.light': 'Sáng', 'theme.dark': 'Tối', 'theme.system': 'Hệ thống',
    'pages.services': 'Dịch vụ', 'pages.endpoints': 'Điểm cuối', 'main.tiles': 'Ô giám sát',
    'setting.mixedPort': 'Cổng đầu vào Mixed', 'setting.tun': 'Đầu vào TUN',
    'rule.privateIp': 'Dải IP riêng', 'rule.srcPrivateIp': 'IP nguồn riêng', 'rule.udpDisableDomainUnmapping': 'Tắt bỏ ánh xạ tên miền UDP',
    'rule.noDrop': 'Không loại bỏ', 'rule.strategy': 'Chiến lược',
    'dns.cacheCapacity': 'Dung lượng bộ nhớ đệm', 'dns.disableExpire': 'Tắt hết hạn bộ nhớ đệm', 'dns.independentCache': 'Bộ nhớ đệm độc lập', 'dns.reverseMapping': 'Ánh xạ ngược',
    'dns.rule.action.rcode': 'Mã phản hồi', 'tls.fragment': 'Phân mảnh', 'tls.fragmentDelay': 'Độ trễ dự phòng phân mảnh', 'tls.recordFragment': 'Phân mảnh nhiều bản ghi',
    'tls.insecure': 'Bỏ qua xác minh chứng chỉ (insecure)',
  },
}
for (const { locale, messages } of catalogs) {
  it(`${locale} labels match the actual routing, DNS, TLS, and menu controls`, () => {
    const flat = flatten(messages)
    for (const [key, value] of Object.entries(correctedLabels[locale as keyof typeof correctedLabels])) expect(flat[key], key).toBe(value)
  })
}
