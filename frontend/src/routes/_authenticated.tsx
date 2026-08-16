import { Outlet, createFileRoute, redirect } from '@tanstack/react-router'
import { ShellSideNav } from '@/app/shell-side-nav/page'
import { AccountApi, normalizeRole } from '@/apis/account-api'
import { buildMenuPathsAndPolicyKeys, useAuthStore } from '@/stores/auth-store'
import { WorkflowNotifyPoller } from '@/features/workflows/workflow-notify-poller'

const LANDING = [
  '/preview',
  '/devices',
  '/map',
  '/review',
  '/recording',
  '/search',
  '/alarms',
  '/alarms/rules',
  '/workflows',
  '/notifications',
  '/system',
  '/users',
]

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    let res: Awaited<ReturnType<typeof AccountApi.userAuth>> | undefined
    try {
      res = await AccountApi.userAuth()
    } catch {
      /* network */
    }

    if (res && res.code === 0 && res.data?.uid) {
      const store = useAuthStore.getState()
      store.setUser(res.data)

      const rolesResp = await AccountApi.getRolesForMe()
      const roles = Array.isArray(rolesResp?.data)
        ? rolesResp!.data.map(normalizeRole)
        : []
      const isRoot = roles.some((r) => r.roleValue.toLowerCase() === 'root')

      let allowedMenus: string[] = []
      let policyAccessKeys: string[] = []
      if (!isRoot) {
        const policyResp = await AccountApi.getPolicyForUser()
        const built = buildMenuPathsAndPolicyKeys(policyResp?.data)
        allowedMenus = built.allowedMenus
        policyAccessKeys = built.policyAccessKeys
      }
      store.setAccess(roles, allowedMenus, policyAccessKeys)

      if (!store.hasMenuAccess(location.pathname)) {
        const fallback = LANDING.find((p) => store.hasMenuAccess(p))
        if (fallback) {
          throw redirect({ to: fallback, replace: true })
        }
        throw redirect({ to: '/login', replace: true })
      }
      return
    }

    useAuthStore.getState().reset()
    throw redirect({
      to: '/login',
      search: { redirect: location.href },
      replace: true,
    })
  },
  component: AuthenticatedLayout,
})

function AuthenticatedLayout() {
  return (
    <ShellSideNav>
      <WorkflowNotifyPoller />
      <Outlet />
    </ShellSideNav>
  )
}
