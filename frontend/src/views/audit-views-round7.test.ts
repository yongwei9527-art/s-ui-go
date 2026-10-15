import { createApp, reactive, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ save: vi.fn(), error: vi.fn(), store: {} as any }))
vi.mock('@/store/modules/data', () => ({ default: () => mocks.store }))
vi.mock('@/locales', () => ({ i18n: { global: { t: (key: string) => key } } }))
vi.mock('notivue', () => ({ push: { error: mocks.error } }))
vi.mock('@/components/Dial.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Dns.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/DnsRule.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Rule.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/Ruleset.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/RuleImport.vue', () => ({ default: {} }))
vi.mock('@/layouts/modals/RulesetImport.vue', () => ({ default: {} }))
import Basics from './Basics.vue'
import Dns from './Dns.vue'
import Rules from './Rules.vue'

const apps: App[] = []
function mount(component: any) {
  const app = createApp({ ...component, render: () => null })
  apps.push(app)
  return (app.mount(document.createElement('div')).$ as any).setupState
}
const copy = (value: any) => JSON.parse(JSON.stringify(value))
function deferred() {
  let resolve!: (value: boolean) => void
  const promise = new Promise<boolean>(res => { resolve = res })
  return { promise, resolve }
}
beforeEach(() => {
  mocks.store = reactive({ lastLoad: '1', config: { log: { level: 'info' }, experimental: {}, dns: { servers: [], rules: [] }, route: { final: 'direct', rules: [], rule_set: [] } }, save: mocks.save })
  mocks.save.mockResolvedValue(false)
})
afterEach(() => { apps.splice(0).forEach(app => app.unmount()); vi.resetAllMocks() })

const configCases = [
  { name: 'Basics', component: Basics, section: 'log', field: 'level', untouched: 'experimental', external: { cache_file: { enabled: true, path: 'mock-cache.db' } } },
  { name: 'Dns', component: Dns, section: 'dns', field: 'final', untouched: 'route', external: { final: 'external', rules: [], rule_set: [] } },
  { name: 'Rules', component: Rules, section: 'route', field: 'final', untouched: 'dns', external: { final: 'external', rules: [], servers: [] } },
]
describe('round7 save acknowledgement rebasing', () => {
  it.each(configCases)('$name rebases untouched sections without losing edits made in flight', async ({ component, section, field, untouched, external }) => {
    const state = mount(component)
    state.appConfig[section][field] = 'submitted'
    mocks.store.config[untouched] = copy(external)
    const reply = deferred(); mocks.save.mockReturnValueOnce(reply.promise)
    const pending = state.saveConfig()
    const submitted = copy(mocks.save.mock.calls[0][2])
    state.appConfig[section][field] = 'later'
    mocks.store.config = submitted; reply.resolve(true); await pending
    expect(state.appConfig[section][field]).toBe('later')
    expect(state.appConfig[untouched]).toEqual(external)
    expect(state.stateChange).toBe(false) // These three pages use true for clean.
    mocks.save.mockImplementationOnce(async (_object, _action, data) => { mocks.store.config = copy(data); return true })
    await state.saveConfig()
    expect(mocks.save.mock.calls[1][2][untouched]).toEqual(external)
    expect(state.stateChange).toBe(true)
  })
})

