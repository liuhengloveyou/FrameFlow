import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'
import { LoginPage } from '@/features/auth/login-page'
import { AccountApi } from '@/apis/account-api'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/login')({
  validateSearch: z.object({
    redirect: z.string().optional(),
  }),
  beforeLoad: async () => {
    let res: Awaited<ReturnType<typeof AccountApi.userAuth>> | undefined
    try {
      res = await AccountApi.userAuth()
    } catch {
      return
    }
    if (res?.code === 0 && res.data?.uid) {
      useAuthStore.getState().setUser(res.data)
      throw redirect({ to: '/devices' })
    }
  },
  component: LoginPage,
})
