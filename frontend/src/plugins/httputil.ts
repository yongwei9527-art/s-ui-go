import axios from 'axios'
import api from './api'
import { i18n } from '@/locales'
import router from '@/router'
import { push } from 'notivue'

export interface Msg {
  success: boolean
  msg: string
  obj: any | null
  cancelled?: boolean
}

function _handleMsg(msg: any): void {
  if (!isMsg(msg)) {
    return
  }
  if(msg.msg){
    if (!msg.success && msg.msg == "Invalid login") {
      localStorage.removeItem('s-ui-authenticated')
      push.error({
        title: i18n.global.t('invalidLogin'),
      })
      router.push('/login')
      return
    }
    if (msg.success) {
      push.success({
        message: i18n.global.t('success') + ": " + i18n.global.t('actions.' + msg.msg),
      })
    } else {
      push.error({
        title: i18n.global.t('failed'),
        message: msg.msg
      })
    }
  }
}

export const logout = async () => {
  const response = await HttpUtils.post('api/logout', null)
  localStorage.removeItem('s-ui-authenticated')
  if(response.success){
    router.push('/login')
  }
}

function _respToMsg(resp: any): Msg {
  const data = resp.data
  if (data == null) {
    return { success: true, msg: "", obj: null }
  } else if (isMsg(data)) {
    return { success: data.success, msg: data.msg, obj: Object.hasOwn(data, 'obj') ? data.obj : null }
  } else {
    return { success: false, msg: `unknown data: ${data}`, obj: null }
  }
}

function isMsg(obj: any): obj is Msg {
  return obj != null && Object.hasOwn(obj,'success') && Object.hasOwn(obj,'msg')
}

function errorToMsg(error: any): Msg {
  if (axios.isCancel(error)) {
    return { success: false, msg: '', obj: null, cancelled: true }
  }

  const body = error?.response?.data
  if (isMsg(body)) {
    return { success: body.success, msg: body.msg, obj: Object.hasOwn(body, 'obj') ? body.obj : null }
  }
  return { success: false, msg: error?.message ?? String(error), obj: null }
}

const HttpUtils = {
  async get(url: string, data: object = {}, options: any[] = []): Promise<Msg> {
    let msg: Msg
    try {
        const resp = await api.get(url, { params: data, ...options })
        msg = _respToMsg(resp)
    } catch (e: any) {
        msg = errorToMsg(e)
    }
    _handleMsg(msg)
    return msg
  },
  async post(url: string, data: object | null, options: any = undefined): Promise<Msg> {
    let msg: Msg
    try {
        const resp = await api.post(url, data, options)
        msg = _respToMsg(resp)
    } catch (e: any) {
        msg = errorToMsg(e)
    }
    _handleMsg(msg)
    return msg
  },
}

export default HttpUtils