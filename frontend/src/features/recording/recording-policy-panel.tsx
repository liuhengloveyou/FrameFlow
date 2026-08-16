import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { Switch } from '@astryxdesign/core/Switch'
import {
  SegmentedControl,
  SegmentedControlItem,
} from '@astryxdesign/core/SegmentedControl'
import { TextInput } from '@astryxdesign/core/TextInput'
import { useToast } from '@astryxdesign/core/Toast'
import {
  listRecordingPolicies,
  upsertRecordingPolicy,
} from '@/apis/recording-api'
import { getErrorMessage } from '@/lib/http'
import type { RecordingMode, RecordingPolicy } from '@/types/api'

const PRESETS = [
  { label: '5 / 10', pre: 5, post: 10 },
  { label: '10 / 10', pre: 10, post: 10 },
  { label: '10 / 15', pre: 10, post: 15 },
] as const

export function RecordingPolicyPanel() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [drafts, setDrafts] = useState<
    Record<string, { pre: string; post: string }>
  >({})

  const policiesQuery = useQuery({
    queryKey: ['recording-policies'],
    queryFn: listRecordingPolicies,
  })

  const saveMut = useMutation({
    mutationFn: upsertRecordingPolicy,
    onSuccess: async () => {
      toast({ body: '策略已保存', type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['recording-policies'] })
    },
    onError: (err) => toast({ body: getErrorMessage(err), type: 'error' }),
  })

  const policies = policiesQuery.data ?? []
  const eventEnabled = useMemo(
    () => policies.filter((p) => p.mode === 'event' && p.enabled),
    [policies],
  )

  const draftOf = (p: RecordingPolicy) =>
    drafts[p.cameraId] ?? {
      pre: String(p.preSeconds),
      post: String(p.postSeconds),
    }

  const saveRow = async (
    p: RecordingPolicy,
    patch: Partial<Omit<RecordingPolicy, 'updatedAt'>>,
  ) => {
    await saveMut.mutateAsync({ ...p, ...patch })
  }

  return (
    <div className="ff-recording-policy">
      <p className="ff-recording-policy-lead">
        按摄像头配置连续录像或事件触发（预录 / 延录）。更改即时生效（Mock）。
      </p>

      {eventEnabled.length > 0 ? (
        <div className="ff-recording-policy-presets">
          <span>事件模式批量预设</span>
          {PRESETS.map((pr) => (
            <button
              key={pr.label}
              type="button"
              className="ff-recording-policy-preset"
              disabled={saveMut.isPending}
              onClick={async () => {
                for (const p of eventEnabled) {
                  await saveMut.mutateAsync({
                    ...p,
                    preSeconds: pr.pre,
                    postSeconds: pr.post,
                  })
                }
              }}
            >
              {pr.label}
            </button>
          ))}
        </div>
      ) : null}

      <div className="ff-recording-policy-list">
        {policiesQuery.isLoading ? (
          <div className="ff-placeholder">加载中…</div>
        ) : !policies.length ? (
          <div className="ff-placeholder">暂无策略</div>
        ) : (
          policies.map((p) => {
            const d = draftOf(p)
            return (
              <article key={p.cameraId} className="ff-recording-policy-row">
                <div className="ff-recording-policy-main">
                  <div className="ff-recording-policy-name">{p.cameraName}</div>
                  <div className="ff-recording-policy-meta">
                    更新 {new Date(p.updatedAt).toLocaleString()}
                  </div>
                </div>

                <label className="ff-recording-policy-enable">
                  <span>启用</span>
                  <Switch
                    label="启用"
                    isLabelHidden
                    value={p.enabled}
                    onChange={(v) => {
                      void saveRow(p, { enabled: v })
                    }}
                  />
                </label>

                <div className="ff-recording-policy-mode">
                  <SegmentedControl
                    label="模式"
                    value={p.mode}
                    onChange={(v) => {
                      void saveRow(p, { mode: v as RecordingMode })
                    }}
                  >
                    <SegmentedControlItem value="continuous" label="连续" />
                    <SegmentedControlItem value="event" label="事件" />
                  </SegmentedControl>
                </div>

                {p.mode === 'event' ? (
                  <div className="ff-recording-policy-prepost">
                    <TextInput
                      label="预录"
                      size="sm"
                      value={d.pre}
                      onChange={(v) =>
                        setDrafts((prev) => ({
                          ...prev,
                          [p.cameraId]: { ...d, pre: v },
                        }))
                      }
                    />
                    <TextInput
                      label="延录"
                      size="sm"
                      value={d.post}
                      onChange={(v) =>
                        setDrafts((prev) => ({
                          ...prev,
                          [p.cameraId]: { ...d, post: v },
                        }))
                      }
                    />
                    <Button
                      label="保存"
                      size="sm"
                      variant="secondary"
                      isLoading={saveMut.isPending}
                      clickAction={async () => {
                        const pre = Math.min(
                          60,
                          Math.max(1, Number(d.pre) || 10),
                        )
                        const post = Math.min(
                          60,
                          Math.max(1, Number(d.post) || 10),
                        )
                        await saveRow(p, { preSeconds: pre, postSeconds: post })
                        setDrafts((prev) => {
                          const next = { ...prev }
                          delete next[p.cameraId]
                          return next
                        })
                      }}
                    />
                  </div>
                ) : (
                  <div className="ff-recording-policy-hint">7×24 全天录像</div>
                )}
              </article>
            )
          })
        )}
      </div>
    </div>
  )
}
