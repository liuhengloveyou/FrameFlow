import axios from 'axios'
import type {
  AccessPolicy,
  AccessRole,
  HttpResponse,
  RoleStruct,
  UserInfo,
} from '@/apis/passport-types'

const passport = axios.create({
  timeout: 30000,
  withCredentials: true,
  validateStatus: (s) => s < 500,
})

function xapi(name: string) {
  return { 'X-API': name }
}

function assertOk<T>(resp: HttpResponse<T> | undefined, fallback: string): T {
  if (!resp || resp.code !== 0) {
    throw new Error(resp?.msg || fallback)
  }
  return resp.data
}

export const AccountApi = {
  async login(nickname: string, password: string) {
    const { data } = await passport.post<HttpResponse<UserInfo>>(
      '/usercenter',
      { nickname, password, sms: '' },
      { headers: xapi('user/login') },
    )
    return assertOk(data, '登录失败')
  },

  async logout() {
    const { data } = await passport.post<HttpResponse<string>>('/usercenter', null, {
      headers: xapi('user/logout'),
    })
    if (data?.code !== 0) throw new Error(data?.msg || '登出失败')
  },

  async userAuth() {
    const { data } = await passport.get<HttpResponse<UserInfo>>('/usercenter', {
      headers: xapi('user/auth'),
    })
    return data
  },

  async getRolesForMe() {
    const { data } = await passport.get<HttpResponse<AccessRole[] | RoleStruct[]>>('/usercenter', {
      headers: xapi('access/getRolesForMe'),
    })
    return data
  },

  async getPolicyForUser() {
    const { data } = await passport.get<HttpResponse<AccessPolicy[]>>('/usercenter', {
      headers: xapi('access/getPolicyForUser'),
    })
    return data
  },

  async tenantUserList(params: { page?: number; pageSize?: number; nickname?: string } = {}) {
    const { data } = await passport.get<HttpResponse<{ list?: UserInfo[]; total?: number } | UserInfo[]>>(
      '/usercenter',
      {
        params: {
          page: params.page ?? 1,
          pageSize: params.pageSize ?? 50,
          hasTotal: 1,
          nickname: params.nickname ?? '',
        },
        headers: xapi('tenant/getUsers'),
      },
    )
    return assertOk(data, '加载用户失败')
  },

  async tenantUserAdd(payload: {
    nickname: string
    password: string
    cellphone?: string
    email?: string
    roles?: string[]
  }) {
    const { data } = await passport.post<HttpResponse<number>>(
      '/usercenter',
      {
        nickname: payload.nickname,
        password: payload.password,
        cellphone: payload.cellphone || undefined,
        email: payload.email || undefined,
        roles: payload.roles ?? [],
      },
      { headers: xapi('tenant/user/add') },
    )
    return assertOk(data, '创建用户失败')
  },

  async tenantUserDel(uid: number) {
    const { data } = await passport.post<HttpResponse<string>>(
      '/usercenter',
      { uid },
      { headers: xapi('tenant/delUser') },
    )
    if (data?.code !== 0) throw new Error(data?.msg || '删除用户失败')
  },

  async tenantUserDisable(uid: number, disable: number) {
    const { data } = await passport.post<HttpResponse<string>>(
      '/usercenter',
      { uid, disable },
      { headers: xapi('tenant/userDisableByUID') },
    )
    if (data?.code !== 0) throw new Error(data?.msg || '更新状态失败')
  },

  async getTenantRoles() {
    const { data } = await passport.get<HttpResponse<RoleStruct[]>>('/usercenter', {
      headers: xapi('tenant/getRoles'),
    })
    return assertOk(data, '加载角色失败')
  },

  async addTenantRole(role: RoleStruct) {
    const { data } = await passport.post<HttpResponse<string>>('/usercenter', role, {
      headers: xapi('tenant/addRole'),
    })
    if (data?.code !== 0) throw new Error(data?.msg || '添加角色失败')
  },

  async delTenantRole(role: RoleStruct) {
    const { data } = await passport.post<HttpResponse<string>>('/usercenter', role, {
      headers: xapi('tenant/delRole'),
    })
    if (data?.code !== 0) throw new Error(data?.msg || '删除角色失败')
  },

  async getRolesForUser(uid: number) {
    const { data } = await passport.get<HttpResponse<RoleStruct[] | AccessRole[]>>('/usercenter', {
      params: { uid },
      headers: xapi('access/getRolesForUser'),
    })
    return assertOk(data, '加载用户角色失败')
  },

  async addRoleForUser(uid: number, roleVal: string) {
    const { data } = await passport.post<HttpResponse<string>>(
      '/usercenter',
      { uid, value: roleVal },
      { headers: xapi('access/addRoleForUser') },
    )
    if (data?.code !== 0) throw new Error(data?.msg || '绑定角色失败')
  },

  async removeRoleForUser(uid: number, roleVal: string) {
    const { data } = await passport.post<HttpResponse<string>>(
      '/usercenter',
      { uid, value: roleVal },
      { headers: xapi('access/removeRoleForUser') },
    )
    if (data?.code !== 0) throw new Error(data?.msg || '移除角色失败')
  },

  async getPolicy(roles: string[]) {
    const { data } = await passport.get<HttpResponse<AccessPolicy[]>>('/usercenter', {
      params: { roles: roles.join(',') },
      headers: xapi('access/getPolicy'),
    })
    return assertOk(data, '加载策略失败')
  },

  async addPolicyToRole(role: string, obj: string, act: string) {
    const { data } = await passport.post<HttpResponse<string>>(
      '/usercenter',
      { role, obj, act },
      { headers: xapi('access/addPolicyToRole') },
    )
    if (data?.code !== 0) throw new Error(data?.msg || '添加策略失败')
  },

  async removePolicyFromRole(role: string, obj: string, act: string) {
    const { data } = await passport.post<HttpResponse<string>>(
      '/usercenter',
      { role, obj, act },
      { headers: xapi('access/removePolicyFromRole') },
    )
    if (data?.code !== 0) throw new Error(data?.msg || '移除策略失败')
  },

  /** 修改当前登录用户资料 */
  async modifyProfile(payload: {
    uid: number
    nickname: string
    cellphone?: string
    email?: string
  }) {
    const { data } = await passport.post<HttpResponse<string>>(
      '/usercenter',
      {
        uid: payload.uid,
        nickName: payload.nickname,
        cellphone: payload.cellphone || undefined,
        email: payload.email || undefined,
      },
      { headers: xapi('user/modify') },
    )
    if (data?.code !== 0) throw new Error(data?.msg || '保存账户信息失败')
  },

  /** 修改当前登录用户密码：n=新密码，o=旧密码 */
  async modifyPassword(newPassword: string, oldPassword: string) {
    const { data } = await passport.post<HttpResponse<string>>(
      '/usercenter',
      { n: newPassword, o: oldPassword },
      { headers: xapi('user/modify/password') },
    )
    if (data?.code !== 0) throw new Error(data?.msg || '修改密码失败')
  },
}

export function normalizeRole(raw: unknown): AccessRole {
  const item = (raw ?? {}) as Record<string, unknown>
  return {
    roleValue: String(item.roleValue ?? item.value ?? ''),
    roleTitle: String(item.roleTitle ?? item.title ?? item.name ?? ''),
  }
}

export function normalizeUserList(raw: unknown): { items: UserInfo[]; total: number } {
  if (Array.isArray(raw)) {
    return { items: raw as UserInfo[], total: raw.length }
  }
  if (raw && typeof raw === 'object') {
    const o = raw as { list?: UserInfo[]; total?: number; items?: UserInfo[] }
    const items = o.list ?? o.items ?? []
    return { items, total: o.total ?? items.length }
  }
  return { items: [], total: 0 }
}
