import { create } from 'zustand'
import type { AccessRole, UserInfo } from '@/apis/passport-types'

function normalizeMenuPath(path: string): string {
  const raw = (path || '').split('?')[0].trim()
  if (!raw) return '/'
  const noTrail = raw.replace(/\/+$/, '')
  return noTrail || '/'
}

export function buildMenuPathsAndPolicyKeys(raw: unknown): {
  allowedMenus: string[]
  policyAccessKeys: string[]
} {
  if (!Array.isArray(raw)) return { allowedMenus: [], policyAccessKeys: [] }
  const objs = new Set<string>()
  const keys = new Set<string>()
  for (const item of raw) {
    if (!item || typeof item !== 'object') continue
    const o = item as Record<string, unknown>
    const obj = String(o.obj ?? '').trim()
    const act = String(o.act ?? '').trim().toUpperCase()
    if (!obj) continue
    objs.add(obj)
    if (act) keys.add(`${obj}\t${act}`)
  }
  return {
    allowedMenus: [...objs],
    policyAccessKeys: [...keys],
  }
}

type AuthState = {
  user: UserInfo | null
  roles: AccessRole[]
  allowedMenus: string[]
  policyAccessKeys: string[]
  /** 会话已确认（passport cookie） */
  sessionReady: boolean
  setUser: (user: UserInfo | null) => void
  setAccess: (roles: AccessRole[], allowedMenus: string[], policyAccessKeys?: string[]) => void
  setSessionReady: (ready: boolean) => void
  hasMenuAccess: (path: string) => boolean
  isRoot: () => boolean
  isAuthenticated: () => boolean
  reset: () => void
  /** @deprecated 兼容旧调用，passport 不用 token */
  logout: () => void
}

export const useAuthStore = create<AuthState>()((set, get) => ({
  user: null,
  roles: [],
  allowedMenus: [],
  policyAccessKeys: [],
  sessionReady: false,
  setUser: (user) => set({ user, sessionReady: Boolean(user) }),
  setAccess: (roles, allowedMenus, policyAccessKeys) =>
    set({
      roles,
      allowedMenus: Array.from(new Set(allowedMenus.map((s) => s.trim()).filter(Boolean))),
      policyAccessKeys: Array.from(
        new Set((policyAccessKeys ?? []).map((s) => s.trim()).filter(Boolean)),
      ),
    }),
  setSessionReady: (ready) => set({ sessionReady: ready }),
  isRoot: () =>
    get().roles.some((r) => String(r.roleValue ?? '').toLowerCase() === 'root'),
  isAuthenticated: () => Boolean(get().user?.uid) || get().sessionReady,
  hasMenuAccess: (path: string) => {
    const state = get()
    const normalized = normalizeMenuPath(path || '')
    if (!path?.trim()) return true
    if (state.isRoot()) return true
    if (normalized === '/') return true
    return state.allowedMenus.some((raw) => {
      const m = normalizeMenuPath(raw)
      if (!m || m === '/') return false
      return m === normalized || normalized.startsWith(`${m}/`) || m.startsWith(`${normalized}/`)
    })
  },
  reset: () =>
    set({
      user: null,
      roles: [],
      allowedMenus: [],
      policyAccessKeys: [],
      sessionReady: false,
    }),
  logout: () => get().reset(),
}))

/** 登录页展示用 */
export type AuthUser = {
  id: string
  username: string
  displayName: string
  role: string
}

export function toAuthUser(u: UserInfo | null): AuthUser | null {
  if (!u) return null
  return {
    id: String(u.uid),
    username: u.nickname,
    displayName: u.nickname,
    role: 'user',
  }
}