const listCases = [
  { name: 'DNS servers', component: Dns, list: (s: any) => s.dns.servers, overlay: 'delDnsOverlay', show: 'showDnsModal', save: 'saveDnsModal', modal: 'dnsModal' },
  { name: 'DNS rules', component: Dns, list: (s: any) => s.dnsRules, overlay: 'delDnsRuleOverlay', show: 'showDnsRuleModal', save: 'saveDnsRuleModal', modal: 'dnsRuleModal' },
  { name: 'route rules', component: Rules, list: (s: any) => s.rules, overlay: 'delRuleOverlay', show: 'showRuleModal', save: 'saveRuleModal', modal: 'ruleModal' },
  { name: 'route rulesets', component: Rules, list: (s: any) => s.rulesets, overlay: 'delRulesetOverlay', show: 'showRulesetModal', save: 'saveRulesetModal', modal: 'rulesetModal' },
]
describe('round7 local list identity', () => {
  it.each(listCases)('$name clears confirmations when rows shift', ({ component, list, overlay }) => {
    const state = mount(component), rows = list(state)
    rows.push({ tag: 'a' }, { tag: 'b' }, { tag: 'c' })
    state[overlay][1] = true
    rows.splice(0, 1)
    expect(Object.values(state[overlay]).some(Boolean)).toBe(false)
  })
  it.each(listCases)('$name saves an editor to its original object after a reorder', ({ component, list, show, save }) => {
    const state = mount(component), rows = list(state)
    rows.push({ tag: 'a' }, { tag: 'b' }, { tag: 'c' })
    state[show](1); rows.splice(0, 1)
    state[save]({ tag: 'edited-b' })
    expect(rows.map((row: any) => row.tag)).toEqual(['edited-b', 'c'])
  })
  it.each(listCases)('$name never overwrites a neighbor after the edited object is removed', ({ component, list, show, save, modal }) => {
    const state = mount(component), rows = list(state)
    rows.push({ tag: 'a' }, { tag: 'b' }, { tag: 'c' })
    state[show](1); rows.splice(1, 1)
    state[save]({ tag: 'edited-b' })
    expect(rows.map((row: any) => row.tag)).toEqual(['a', 'c'])
    expect(state[modal].visible).toBe(false)
  })
  it.each([Dns, Rules])('clears rule confirmations when drag/drop reorders rows: %s', component => {
    const state = mount(component), rows = component === Dns ? state.dnsRules : state.rules
    const overlay = component === Dns ? 'delDnsRuleOverlay' : 'delRuleOverlay'
    rows.push({ action: 'a' }, { action: 'b' }, { action: 'c' })
    state[overlay][1] = true
    state.onDragStart(2); state.onDrop(0)
    expect(rows.map((row: any) => row.action)).toEqual(['c', 'a', 'b'])
    expect(Object.values(state[overlay]).some(Boolean)).toBe(false)
  })
})

// If a previously untouched section changes externally before submission and is
// then edited locally in flight, retain an honest baseline so retry must conflict.
describe('round7 edits to externally changed sections', () => {
  it.each(configCases.slice(0, 2))('$name conflicts instead of silently erasing an unseen external change', async ({ component, section, field, untouched, external }) => {
    const state = mount(component)
    state.appConfig[section][field] = 'submitted'
    mocks.store.config[untouched] = copy(external)
    const reply = deferred(); mocks.save.mockReturnValueOnce(reply.promise)
    const pending = state.saveConfig(), submitted = copy(mocks.save.mock.calls[0][2])
    state.appConfig[untouched].localEdit = 'later'
    mocks.store.config = submitted; reply.resolve(true); await pending
    await state.saveConfig()
    expect(mocks.save).toHaveBeenCalledTimes(1)
    expect(mocks.error).toHaveBeenCalled()
    expect(mocks.store.config[untouched]).toEqual(external)
    expect(state.appConfig[untouched].localEdit).toBe('later')
  })
})

describe('round7 editor identity boundaries', () => {
  it.each(listCases)('$name still adds and edits ordinary rows', ({ component, list, show, save }) => {
    const state = mount(component), rows = list(state)
    state[show](-1); state[save]({ tag: 'new' })
    expect(rows.map((row: any) => row.tag)).toEqual(['new'])
    state[show](0); state[save]({ tag: 'edited' })
    expect(rows.map((row: any) => row.tag)).toEqual(['edited'])
  })
  it.each(listCases)('$name rejects a stale editor after a whole-list replacement', ({ component, list, show, save }) => {
    const state = mount(component), rows = list(state)
    rows.push({ tag: 'original' })
    state[show](0); rows.splice(0, 1, { tag: 'external' })
    state[save]({ tag: 'stale' })
    expect(rows.map((row: any) => row.tag)).toEqual(['external'])
  })
})
