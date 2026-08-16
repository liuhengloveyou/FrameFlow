import { useEffect, useRef } from 'react'
import { useToast } from '@astryxdesign/core/Toast'
import { fetchLatestWorkflowToast } from '@/apis/workflows-api'

/** 轮询工作流 toast 通道，弹出页面提示 */
export function WorkflowNotifyPoller() {
  const toast = useToast()
  const lastSeq = useRef(0)
  const ready = useRef(false)

  useEffect(() => {
    let cancelled = false
    const tick = async () => {
      try {
        const res = await fetchLatestWorkflowToast()
        if (cancelled) return
        const seq = Number(res.seq ?? 0)
        if (!ready.current) {
          lastSeq.current = seq
          ready.current = true
          return
        }
        if (seq > lastSeq.current && res.item?.message) {
          lastSeq.current = seq
          toast({
            body: String(res.item.message),
            type: 'info',
          })
        } else if (seq > lastSeq.current) {
          lastSeq.current = seq
        }
      } catch {
        /* ignore poll errors */
      }
    }
    void tick()
    const id = window.setInterval(() => void tick(), 3000)
    return () => {
      cancelled = true
      window.clearInterval(id)
    }
  }, [toast])

  return null
}
