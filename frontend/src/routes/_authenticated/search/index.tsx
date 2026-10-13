import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { SearchPage } from '@/features/search/search-page'

export const Route = createFileRoute('/_authenticated/search/')({
  validateSearch: z.object({
    /** 全局顶栏搜索带入的初始关键词 */
    q: z.string().optional(),
  }),
  component: SearchPage,
})
