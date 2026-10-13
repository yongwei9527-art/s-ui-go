import axios from 'axios'

axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8'
axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest'

axios.defaults.baseURL = "./"
const api = axios.create()
const pendingRequests = new Map()
const requestKeys = new WeakMap<object, string>()

const stableRequestValue = (value: any): string => {
    if (value === undefined || value === null) return ''
    if (value instanceof URLSearchParams) return value.toString()
    if (typeof value !== 'object') return String(value)

    const ordered: Record<string, any> = {}
    for (const key of Object.keys(value).sort()) {
        ordered[key] = value[key]
    }
    return JSON.stringify(ordered)
}

// Different outbound checks share one URL but use different query parameters.
// Include parameters in the key so bulk checks do not cancel each other.
const getRequestKey = (config: any): string | null => {
    if (config.data instanceof FormData) return null
    return `${config.method ?? 'get'}:${config.url ?? ''}?${stableRequestValue(config.params)}:${stableRequestValue(config.data)}`
}

const clearPendingRequest = (config: any) => {
    const token = config?.cancelToken
    if (!token) return
    const requestKey = requestKeys.get(token)
    if (requestKey === undefined) return
    requestKeys.delete(token)

    const pending = pendingRequests.get(requestKey)
    if (pending?.token === config.cancelToken) {
        pendingRequests.delete(requestKey)
    }
}

api.interceptors.request.use(
    (config) => {
        const requestKey = getRequestKey(config)
        if (requestKey !== null && pendingRequests.has(requestKey)) {
            pendingRequests.get(requestKey).cancel('Duplicate request cancelled')
        }

        const cancelSource = axios.CancelToken.source()
        config.cancelToken = cancelSource.token
        if (requestKey !== null) {
            pendingRequests.set(requestKey, cancelSource)
            requestKeys.set(cancelSource.token, requestKey)
            void cancelSource.token.promise.then(() => clearPendingRequest({ cancelToken: cancelSource.token }))
        }

        if (config.data instanceof FormData) {
            config.headers['Content-Type'] = 'multipart/form-data'
        }
        return config
    },
    (error) => Promise.reject(error),
)

api.interceptors.response.use(
    (response) => {
        clearPendingRequest(response.config)
        return response
    },
    (error) => {
        if (axios.isCancel(error)) {
            console.warn(error.message)
        } else if (error.config) {
            clearPendingRequest(error.config)
        }
        return Promise.reject(error)
    },
)

export default api
