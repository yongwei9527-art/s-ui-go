import { createApp, defineComponent, h, nextTick, reactive, ref, type App, type Component } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ mobile: null as any }))
vi.mock('vuetify', () => ({ useDisplay: () => ({ smAndDown: mocks.mobile }) }))
vi.mock('@/router', () => ({ default: { currentRoute: { value: { path: '/' } } } }))
vi.mock('@/plugins/httputil', () => ({ logout: vi.fn() }))
vi.mock('@/layouts/default/AppBar.vue', () => ({ default: {} }))
vi.mock('@/layouts/default/View.vue', () => ({ default: {} }))
import Default from '@/layouts/default/Default.vue'
import Drawer from '@/layouts/default/Drawer.vue'

let app: App | undefined
afterEach(() => { app?.unmount(); app = undefined })

describe('default layout responsive drawer state', () => {
  it('does not undo manual toggles on a computed read, and follows breakpoint changes', async () => {
    mocks.mobile = ref(true)
    app = createApp({ ...Default, render: () => null })
    const vm = app.mount(document.createElement('div'))
    const state = (vm.$ as any).setupState
    expect(state.isMobile).toBe(true)
    expect(state.displayDrawer).toBe(false)
    state.toggleDrawer()
    expect(state.isMobile).toBe(true)
    expect(state.displayDrawer).toBe(true)
    mocks.mobile.value = false
    await nextTick()
    expect(state.displayDrawer).toBe(true)
    state.toggleDrawer()
    expect(state.isMobile).toBe(false)
    expect(state.displayDrawer).toBe(false)
    mocks.mobile.value = true
    await nextTick()
    expect(state.displayDrawer).toBe(false)
  })
  it('treats repeated close requests in the same tick as an idempotent update', async () => {
    const parent = reactive({ visible: true })
    const drawer = ref<any>()
    const updates = vi.fn((visible: boolean) => { parent.visible = visible })
    const headlessDrawer: Component = { ...Drawer, render: () => null }
    app = createApp(defineComponent({
      setup: () => () => h(headlessDrawer, {
        ref: drawer,
        isMobile: true,
        displayDrawer: parent.visible,
        onToggleDrawer: () => { parent.visible = !parent.visible },
        'onUpdate:displayDrawer': updates,
      }),
    }))
    app.mount(document.createElement('div'))
    const state = drawer.value.$.setupState
    state.showDrawer = false
    state.showDrawer = false
    await nextTick()
    expect(parent.visible).toBe(false)
    expect(state.showDrawer).toBe(false)
    expect(updates).toHaveBeenCalledWith(false)
    const count = updates.mock.calls.length
    state.showDrawer = false
    expect(updates).toHaveBeenCalledTimes(count)
  })
})
