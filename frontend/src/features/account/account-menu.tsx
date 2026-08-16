import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { Dialog, DialogHeader } from '@astryxdesign/core/Dialog'
import { DropdownMenu } from '@astryxdesign/core/DropdownMenu'
import { HStack } from '@astryxdesign/core/HStack'
import { Icon } from '@astryxdesign/core/Icon'
import { Layout } from '@astryxdesign/core/Layout'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import { VStack } from '@astryxdesign/core/VStack'
import {
  ArrowRightStartOnRectangleIcon,
  KeyIcon,
  UserCircleIcon,
} from '@heroicons/react/24/outline'
import { useShellNav } from '@/app/shell-nav-context'
import { AccountApi } from '@/apis/account-api'
import { logout as logoutApi } from '@/apis/auth-api'
import { getErrorMessage } from '@/lib/http'
import { useAuthStore } from '@/stores/auth-store'

/**
 * 侧栏底部账号菜单：改资料 / 改密码 / 退出（不再跳转 /users）
 */
export function AccountMenu() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const shellNav = useShellNav()
  const collapsed = shellNav?.isCollapsed ?? false
  const user = useAuthStore((s) => s.user)
  const setUser = useAuthStore((s) => s.setUser)
  const reset = useAuthStore((s) => s.reset)

  const [menuOpen, setMenuOpen] = useState(false)
  const [profileOpen, setProfileOpen] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)

  const displayName = user?.nickname?.trim() || '未登录'

  /** 退出：passport 清会话 cookie → 清空前端状态 → 硬跳登录页 */
  const doLogout = async () => {
    try {
      await logoutApi()
    } finally {
      reset()
      queryClient.clear()
      // 硬跳转，确保浏览器应用 Set-Cookie 清会话，并丢掉 SPA 内存态
      window.location.assign('/login')
    }
  }

  return (
    <>
      <div className="ff-sidenav-account">
        <DropdownMenu
          isMenuOpen={menuOpen}
          onOpenChange={setMenuOpen}
          hasChevron={false}
          placement="above"
          menuWidth={200}
          button={{
            label: displayName,
            variant: 'ghost',
            size: 'md',
            isIconOnly: collapsed,
            icon: <Icon icon={UserCircleIcon} size="sm" />,
            className: 'ff-sidenav-account-trigger',
          }}
          items={[
            {
              label: '账号信息',
              icon: UserCircleIcon,
              onClick: () => {
                setMenuOpen(false)
                setProfileOpen(true)
              },
            },
            {
              label: '修改密码',
              icon: KeyIcon,
              onClick: () => {
                setMenuOpen(false)
                setPasswordOpen(true)
              },
            },
            { type: 'divider' as const },
            {
              label: '退出登录',
              icon: ArrowRightStartOnRectangleIcon,
              onClick: () => {
                setMenuOpen(false)
                void doLogout()
              },
            },
          ]}
        />
      </div>

      <AccountProfileDialog
        isOpen={profileOpen}
        onOpenChange={setProfileOpen}
        onSaved={async () => {
          try {
            const res = await AccountApi.userAuth()
            if (res?.code === 0 && res.data) setUser(res.data)
          } catch {
            /* ignore */
          }
          toast({ body: '账户信息已保存', type: 'info' })
        }}
      />

      <AccountPasswordDialog
        isOpen={passwordOpen}
        onOpenChange={setPasswordOpen}
        onSaved={() => toast({ body: '密码修改成功', type: 'info' })}
      />
    </>
  )
}

