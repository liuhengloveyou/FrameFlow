import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Badge } from '@astryxdesign/core/Badge'
import { Button } from '@astryxdesign/core/Button'
import { Switch } from '@astryxdesign/core/Switch'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import { Tab, TabList } from '@astryxdesign/core/TabList'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import { PageLayout } from '@/components/page-layout'
import { SidePanel } from '@/components/side-panel'
import {
  DataTable,
  proportional,
  pixel,
  type DataTableColumn,
} from '@/components/data-table'
import {
  AccountApi,
  normalizeRole,
  normalizeUserList,
} from '@/apis/account-api'
import { getErrorMessage } from '@/lib/http'
import { MENU_POLICY_OPTIONS } from '@/lib/menu-policies'
import type { RoleStruct, UserInfo } from '@/apis/passport-types'

type TabKey = 'users' | 'roles' | 'policies'

const TABLE_PROPS = {
  density: 'compact' as const,
  isStriped: true,
  dividers: 'rows' as const,
}

export function UsersPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<TabKey>('users')
  const [q, setQ] = useState('')
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [form, setForm] = useState({
    nickname: '',
    password: '',
    cellphone: '',
    role: 'viewer',
  })
  const [roleForm, setRoleForm] = useState({ title: '', value: '' })
  const [policyRole, setPolicyRole] = useState('operator')
  const [bindUid, setBindUid] = useState<number | null>(null)
  const [bindRole, setBindRole] = useState('viewer')

  const usersQuery = useQuery({
    queryKey: ['passport-users', q],
    queryFn: async () => {
      const raw = await AccountApi.tenantUserList({ nickname: q || undefined })
      return normalizeUserList(raw)
    },
    enabled: tab === 'users' || tab === 'policies',
  })

  const rolesQuery = useQuery({
    queryKey: ['passport-roles'],
    queryFn: () => AccountApi.getTenantRoles(),
  })

  const policyQuery = useQuery({
    queryKey: ['passport-policy', policyRole],
    queryFn: () => AccountApi.getPolicy([policyRole]),
    enabled: tab === 'policies' && Boolean(policyRole),
  })

  const userRolesQuery = useQuery({
    queryKey: ['passport-user-roles', bindUid],
    queryFn: async () => {
      if (!bindUid) return []
      const raw = await AccountApi.getRolesForUser(bindUid)
      return (Array.isArray(raw) ? raw : []).map(normalizeRole)
    },
    enabled: bindUid != null,
  })

  const roleOptions = useMemo(() => {
    const list = (rolesQuery.data ?? []).map((r) => ({
      label: r.title || r.value,
      value: r.value,
    }))
    if (!list.length) {
      return [
        { label: '管理员', value: 'admin' },
        { label: '运维', value: 'operator' },
        { label: '只读', value: 'viewer' },
      ]
    }
    return list.filter((r) => r.value !== 'root')
  }, [rolesQuery.data])

  const createUserMutation = useMutation({
    mutationFn: () =>
      AccountApi.tenantUserAdd({
        nickname: form.nickname.trim(),
        password: form.password,
        cellphone: form.cellphone.trim() || undefined,
        roles: form.role ? [form.role] : [],
      }),
    onSuccess: async () => {
      toast({ body: '用户已创建', type: 'info' })
      setDrawerOpen(false)
      setForm({ nickname: '', password: '', cellphone: '', role: 'viewer' })
      await queryClient.invalidateQueries({ queryKey: ['passport-users'] })
    },
    onError: (e) => toast({ body: getErrorMessage(e), type: 'error' }),
  })

  const deleteUserMutation = useMutation({
    mutationFn: (uid: number) => AccountApi.tenantUserDel(uid),
    onSuccess: async () => {
      toast({ body: '用户已删除', type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['passport-users'] })
    },
    onError: (e) => toast({ body: getErrorMessage(e), type: 'error' }),
  })

  const addRoleMutation = useMutation({
    mutationFn: () =>
      AccountApi.addTenantRole({
        title: roleForm.title.trim(),
        value: roleForm.value.trim(),
      }),
    onSuccess: async () => {
      toast({ body: '角色已添加', type: 'info' })
      setRoleForm({ title: '', value: '' })
      await queryClient.invalidateQueries({ queryKey: ['passport-roles'] })
    },
    onError: (e) => toast({ body: getErrorMessage(e), type: 'error' }),
  })

  const delRoleMutation = useMutation({
    mutationFn: (role: RoleStruct) => AccountApi.delTenantRole(role),
    onSuccess: async () => {
      toast({ body: '角色已删除', type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['passport-roles'] })
    },
    onError: (e) => toast({ body: getErrorMessage(e), type: 'error' }),
  })

  const toggleMenuMutation = useMutation({
    mutationFn: async ({
      obj,
      enabled,
    }: {
      obj: string
      enabled: boolean
    }) => {
      if (enabled) {
        await AccountApi.addPolicyToRole(policyRole, obj, 'GET')
      } else {
        await AccountApi.removePolicyFromRole(policyRole, obj, 'GET')
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['passport-policy', policyRole],
      })
    },
    onError: (e) => toast({ body: getErrorMessage(e), type: 'error' }),
  })

  const bindRoleMutation = useMutation({
    mutationFn: async () => {
      if (!bindUid) throw new Error('未选择用户')
      await AccountApi.addRoleForUser(bindUid, bindRole)
    },
    onSuccess: async () => {
      toast({ body: '已绑定角色', type: 'info' })
      await queryClient.invalidateQueries({
        queryKey: ['passport-user-roles', bindUid],
      })
    },
    onError: (e) => toast({ body: getErrorMessage(e), type: 'error' }),
  })

  const unbindRoleMutation = useMutation({
    mutationFn: async (roleVal: string) => {
      if (!bindUid) throw new Error('未选择用户')
      await AccountApi.removeRoleForUser(bindUid, roleVal)
    },
    onSuccess: async () => {
      toast({ body: '已移除角色', type: 'info' })
      await queryClient.invalidateQueries({
        queryKey: ['passport-user-roles', bindUid],
      })
    },
    onError: (e) => toast({ body: getErrorMessage(e), type: 'error' }),
  })

  const grantedMenus = useMemo(() => {
    const set = new Set<string>()
    for (const p of policyQuery.data ?? []) {
      if (String(p.act).toUpperCase() === 'GET') set.add(p.obj)
    }
    return set
  }, [policyQuery.data])

  async function submitAddRole() {
    if (!roleForm.title.trim() || !roleForm.value.trim()) {
      toast({ body: '请填写名称与角色值', type: 'error' })
      return
    }
    await addRoleMutation.mutateAsync()
  }

  const headerAction =
    tab === 'users' ? (
      <Button
        label="新增用户"
        variant="primary"
        size="sm"
        onClick={() => setDrawerOpen(true)}
      />
    ) : tab === 'roles' ? (
      <Button
        label="添加角色"
        variant="primary"
        size="sm"
        isLoading={addRoleMutation.isPending}
        clickAction={submitAddRole}
      />
    ) : null

  return (
    <PageLayout
      title="用户&权限"
      description="Passport 用户、角色字典与菜单策略"
      actions={headerAction}
      toolbar={
        <TabList
          value={tab}
          onChange={(v) => setTab((v as TabKey) || 'users')}
          size="sm"
          hasDivider
        >
          <Tab value="users" label="用户" />
          <Tab value="roles" label="角色字典" />
          <Tab value="policies" label="角色策略" />
        </TabList>
      }
    >
      {tab === 'users' ? (
        <div className="ff-access-stack">
          <div className="ff-access-panel">
            <div className="ff-access-panel-fields">
              <TextInput
                label="搜索用户"
                placeholder="按昵称搜索"
                value={q}
                onChange={setQ}
                hasClear
              />
            </div>
          </div>
          <div className="ff-table-wrap">
            <DataTable
              {...TABLE_PROPS}
              data={usersQuery.data?.items ?? []}
              idKey={'uid' as keyof UserInfo & string}
              emptyLabel={usersQuery.isLoading ? '加载中…' : '暂无用户'}
              columns={
                [
                  {
                    key: 'nickname',
                    header: '用户名',
                    width: proportional(1.2),
                    render: (u) => <Text type="body">{u.nickname}</Text>,
                  },
                  {
                    key: 'uid',
                    header: 'UID',
                    width: pixel(88),
                    render: (u) => (
                      <Text type="supporting" color="secondary">
                        {u.uid}
                      </Text>
                    ),
                  },
                  {
                    key: 'cellphone',
                    header: '手机',
                    width: pixel(120),
                    render: (u) => (
                      <Text type="supporting" color="secondary">
                        {u.cellphone || '—'}
                      </Text>
                    ),
                  },
                  {
                    key: 'email',
                    header: '邮箱',
                    width: proportional(1),
                    render: (u) => (
                      <Text type="supporting" color="secondary">
                        {u.email || '—'}
                      </Text>
                    ),
                  },
                  {
                    key: 'status',
                    header: '状态',
                    width: pixel(88),
                    render: (u) => (
                      <Badge
                        label={u.ext?.disabled ? '禁用' : '正常'}
                        variant={u.ext?.disabled ? 'warning' : 'success'}
                      />
                    ),
                  },
                  {
                    key: 'actions',
                    header: '操作',
                    width: pixel(140),
                    align: 'end',
                    render: (u: UserInfo) => (
                      <HStack gap={1}>
                        <Button
                          label="角色"
                          size="sm"
                          variant="ghost"
                          onClick={() => {
                            setBindUid(u.uid)
                            setBindRole(roleOptions[0]?.value ?? 'viewer')
                          }}
                        />
                        {u.uid !== 10000 ? (
                          <Button
                            label="删除"
                            size="sm"
                            variant="ghost"
                            clickAction={async () => {
                              await deleteUserMutation.mutateAsync(u.uid)
                            }}
                          />
                        ) : null}
                      </HStack>
                    ),
                  },
                ] satisfies DataTableColumn<UserInfo>[]
              }
            />
          </div>
        </div>
      ) : null}

      {tab === 'roles' ? (
        <div className="ff-access-stack">
          <div className="ff-access-panel">
            <div className="ff-access-panel-fields">
              <TextInput
                label="角色名称"
                value={roleForm.title}
                onChange={(v) => setRoleForm((f) => ({ ...f, title: v }))}
              />
              <TextInput
                label="角色值"
                placeholder="如 auditor"
                value={roleForm.value}
                onChange={(v) => setRoleForm((f) => ({ ...f, value: v }))}
              />
            </div>
          </div>
          <div className="ff-table-wrap">
            <DataTable
              {...TABLE_PROPS}
              data={rolesQuery.data ?? []}
              idKey="value"
              emptyLabel={rolesQuery.isLoading ? '加载中…' : '暂无角色'}
              columns={
                [
                  {
                    key: 'title',
                    header: '名称',
                    width: proportional(1.2),
                    render: (r) => <Text type="body">{r.title}</Text>,
                  },
                  {
                    key: 'value',
                    header: '值',
                    width: proportional(1),
                    render: (r) => (
                      <span className="ff-access-meta">{r.value}</span>
                    ),
                  },
                  {
                    key: 'actions',
                    header: '操作',
                    width: pixel(100),
                    align: 'end',
                    render: (r) =>
                      r.value === 'root' ? (
                        <Text type="supporting" color="secondary">
                          系统
                        </Text>
                      ) : (
                        <Button
                          label="删除"
                          size="sm"
                          variant="ghost"
                          clickAction={async () => {
                            await delRoleMutation.mutateAsync(r)
                          }}
                        />
                      ),
                  },
                ] satisfies DataTableColumn<RoleStruct>[]
              }
            />
          </div>
        </div>
      ) : null}

      {tab === 'policies' ? (
        <div className="ff-access-stack">
          <div className="ff-access-panel">
            <div className="ff-access-panel-fields">
              <Selector
                label="编辑角色"
                value={policyRole}
                onChange={(v) => setPolicyRole(v || 'operator')}
                options={roleOptions}
              />
            </div>
            <p className="ff-access-hint">
              开启后该角色登录可见对应侧栏（策略 act=GET）
            </p>
          </div>
          <div className="ff-table-wrap">
            <DataTable
              {...TABLE_PROPS}
              data={[...MENU_POLICY_OPTIONS]}
              idKey="value"
              emptyLabel="暂无功能项"
              columns={
                [
                  {
                    key: 'label',
                    header: '功能',
                    width: proportional(1.2),
                    render: (m) => <Text type="body">{m.label}</Text>,
                  },
                  {
                    key: 'value',
                    header: '路径',
                    width: proportional(1),
                    render: (m) => (
                      <span className="ff-access-meta">{m.value}</span>
                    ),
                  },
                  {
                    key: 'status',
                    header: '状态',
                    width: pixel(88),
                    render: (m) => {
                      const on = grantedMenus.has(m.value)
                      return (
                        <Badge
                          label={on ? '已授权' : '未授权'}
                          variant={on ? 'success' : 'neutral'}
                        />
                      )
                    },
                  },
                  {
                    key: 'toggle',
                    header: '授权',
                    width: pixel(88),
                    align: 'end',
                    render: (m) => {
                      const on = grantedMenus.has(m.value)
                      return (
                        <Switch
                          label={`${m.label}授权`}
                          isLabelHidden
                          value={on}
                          isDisabled={toggleMenuMutation.isPending}
                          onChange={(next) => {
                            void toggleMenuMutation.mutateAsync({
                              obj: m.value,
                              enabled: next,
                            })
                          }}
                        />
                      )
                    },
                  },
                ] satisfies DataTableColumn<
                  (typeof MENU_POLICY_OPTIONS)[number]
                >[]
              }
            />
          </div>
        </div>
      ) : null}

      <SidePanel
        open={drawerOpen}
        title="新增用户"
        onClose={() => setDrawerOpen(false)}
        footer={
          <>
            <Button
              label="取消"
              variant="secondary"
              onClick={() => setDrawerOpen(false)}
            />
            <Button
              label="创建"
              variant="primary"
              isLoading={createUserMutation.isPending}
              clickAction={async () => {
                if (!form.nickname.trim() || form.password.length < 6) {
                  toast({ body: '用户名必填，密码至少 6 位', type: 'error' })
                  return
                }
                await createUserMutation.mutateAsync()
              }}
            />
          </>
        }
      >
        <VStack gap={3}>
          <TextInput
            label="用户名"
            value={form.nickname}
            onChange={(v) => setForm((f) => ({ ...f, nickname: v }))}
          />
          <TextInput
            label="密码"
            type="password"
            value={form.password}
            onChange={(v) => setForm((f) => ({ ...f, password: v }))}
          />
          <TextInput
            label="手机（可选）"
            value={form.cellphone}
            onChange={(v) => setForm((f) => ({ ...f, cellphone: v }))}
          />
          <Selector
            label="初始角色"
            value={form.role}
            onChange={(v) => setForm((f) => ({ ...f, role: v || 'viewer' }))}
            options={roleOptions}
          />
        </VStack>
      </SidePanel>

      <SidePanel
        open={bindUid != null}
        title={`绑定角色 · UID ${bindUid ?? ''}`}
        onClose={() => setBindUid(null)}
        footer={
          <Button
            label="关闭"
            variant="secondary"
            onClick={() => setBindUid(null)}
          />
        }
      >
        <VStack gap={3}>
          <HStack gap={2} vAlign="end">
            <Selector
              label="角色"
              value={bindRole}
              onChange={(v) => setBindRole(v || 'viewer')}
              options={roleOptions}
            />
            <Button
              label="绑定"
              variant="primary"
              size="sm"
              isLoading={bindRoleMutation.isPending}
              clickAction={async () => {
                await bindRoleMutation.mutateAsync()
              }}
            />
          </HStack>
          <Text type="label">当前角色</Text>
          {(userRolesQuery.data ?? []).length === 0 ? (
            <Text type="supporting" color="secondary">
              暂无绑定角色
            </Text>
          ) : (
            (userRolesQuery.data ?? []).map((r) => (
              <HStack key={r.roleValue} justify="between" vAlign="center">
                <Badge label={r.roleTitle || r.roleValue} variant="neutral" />
                {r.roleValue !== 'root' ? (
                  <Button
                    label="移除"
                    size="sm"
                    variant="ghost"
                    clickAction={async () => {
                      await unbindRoleMutation.mutateAsync(r.roleValue)
                    }}
                  />
                ) : null}
              </HStack>
            ))
          )}
        </VStack>
      </SidePanel>
    </PageLayout>
  )
}
