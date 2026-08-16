import { AccountApi } from '@/apis/account-api'
import type { UserInfo } from '@/apis/passport-types'

/** 与 configs/passport.conf.yaml 的 session_key 一致 */
export const PASSPORT_SESSION_COOKIE = 'frameflow-sess'

/** 兼容旧类型：真实登录走 passport cookie，不再返回 JWT */
export type LoginResult = {
  user: UserInfo
}

export async function login(username: string, password: string): Promise<LoginResult> {
  const user = await AccountApi.login(username.trim(), password)
  return { user }
}

/**
 * 退出登录：
 * 1) 调 passport `user/logout`（服务端 Set-Cookie Max-Age=-1 清除 HttpOnly 会话）
 * 2) 客户端再尽力过期同名 cookie（非 HttpOnly 时生效）
 */
export async function logout(): Promise<void> {
  try {
    await AccountApi.logout()
  } catch {
    /* 会话已失效时忽略 */
  }
  clearPassportSessionCookie()
}

/** 尽力清除会话 cookie（HttpOnly 只能靠服务端 logout 的 Set-Cookie） */
export function clearPassportSessionCookie() {
  if (typeof document === 'undefined') return
  const expire = `${PASSPORT_SESSION_COOKIE}=; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Path=/`
  document.cookie = expire
  document.cookie = `${expire}; SameSite=Lax`
}