function AccountProfileDialog({
  isOpen,
  onOpenChange,
  onSaved,
}: {
  isOpen: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void | Promise<void>
}) {
  const toast = useToast()
  const user = useAuthStore((s) => s.user)
  const [nickname, setNickname] = useState('')
  const [cellphone, setCellphone] = useState('')
  const [email, setEmail] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!isOpen || !user) return
    setNickname(user.nickname ?? '')
    setCellphone(user.cellphone ?? '')
    setEmail(user.email ?? '')
  }, [isOpen, user])

  const save = async () => {
    if (!user?.uid) return
    if (!nickname.trim()) {
      toast({ body: '账号名不能为空', type: 'error' })
      return
    }
    setSaving(true)
    try {
      await AccountApi.modifyProfile({
        uid: user.uid,
        nickname: nickname.trim(),
        cellphone: cellphone.trim(),
        email: email.trim(),
      })
      onOpenChange(false)
      await onSaved()
    } catch (err) {
      toast({ body: getErrorMessage(err), type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      purpose="form"
      width={420}
      padding={0}
    >
      <Layout
        height="auto"
        padding={4}
        header={<DialogHeader title="账号信息" onOpenChange={onOpenChange} />}
        content={
          <div className="ff-dialog-body">
            <VStack gap={3}>
              <Text type="supporting" color="secondary">
                修改当前登录账号的基本资料。
              </Text>
              <TextInput
                label="账号名"
                isRequired
                value={nickname}
                onChange={setNickname}
                width="100%"
              />
              <TextInput
                label="手机号"
                value={cellphone}
                onChange={setCellphone}
                width="100%"
              />
              <TextInput label="邮箱" value={email} onChange={setEmail} width="100%" />
              <Text type="supporting" color="secondary">
                UID：{user?.uid ?? '—'}
              </Text>
            </VStack>
          </div>
        }
        footer={
          <div className="ff-dialog-footer">
            <HStack gap={2} justify="end">
              <Button
                label="取消"
                variant="secondary"
                size="sm"
                onClick={() => onOpenChange(false)}
              />
              <Button
                label="保存"
                variant="primary"
                size="sm"
                isLoading={saving}
                clickAction={save}
              />
            </HStack>
          </div>
        }
      />
    </Dialog>
  )
}

function AccountPasswordDialog({
  isOpen,
  onOpenChange,
  onSaved,
}: {
  isOpen: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const toast = useToast()
  const [oldPwd, setOldPwd] = useState('')
  const [newPwd, setNewPwd] = useState('')
  const [confirmPwd, setConfirmPwd] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!isOpen) return
    setOldPwd('')
    setNewPwd('')
    setConfirmPwd('')
  }, [isOpen])

  const save = async () => {
    if (!oldPwd.trim() || !newPwd.trim()) {
      toast({ body: '请填写旧密码和新密码', type: 'error' })
      return
    }
    if (newPwd.length < 6) {
      toast({ body: '新密码长度不能少于 6 位', type: 'error' })
      return
    }
    if (newPwd !== confirmPwd) {
      toast({ body: '两次输入的新密码不一致', type: 'error' })
      return
    }
    setSaving(true)
    try {
      await AccountApi.modifyPassword(newPwd, oldPwd)
      onOpenChange(false)
      onSaved()
    } catch (err) {
      toast({ body: getErrorMessage(err), type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      purpose="form"
      width={420}
      padding={0}
    >
      <Layout
        height="auto"
        padding={4}
        header={<DialogHeader title="修改密码" onOpenChange={onOpenChange} />}
        content={
          <div className="ff-dialog-body">
            <VStack gap={3}>
              <TextInput
                label="旧密码"
                type="password"
                isRequired
                value={oldPwd}
                onChange={setOldPwd}
                width="100%"
              />
              <TextInput
                label="新密码"
                type="password"
                isRequired
                value={newPwd}
                onChange={setNewPwd}
                width="100%"
              />
              <TextInput
                label="确认新密码"
                type="password"
                isRequired
                value={confirmPwd}
                onChange={setConfirmPwd}
                width="100%"
              />
            </VStack>
          </div>
        }
        footer={
          <div className="ff-dialog-footer">
            <HStack gap={2} justify="end">
              <Button
                label="取消"
                variant="secondary"
                size="sm"
                onClick={() => onOpenChange(false)}
              />
              <Button
                label="保存"
                variant="primary"
                size="sm"
                isLoading={saving}
                clickAction={save}
              />
            </HStack>
          </div>
        }
      />
    </Dialog>
  )
}
