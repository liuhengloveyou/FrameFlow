import axios, { type AxiosError } from 'axios'

/** 业务 /api/v1 接口（与 passport /usercenter 分离） */
export const http = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  withCredentials: true,
})

http.interceptors.response.use(
  (res) => res,
  async (error: AxiosError) => {
    if (error.response?.status === 401) {
      // 业务 API 未授权时不自动清 passport 会话；由路由 beforeLoad 处理
    }
    return Promise.reject(error)
  },
)

export function getErrorMessage(error: unknown, fallback = '请求失败'): string {
  if (axios.isAxiosError(error)) {
    if (error.code === 'ERR_CANCELED' || error.name === 'CanceledError') {
      return fallback
    }
    const data = error.response?.data
    if (typeof data === 'string' && data.trim()) return data.trim()
    if (data && typeof data === 'object') {
      const obj = data as {
        message?: string
        error?: string
        msg?: string
        data?: { msg?: string; message?: string }
      }
      const nested = obj.data?.msg || obj.data?.message
      const fromBody = obj.msg || obj.message || obj.error || nested
      if (fromBody) return fromBody
    }
    // 避免只暴露 axios 默认的 "Request failed with status code 409"
    const status = error.response?.status
    if (status === 409) return '操作冲突：资源正被引用或状态不允许'
    if (status === 404) return '资源不存在'
    if (status === 403) return '无权限执行此操作'
    return error.message || fallback
  }
  if (error instanceof Error) return error.message
  return fallback
}
