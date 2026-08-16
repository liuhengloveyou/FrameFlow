export type HttpResponse<T = unknown> = {
  code: number
  total?: number
  msg: string
  data: T
}

export type UserInfo = {
  uid: number
  tenant_id: number
  nickname: string
  cellphone?: string
  email?: string
  avatarUrl?: string
  loginTime?: string
  ext?: {
    disabled?: number
    deps?: number[]
    [key: string]: unknown
  }
  roles?: string[]
}

export type AccessRole = {
  roleValue: string
  roleTitle: string
}

export type RoleStruct = {
  title: string
  value: string
}

export type AccessPolicy = {
  role: string
  obj: string
  act: string
}

export type PageUsers = {
  list?: UserInfo[]
  total?: number
}
