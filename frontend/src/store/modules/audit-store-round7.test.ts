import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), error: vi.fn(), success: vi.fn() }))
vi.mock('@/plugins/httputil', () => ({ default: { get: mocks.get, post: mocks.post } }))
vi.mock('@/locales', () => ({ i18n: { global: { t: (key: string) => key } } }))
vi.mock('notivue', () => ({ push: { error: mocks.error, success: mocks.success } }))
import Data from './data'

function deferred() {
  let resolve!: (value: any) => void, reject!: (reason: any) => void
  const promise = new Promise<any>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
const ok = (obj: any) => ({ success: true, obj })
const config = (final: string) => ({ route: { final } })
const full = (revision: string, final: string) => ok({ lastUpdate: revision, config: config(final), onlines: { inbound: [], outbound: [], user: [] } })
beforeEach(() => {
  localStorage.clear(); setActivePinia(createPinia())
  mocks.get.mockReset(); mocks.post.mockReset(); mocks.error.mockReset(); mocks.success.mockReset()
  mocks.get.mockResolvedValue({ success: false }); mocks.post.mockResolvedValue({ success: false })
})
afterEach(() => vi.restoreAllMocks())

describe('store round7 read/write epochs', () => {
  it.each(['before', 'during'])('rejects a full read started %s a successful write', async timing => {
    const state = Data(); state.lastLoad = '10'; state.config = config('initial')
    const read = deferred(), write = deferred()
    mocks.get.mockReturnValue(read.promise); mocks.post.mockReturnValue(write.promise)
    let pendingRead: Promise<void>, pendingWrite: Promise<boolean>
    if (timing === 'before') { pendingRead = state.loadData(); pendingWrite = state.save('config', 'set', config('saved')) }
    else { pendingWrite = state.save('config', 'set', config('saved')); pendingRead = state.loadData() }
    write.resolve(ok({ config: config('saved') })); expect(await pendingWrite).toBe(true)
    read.resolve(full('11', 'stale')); await pendingRead
    expect(state.config.route.final).toBe('saved'); expect(state.lastLoad).toBe('10')
  })
  it('does not apply a read that finishes while a write is still pending', async () => {
    const state = Data(); state.config = config('initial')
    const write = deferred(); mocks.post.mockReturnValue(write.promise)
    const saving = state.save('config', 'set', config('saved'))
    mocks.get.mockResolvedValue(full('12', 'stale')); await state.loadData()
    expect(state.config.route.final).toBe('initial'); expect(state.lastLoad).toBe('')
    write.resolve(ok({ config: config('saved') })); await saving
    expect(state.config.route.final).toBe('saved')
  })
  it('forces a fresh full poll after a save, then resumes lossless lu polling', async () => {
    const state = Data(); state.lastLoad = '10'
    mocks.post.mockResolvedValue(ok({ config: config('saved') })); await state.save('config', 'set', config('saved'))
    mocks.get.mockResolvedValue(full('11', 'fresh')); await state.loadData()
    expect(mocks.get).toHaveBeenLastCalledWith('api/load', {})
    expect(state.lastLoad).toBe('11'); expect(state.config.route.final).toBe('fresh')
    await state.loadData(); expect(mocks.get).toHaveBeenLastCalledWith('api/load', { lu: '11' })
  })
  it.each(['false', 'throw'])('releases write barriers after a %s save and allows fresh reads', async mode => {
    const state = Data()
    if (mode === 'throw') { mocks.post.mockRejectedValueOnce(new Error('mock offline')); await expect(state.save('config', 'set', {})).rejects.toThrow('mock offline') }
    else expect(await state.save('config', 'set', {})).toBe(false)
    mocks.get.mockResolvedValue(full('12', 'fresh')); await state.loadData()
    expect(state.config.route.final).toBe('fresh'); expect(state.lastLoad).toBe('12')
  })
  it('invalidates pre-existing reads after a public setNewData without fabricating a revision', async () => {
    const state = Data(); state.lastLoad = '9'
    const reply = deferred(); mocks.get.mockReturnValue(reply.promise)
    const read = state.loadData(); state.setNewData({ config: config('local') })
    reply.resolve(full('10', 'stale')); await read
    expect(state.config.route.final).toBe('local'); expect(state.lastLoad).toBe('9')
  })
  it('keeps unrelated fields from a poll while refusing fields affected by a write', async () => {
    const state = Data(); state.lastLoad = '10'; const reply = deferred(); mocks.get.mockReturnValueOnce(reply.promise)
    const read = state.loadData()
    mocks.post.mockResolvedValue(ok({ clients: [{ id: 1, name: 'saved' }], inbounds: [] }))
    await state.save('clients', 'edit', { id: 1 })
    reply.resolve(ok({ lastUpdate: '11', config: config('independent'), clients: [{ id: 1, name: 'stale' }] })); await read
    expect(state.config.route.final).toBe('independent'); expect(state.clients[0].name).toBe('saved'); expect(state.lastLoad).toBe('10')
    mocks.get.mockResolvedValue(full('12', 'fresh')); await state.loadData()
    expect(mocks.get).toHaveBeenLastCalledWith('api/load', {})
  })
})

const resources = ['config', 'clients', 'inbounds', 'outbounds', 'endpoints', 'services', 'tls'] as const
describe('store round7 independent partial loads', () => {
  it.each(resources)('applies a %s partial payload without requiring config or clearing absent fields', async resource => {
    const state = Data(); state.lastLoad = '9'; state.onlines.user = ['keep']
    const value = resource === 'config' ? config('partial') : [{ id: 1, tag: 'partial', name: 'partial' }]
    mocks.get.mockResolvedValue(ok({ [resource]: value })); await state.loadData(resource)
    expect(mocks.get).toHaveBeenCalledWith('api/' + resource, {})
    expect((state as any)[resource === 'tls' ? 'tlsConfigs' : resource]).toEqual(value)
    expect(state.onlines.user).toEqual(['keep']); expect(state.lastLoad).toBe('9')
  })
  it.each([true, false])('keeps both config and clients when independent loads finish in either order (%s)', async configFirst => {
    const state = Data(), a = deferred(), b = deferred()
    mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const c = state.loadData('config'), u = state.loadData('clients')
    const finishConfig = async () => { a.resolve(ok({ config: config('valid') })); await c }
    const finishClients = async () => { b.resolve(ok({ clients: [{ id: 2, name: 'valid' }] })); await u }
    if (configFirst) { await finishConfig(); await finishClients() } else { await finishClients(); await finishConfig() }
    expect(state.config.route.final).toBe('valid'); expect(state.clients[0].id).toBe(2)
  })
  it('rejects an older same-field partial after a newer partial was applied', async () => {
    const state = Data(), a = deferred(), b = deferred()
    mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const old = state.loadData('clients'), latest = state.loadData('clients')
    b.resolve(ok({ clients: [{ name: 'latest' }] })); await latest
    a.resolve(ok({ clients: [{ name: 'old' }] })); await old
    expect(state.clients[0].name).toBe('latest')
  })
  it('does not invalidate an independent config partial when clients are saved', async () => {
    const state = Data(), a = deferred(); mocks.get.mockReturnValue(a.promise)
    const pending = state.loadData('config')
    mocks.post.mockResolvedValue(ok({ clients: [{ name: 'saved' }] })); await state.save('clients', 'new', {})
    a.resolve(ok({ config: config('valid') })); await pending
    expect(state.config.route.final).toBe('valid'); expect(state.clients[0].name).toBe('saved')
  })
  it('applies an api/load partial object even when it omits config', async () => {
    const state = Data(); state.config = config('keep'); state.onlines.user = ['keep']
    mocks.get.mockResolvedValue(ok({ lastUpdate: '12', clients: [{ name: 'loaded' }] })); await state.loadData()
    expect(state.clients[0].name).toBe('loaded'); expect(state.config.route.final).toBe('keep'); expect(state.onlines.user).toEqual(['keep'])
  })
})

describe('store round7 revision and request ordering', () => {
  it.each([['10', '11'], ['9007199254740992', '9007199254740993'], ['9999999999999999999', '10000000000000000000']])('does not regress decimal revision %s after %s', async (oldRevision, newRevision) => {
    const state = Data(), a = deferred(), b = deferred()
    mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const old = state.loadData(), latest = state.loadData()
    b.resolve(full(newRevision, 'latest')); await latest
    a.resolve({ ...full(oldRevision, 'old'), obj: { ...full(oldRevision, 'old').obj, lastLog: 'stale error', onlines: { user: ['stale'] } } }); await old
    expect(state.lastLoad).toBe(newRevision); expect(state.config.route.final).toBe('latest'); expect(state.onlines.user).toEqual([]); expect(mocks.error).not.toHaveBeenCalled()
  })
  it('rejects a lower server revision even when that request started later', async () => {
    const state = Data(); mocks.get.mockResolvedValueOnce(full('11', 'latest')).mockResolvedValueOnce(full('10', 'old'))
    await state.loadData(); await state.loadData()
    expect(state.lastLoad).toBe('11'); expect(state.config.route.final).toBe('latest')
  })
  it('uses request order to break ties at the same revision', async () => {
    const state = Data(), a = deferred(), b = deferred(); mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const old = state.loadData(), latest = state.loadData()
    b.resolve(full('11', 'latest')); await latest; a.resolve(full('11', 'old')); await old
    expect(state.config.route.final).toBe('latest')
  })
  it('can accept a genuinely higher revision from an earlier-issued request', async () => {
    const state = Data(), a = deferred(), b = deferred(); mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const earlier = state.loadData(), later = state.loadData()
    b.resolve(full('11', 'eleven')); await later; a.resolve(full('12', 'twelve')); await earlier
    expect(state.config.route.final).toBe('twelve'); expect(state.lastLoad).toBe('12')
  })
  it('does not advance the global lu from a resource-only revision', async () => {
    const state = Data(); state.lastLoad = '9'
    mocks.get.mockResolvedValueOnce(ok({ lastUpdate: '12', config: config('twelve') }))
    await state.loadData('config'); expect(state.lastLoad).toBe('9')
    mocks.get.mockResolvedValueOnce(full('11', 'stale')); await state.loadData()
    expect(state.config.route.final).toBe('twelve'); expect(state.lastLoad).toBe('9')
    mocks.get.mockResolvedValueOnce(full('12', 'twelve')); await state.loadData()
    expect(mocks.get).toHaveBeenLastCalledWith('api/load', {}); expect(state.lastLoad).toBe('12')
  })
  it('keeps setLastLoadRevision monotonic without unsafe numeric conversion', () => {
    const state = Data(); state.setLastLoadRevision('9007199254740993')
    for (const revision of ['9007199254740992', '0', 12, Number.MAX_SAFE_INTEGER + 1, 'invalid', '-1', '01']) state.setLastLoadRevision(revision)
    expect(state.lastLoad).toBe('9007199254740993')
  })
})

describe('store round7 save compatibility', () => {
  it('preserves save payload, boolean return, related resource updates and notifications', async () => {
    const state = Data(), data = { id: 1, name: 'mock' }
    mocks.post.mockResolvedValue(ok({ clients: [data], inbounds: [{ id: 2, users: ['mock'] }], enableTraffic: false, subURI: '' }))
    expect(await state.save('clients', 'edit', data, [2, 3])).toBe(true)
    expect(mocks.post).toHaveBeenCalledWith('api/save', { object: 'clients', action: 'edit', data: JSON.stringify(data, null, 2), initUsers: '2,3' })
    expect(state.clients[0].name).toBe('mock'); expect(state.inbounds[0].users).toEqual(['mock']); expect(mocks.success).toHaveBeenCalledTimes(1)
  })
  it('rejects a late older save acknowledgement for the same resource', async () => {
    const state = Data(), a = deferred(), b = deferred(); mocks.post.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const old = state.save('config', 'set', config('old')), latest = state.save('config', 'set', config('latest'))
    b.resolve(ok({ config: config('latest') })); expect(await latest).toBe(true)
    a.resolve(ok({ config: config('old') })); expect(await old).toBe(true)
    expect(state.config.route.final).toBe('latest')
  })
  it.each([true, false])('does not drop disjoint successful save responses (%s)', async configFirst => {
    const state = Data(), a = deferred(), b = deferred(); mocks.post.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const c = state.save('config', 'set', config('saved')), u = state.save('clients', 'new', {})
    const finishConfig = async () => { a.resolve(ok({ config: config('saved') })); await c }
    const finishClients = async () => { b.resolve(ok({ clients: [{ name: 'saved' }] })); await u }
    if (configFirst) { await finishConfig(); await finishClients() } else { await finishClients(); await finishConfig() }
    expect(state.config.route.final).toBe('saved'); expect(state.clients[0].name).toBe('saved')
  })
  it('keeps existing cache on a successful empty acknowledgement', async () => {
    const state = Data(); state.config = config('keep'); mocks.post.mockResolvedValue(ok(null))
    expect(await state.save('config', 'set', {})).toBe(true); expect(state.config.route.final).toBe('keep')
  })
  it('applies null clears and explicit false/empty without clearing absent resources', () => {
    const state = Data(); state.config = config('keep'); state.clients = [{ name: 'old' }]
    state.setNewData({ clients: null, tls: null, subURI: '', enableTraffic: false })
    expect(state.clients).toEqual([]); expect(state.tlsConfigs).toEqual([]); expect(state.config.route.final).toBe('keep'); expect(state.subURI).toBe(''); expect(state.enableTraffic).toBe(false)
  })
})

const details = [
  { name: 'clients', call: (s: ReturnType<typeof Data>, id: number) => s.loadClients(id), data: (id: number) => ({ clients: [{ id, name: 'client-' + id }] }), empty: {} },
  { name: 'inbounds', call: (s: ReturnType<typeof Data>, id: number) => s.loadInbounds([id]), data: (id: number) => ({ inbounds: [{ id, tag: 'inbound-' + id }] }), empty: [] },
]
describe('store round7 detail read identities', () => {
  it.each(details)('$name rejects older detail results for the same query', async ({ call, data, empty }) => {
    const state = Data(), a = deferred(), b = deferred(); mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const old = call(state, 1), latest = call(state, 1)
    b.resolve(ok(data(1))); expect(await latest).not.toEqual(empty)
    a.resolve(ok(data(1))); expect(await old).toEqual(empty)
  })
  it.each(details)('$name keeps simultaneous detail loads for different IDs', async ({ call, data, empty }) => {
    const state = Data(), a = deferred(), b = deferred(); mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const one = call(state, 1), two = call(state, 2)
    b.resolve(ok(data(2))); expect(await two).not.toEqual(empty)
    a.resolve(ok(data(1))); expect(await one).not.toEqual(empty)
  })
  it.each(details)('$name rejects a detail response after a related TLS write', async ({ call, data, empty }) => {
    const state = Data(), reply = deferred(); mocks.get.mockReturnValue(reply.promise)
    const pending = call(state, 1); mocks.post.mockResolvedValue(ok({ tls: [] })); await state.save('tls', 'edit', {})
    reply.resolve(ok(data(1))); expect(await pending).toEqual(empty)
  })
  it.each(details)('$name rejects a detail read started during a write even if it completes after the acknowledgement', async ({ name, call, data, empty }) => {
    const state = Data(), read = deferred(), write = deferred(); mocks.get.mockReturnValue(read.promise); mocks.post.mockReturnValue(write.promise)
    const saving = state.save(name, 'edit', {}), pending = call(state, 1)
    write.resolve(ok({ [name]: [] })); await saving; read.resolve(ok(data(1)))
    expect(await pending).toEqual(empty)
  })
  it.each(details)('$name rejects an in-flight detail after a newer cache snapshot is applied', async ({ name, call, data, empty }) => {
    const state = Data(), reply = deferred(); mocks.get.mockReturnValueOnce(reply.promise)
    const pending = call(state, 1); mocks.get.mockResolvedValueOnce(ok({ [name]: [{ id: 1 }] })); await state.loadData(name as 'clients' | 'inbounds')
    reply.resolve(ok(data(1))); expect(await pending).toEqual(empty)
  })
  it.each(details)('$name returns its existing empty shape for missing/malformed data and recovers on retry', async ({ name, call, data, empty }) => {
    const state = Data()
    for (const obj of [null, {}, { [name]: null }, { [name]: {} }]) {
      mocks.get.mockResolvedValueOnce(ok(obj)); expect(await call(state, 1)).toEqual(empty)
    }
    mocks.get.mockResolvedValueOnce(ok(data(1))); expect(await call(state, 1)).not.toEqual(empty)
  })
  it('retains empty-ID request options and existing return shapes', async () => {
    const state = Data(); mocks.get.mockResolvedValueOnce(ok({ inbounds: [] })).mockResolvedValueOnce(ok({ clients: [] }))
    expect(await state.loadInbounds([])).toEqual([]); expect(mocks.get).toHaveBeenNthCalledWith(1, 'api/inbounds', {})
    expect(await state.loadClients(0)).toEqual({}); expect(mocks.get).toHaveBeenNthCalledWith(2, 'api/clients', {})
  })
  it('isolates request metadata across Pinia store instances', async () => {
    const one = Data(), reply = deferred(); mocks.get.mockReturnValueOnce(reply.promise); const pending = one.loadData()
    setActivePinia(createPinia()); const two = Data(); mocks.post.mockResolvedValue(ok({ config: config('second') })); await two.save('config', 'set', {})
    reply.resolve(full('10', 'first')); await pending
    expect(one.config.route.final).toBe('first'); expect(two.config.route.final).toBe('second')
  })
})

describe('store round7 final integration edge cases', () => {
  it('does not discard useful same-revision fields merely because a newer no-change poll completed', async () => {
    const state = Data(), a = deferred(), b = deferred(); mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const withData = state.loadData(), noChange = state.loadData()
    b.resolve(ok({ lastUpdate: '11', onlines: { inbound: [], outbound: [], user: ['newer'] } })); await noChange
    a.resolve(full('11', 'useful')); await withData
    expect(state.config.route.final).toBe('useful'); expect(state.lastLoad).toBe('11'); expect(state.onlines.user).toEqual(['newer'])
  })
  it('merges distinct same-revision resource partials in reverse order', async () => {
    const state = Data(), a = deferred(), b = deferred(); mocks.get.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const c = state.loadData('config'), u = state.loadData('clients')
    b.resolve(ok({ lastUpdate: '11', clients: [{ name: 'valid' }] })); await u
    a.resolve(ok({ lastUpdate: '11', config: config('valid') })); await c
    expect(state.config.route.final).toBe('valid'); expect(state.clients[0].name).toBe('valid'); expect(state.lastLoad).toBe('')
  })
  it('does not lose an older valid resource read just because a later read failed', async () => {
    const state = Data(), a = deferred(); mocks.get.mockReturnValueOnce(a.promise).mockResolvedValueOnce({ success: false })
    const first = state.loadData('clients'); await state.loadData('clients')
    a.resolve(ok({ clients: [{ name: 'valid' }] })); await first
    expect(state.clients[0].name).toBe('valid')
  })
  it.each(['false', 'throw'])('keeps a disjoint successful ACK and clears all barriers after concurrent %s', async mode => {
    const state = Data(), a = deferred(), b = deferred(); mocks.post.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const failed = state.save('config', 'set', {}), saved = state.save('clients', 'new', {})
    const rejected = mode === 'throw' ? expect(failed).rejects.toThrow('mock offline') : undefined
    b.resolve(ok({ clients: [{ name: 'saved' }] })); expect(await saved).toBe(true)
    if (mode === 'throw') { a.reject(new Error('mock offline')); await rejected }
    else { a.resolve({ success: false }); expect(await failed).toBe(false) }
    expect(state.clients[0].name).toBe('saved')
    mocks.get.mockResolvedValue(ok({ lastUpdate: '12', config: config('fresh'), clients: [{ name: 'fresh' }], inbounds: [] }))
    await state.loadData()
    expect(state.config.route.final).toBe('fresh'); expect(state.clients[0].name).toBe('fresh'); expect(state.lastLoad).toBe('12')
  })
  it.each(['false', 'throw'])('releases overlapping write counters after a late %s response', async mode => {
    const state = Data(), a = deferred(), b = deferred(); mocks.post.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)
    const old = state.save('config', 'set', {}), latest = state.save('config', 'set', {})
    const rejected = mode === 'throw' ? expect(old).rejects.toThrow('mock offline') : undefined
    b.resolve(ok({ config: config('saved') })); await latest
    if (mode === 'throw') { a.reject(new Error('mock offline')); await rejected }
    else { a.resolve({ success: false }); expect(await old).toBe(false) }
    mocks.get.mockResolvedValue(full('12', 'fresh')); await state.loadData()
    expect(state.config.route.final).toBe('fresh'); expect(state.lastLoad).toBe('12')
  })
  it('keeps the old cursor after a rejected high-revision read, even when the save ACK has no revision', async () => {
    const state = Data(); state.lastLoad = '10'
    const read = deferred(), write = deferred(); mocks.post.mockReturnValue(write.promise); mocks.get.mockReturnValueOnce(read.promise)
    const saving = state.save('clients', 'edit', {}), during = state.loadData()
    write.resolve(ok({ clients: [{ name: 'saved' }], inbounds: [] })); await saving
    read.resolve(ok({ lastUpdate: '12', clients: [{ name: 'stale' }], config: config('independent') })); await during
    expect(state.clients[0].name).toBe('saved'); expect(state.lastLoad).toBe('10')
    mocks.get.mockResolvedValueOnce(ok({ lastUpdate: '12', config: config('fresh'), clients: [{ name: 'fresh' }], inbounds: [] }))
    await state.loadData()
    expect(mocks.get).toHaveBeenLastCalledWith('api/load', {}); expect(state.lastLoad).toBe('12'); expect(state.clients[0].name).toBe('fresh')
  })
  it.each(details)('$name clears detail request bookkeeping after transport throws', async ({ call, data, empty }) => {
    const state = Data(); mocks.get.mockRejectedValueOnce(new Error('mock offline'))
    await expect(call(state, 1)).rejects.toThrow('mock offline')
    mocks.get.mockResolvedValueOnce(ok(data(1))); expect(await call(state, 1)).not.toEqual(empty)
  })
})
