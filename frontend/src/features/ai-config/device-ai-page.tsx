import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { Badge } from '@astryxdesign/core/Badge'
import { NumberInput } from '@astryxdesign/core/NumberInput'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import { Switch } from '@astryxdesign/core/Switch'
import { FieldStatus } from '@astryxdesign/core/FieldStatus'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import { PageLayout } from '@/components/page-layout'
import { CameraStatusBadge } from '@/components/status-badge'
import { getCamera } from '@/apis/devices-api'
import {
  algorithmAction,
  bindCameraAlgorithm,
  listCameraAlgorithms,
  saveCameraAlgorithm,
  unbindCameraAlgorithm,
} from '@/apis/device-ai-api'
import { listAlgorithms } from '@/apis/algorithms-api'
import { getErrorMessage } from '@/lib/http'
import { resolveStreamSrc } from '@/lib/preview-source'
import type { AlgorithmRunStatus, AlgorithmRuleField, Point2D } from '@/types/api'
import { RegionEditor } from './region-editor'
import {
  hasAiConfigErrors,
  validateAiConfig,
  type AiConfigErrors,
} from '@/lib/validate-ai-config'

const statusLabel: Record<AlgorithmRunStatus, string> = {
  uninitialized: '待配置',
  running: '运行中',
  stopped: '已停止',
  error: '异常',
}

function statusVariant(
  s: AlgorithmRunStatus,
): 'neutral' | 'success' | 'warning' | 'error' {
  if (s === 'running') return 'success'
  if (s === 'error') return 'error'
  if (s === 'stopped') return 'warning'
  return 'neutral'
}

function asPointRegions(raw: unknown): Point2D[][] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((poly) => {
      if (!Array.isArray(poly)) return []
      return poly
        .map((p) => {
          const o = p as { x?: unknown; y?: unknown }
          return { x: Number(o.x) || 0, y: Number(o.y) || 0 }
        })
        .filter((p) => Number.isFinite(p.x) && Number.isFinite(p.y))
    })
    .filter((poly) => poly.length >= 3)
}

function regionsEqual(a: Point2D[][], b: Point2D[][]) {
  return JSON.stringify(a) === JSON.stringify(b)
}

function rulesEqual(a: Record<string, unknown>, b: Record<string, unknown>) {
  return JSON.stringify(a) === JSON.stringify(b)
}

function groupRuleFields(fields: AlgorithmRuleField[]) {
  const groups: Array<{ title: string; fields: AlgorithmRuleField[] }> = []
  for (const field of fields) {
    const title = field.group?.trim() || '参数'
    const last = groups[groups.length - 1]
    if (last && last.title === title) {
      last.fields.push(field)
    } else {
      groups.push({ title, fields: [field] })
    }
  }
  return groups
}

/** 与后端 vision.DefaultBindingRulesMap 一致；仅用于表单展示补齐缺失键 */
const DEFAULT_BINDING_RULES: Record<string, unknown> = {
  conf: 0.4,
  iou: 0.45,
  max_det: 100,
  pull_keyframe_only: false,
  pull_sample_ms: 100,
  pull_max_gop_mb: 8,
  enable_track: true,
  alarm_level: 'warn',
  cooldown_sec: 30,
}

function mergeRulesWithDefaults(
  stored: Record<string, unknown> | undefined,
): Record<string, unknown> {
  const out = { ...DEFAULT_BINDING_RULES }
  for (const [k, v] of Object.entries(stored ?? {})) {
    if (v === undefined || v === null || v === '') continue
    out[k] = v
  }
  return out
}

/** 按码流配置 AI：选算法绑定 → 画区域 → 一键启停 */
export function DeviceAiPage({
  cameraId,
  channelId,
}: {
  cameraId: string
  channelId: string
}) {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [rules, setRules] = useState<Record<string, unknown>>({})
  const [regions, setRegions] = useState<Point2D[][]>([])
  const [bindAlgId, setBindAlgId] = useState('')
  /** 已有绑定时，展开「再绑一个」表单 */
  const [addingBind, setAddingBind] = useState(false)
  const [errors, setErrors] = useState<AiConfigErrors>({ fields: {} })
  const [touched, setTouched] = useState(false)
  const baselineRef = useRef<{ rules: Record<string, unknown>; regions: Point2D[][] }>({
    rules: {},
    regions: [],
  })

  const algoQueryKey = ['camera-algorithms', cameraId, channelId] as const

  const cameraQuery = useQuery({
    queryKey: ['camera', cameraId],
    queryFn: () => getCamera(cameraId),
  })
  const algosQuery = useQuery({
    queryKey: algoQueryKey,
    queryFn: () => listCameraAlgorithms(cameraId, channelId),
  })
  const catalogQuery = useQuery({
    queryKey: ['algorithms'],
    queryFn: () => listAlgorithms(),
  })

  const boundList = algosQuery.data ?? []
  const selected =
    boundList.find((a) => a.id === selectedId) ?? boundList[0] ?? null

  const channel = useMemo(() => {
    const cam = cameraQuery.data
    if (!cam) return null
    return cam.channels?.find((c) => String(c.id) === channelId) ?? null
  }, [cameraQuery.data, channelId])

  const streamLabel = useMemo(() => {
    const cam = cameraQuery.data
    if (!cam) return channelId
    const name = channel?.name?.trim()
    if (name) return name
    if (channelId === String(cam.id)) return '默认码流'
    return channelId
  }, [cameraQuery.data, channel, channelId])

  const streamSrc = useMemo(() => {
    const cam = cameraQuery.data
    if (!cam) return ''
    return resolveStreamSrc(cam, channelId, 'sub')
  }, [cameraQuery.data, channelId])

  useEffect(() => {
    setSelectedId(null)
    setBindAlgId('')
    setAddingBind(false)
  }, [cameraId, channelId])

  useEffect(() => {
    if (!selectedId && boundList.length > 0) {
      setSelectedId(boundList[0].id)
    }
    if (selectedId && !boundList.some((a) => a.id === selectedId)) {
      setSelectedId(boundList[0]?.id ?? null)
    }
  }, [boundList, selectedId])

  useEffect(() => {
    if (!selected) {
      setRules({})
      setRegions([])
      return
    }
    const nextRules = mergeRulesWithDefaults(selected.rules)
    const nextRegions = asPointRegions(selected.regions)
    setRules(nextRules)
    setRegions(nextRegions)
    baselineRef.current = {
      rules: structuredClone(nextRules),
      regions: structuredClone(nextRegions),
    }
    setErrors({ fields: {} })
    setTouched(false)
  }, [selected?.id]) // eslint-disable-line react-hooks/exhaustive-deps

  const dirty = useMemo(() => {
    if (!selected) return false
    return (
      !rulesEqual(rules, baselineRef.current.rules) ||
      !regionsEqual(regions, baselineRef.current.regions)
    )
  }, [selected, rules, regions])

  const liveErrors = useMemo(() => {
    if (!selected) return { fields: {} } as AiConfigErrors
    return validateAiConfig(selected.ruleSchema, rules, regions)
  }, [selected, rules, regions])

  const shownErrors = touched ? liveErrors : errors

  function runValidation(): boolean {
    if (!selected) return false
    const next = validateAiConfig(selected.ruleSchema, rules, regions)
    setErrors(next)
    setTouched(true)
    if (hasAiConfigErrors(next)) {
      const first =
        next.regions ?? Object.values(next.fields)[0] ?? '请修正配置后再试'
      toast({ body: first, type: 'error' })
      return false
    }
    return true
  }

  const actionMutation = useMutation({
    mutationFn: (action: 'init' | 'start' | 'stop' | 'release') =>
      algorithmAction(cameraId, channelId, selected!.id, action),
    onSuccess: async (_algo, action) => {
      const msg =
        action === 'start'
          ? '已启动'
          : action === 'stop'
            ? '已停用'
            : action === 'init'
              ? '已就绪'
              : '已释放'
      toast({ body: msg, type: 'info' })
      await queryClient.invalidateQueries({ queryKey: algoQueryKey })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const saveMutation = useMutation({
    mutationFn: () =>
      saveCameraAlgorithm(
        cameraId,
        channelId,
        selected!.id,
        { rules, regions },
        selected?.ruleSchema,
      ),
    onSuccess: async () => {
      toast({ body: '配置已保存', type: 'info' })
      baselineRef.current = {
        rules: structuredClone(rules),
        regions: structuredClone(regions),
      }
      setErrors({ fields: {} })
      setTouched(false)
      await queryClient.invalidateQueries({ queryKey: algoQueryKey })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const bindMutation = useMutation({
    mutationFn: () => bindCameraAlgorithm(cameraId, channelId, bindAlgId),
    onSuccess: async (algo) => {
      toast({ body: `已绑定「${algo.name}」· 请在画面上画检测区域`, type: 'info' })
      setBindAlgId('')
      setAddingBind(false)
      setSelectedId(algo.id)
      await queryClient.invalidateQueries({ queryKey: algoQueryKey })
      await queryClient.invalidateQueries({ queryKey: ['algorithms'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  function switchBoundAlgo(id: string) {
    if (id === selected?.id) return
    if (dirty) {
      toast({
        body: '当前算法有未保存修改，请先保存后再切换',
        type: 'error',
      })
      return
    }
    setAddingBind(false)
    setSelectedId(id)
  }

  function openAddBind() {
    if (dirty) {
      toast({
        body: '当前算法有未保存修改，请先保存后再绑定其它算法',
        type: 'error',
      })
      return
    }
    setBindAlgId('')
    setAddingBind(true)
  }

  const unbindMutation = useMutation({
    mutationFn: () => unbindCameraAlgorithm(cameraId, channelId, selected!.id),
    onSuccess: async () => {
      toast({ body: '已解绑算法', type: 'info' })
      setSelectedId(null)
      await queryClient.invalidateQueries({ queryKey: algoQueryKey })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  async function saveThenStart() {
    if (!runValidation()) return
    if (dirty) await saveMutation.mutateAsync()
    await actionMutation.mutateAsync('start')
  }

  const camera = cameraQuery.data
  const catalogItems = catalogQuery.data?.items ?? []
  const boundIds = new Set(boundList.map((a) => a.algorithmId))
  const bindOptions = catalogItems
    .filter((a) => a.status === 'ready' && !boundIds.has(a.id))
    .map((a) => ({ label: `${a.name}（${a.category}）`, value: a.id }))
  const catalogLoading = catalogQuery.isLoading
  const canBind = bindOptions.length > 0

  const busy =
    actionMutation.isPending ||
    saveMutation.isPending ||
    unbindMutation.isPending ||
    bindMutation.isPending

  const primaryCta = useMemo(() => {
    if (!selected) return null
    if (selected.status === 'running') {
      return {
        kind: 'stop' as const,
        label: '停用',
        disabled: false,
        loading: actionMutation.isPending,
      }
    }
    return {
      kind: 'start' as const,
      label: '启动',
      disabled: false,
      loading: busy && (actionMutation.isPending || saveMutation.isPending),
    }
  }, [
    selected,
    actionMutation.isPending,
    saveMutation.isPending,
    busy,
  ])

  return (
    <PageLayout
      title={camera ? `视频 AI · ${camera.name}` : '视频 AI 配置'}
      description={`${streamLabel} · 可绑多个算法 · 整帧或画区域 → 启动`}
      actions={
        camera ? (
          <div className="ff-ai-stream-meta">
            <span>{streamLabel}</span>
            <span aria-hidden>·</span>
            <span>{channelId}</span>
            <span aria-hidden>·</span>
            <span>
              {camera.ip}:{camera.port}
            </span>
            {camera.vendor ? (
              <>
                <span aria-hidden>·</span>
                <span>{camera.vendor}</span>
              </>
            ) : null}
            <CameraStatusBadge status={camera.status} />
          </div>
        ) : null
      }
    >
      <div className="ff-ai-workspace ff-ai-workspace--stage">
        <section className="ff-ai-canvas-col">
          {!selected ? (
            <div className="ff-ai-bind-stage">
              <RegionEditor
                regions={[]}
                streamSrc={streamSrc}
                onChange={() => {}}
                cameraLabel={`${camera?.name ?? ''} · ${streamLabel}`}
                readOnly
              />
              <div className="ff-ai-bind-overlay">
                <div className="ff-ai-bind-card">
                  <strong>绑定算法到本流</strong>
                  <Text type="supporting" color="secondary">
                    {catalogLoading
                      ? '正在加载可用算法…'
                      : catalogQuery.isError
                        ? '算法库加载失败'
                        : canBind
                          ? '选择一个「可用」算法绑定到本流'
                          : '暂无可用算法，请先到算法管理创建并将状态设为可用'}
                  </Text>
                  {canBind ? (
                    <>
                      <Selector
                        label="算法"
                        placeholder="选择算法"
                        value={bindAlgId}
                        onChange={(v) => setBindAlgId(v ?? '')}
                        options={bindOptions}
                      />
                      <Button
                        label="绑定到本流"
                        size="sm"
                        variant="primary"
                        isDisabled={!bindAlgId}
                        isLoading={bindMutation.isPending}
                        clickAction={async () => {
                          try {
                            await bindMutation.mutateAsync()
                          } catch {
                            /* onError */
                          }
                        }}
                      />
                    </>
                  ) : catalogQuery.isError ? (
                    <Button
                      label="重新加载"
                      size="sm"
                      variant="secondary"
                      clickAction={async () => {
                        await catalogQuery.refetch()
                      }}
                    />
                  ) : (
                    <Link to="/algorithms">
                      <Button label="去算法管理" size="sm" variant="secondary" />
                    </Link>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <>
              <RegionEditor
                regions={regions}
                streamSrc={streamSrc}
                onChange={(next) => {
                  setRegions(next)
                  if (touched) {
                    setErrors(
                      validateAiConfig(selected.ruleSchema, rules, next),
                    )
                  }
                }}
                cameraLabel={`${camera?.name ?? ''} · ${streamLabel}`}
              />
              {shownErrors.regions ? (
                <div className="ff-ai-field-err">
                  <FieldStatus type="error" message={shownErrors.regions} />
                </div>
              ) : null}
            </>
          )}
        </section>

        <aside className="ff-ai-params-col">
          {boundList.length === 0 ? (
            <div className="ff-ai-side-empty">
              <p>等待绑定</p>
              <Text type="supporting" color="secondary">
                左侧绑定算法后，可在此调整置信度等参数；多算法时点开对应项。
              </Text>
              <Link to="/algorithms">
                <Button label="算法管理" size="sm" variant="ghost" />
              </Link>
            </div>
          ) : (
            <div className="ff-ai-algo-list" role="list">
              {boundList.map((algo) => {
                const open = selected?.id === algo.id
                return (
                  <div
                    key={algo.id}
                    className="ff-ai-algo-fold"
                    data-open={open.toString()}
                    role="listitem"
                  >
                    <button
                      type="button"
                      className="ff-ai-algo-fold-head"
                      aria-expanded={open}
                      onClick={() => {
                        if (open) return
                        switchBoundAlgo(algo.id)
                      }}
                    >
                      <span className="ff-ai-algo-fold-title">{algo.name}</span>
                      <Badge
                        label={statusLabel[algo.status]}
                        variant={statusVariant(algo.status)}
                      />
                      {open && dirty ? (
                        <Badge label="未保存" variant="warning" />
                      ) : null}
                      <span className="ff-ai-algo-fold-chevron" aria-hidden>
                        {open ? '▾' : '▸'}
                      </span>
                    </button>

                    {open && selected ? (
                      <div className="ff-ai-algo-fold-body">
                        {groupRuleFields(selected.ruleSchema).map((group) => (
                          <div key={group.title} className="ff-ai-param-group">
                            <div className="ff-ai-param-group-title">
                              {group.title}
                            </div>
                            <VStack gap={3}>
                              {group.fields.map((field) => {
                                const fieldErr = shownErrors.fields[field.key]
                                const status = fieldErr
                                  ? {
                                      type: 'error' as const,
                                      message: fieldErr,
                                    }
                                  : undefined

                                if (field.type === 'boolean') {
                                  return (
                                    <Switch
                                      key={field.key}
                                      label={field.label}
                                      description={field.description}
                                      value={Boolean(rules[field.key])}
                                      onChange={(v) =>
                                        setRules((r) => ({
                                          ...r,
                                          [field.key]: v,
                                        }))
                                      }
                                    />
                                  )
                                }
                                if (field.type === 'select') {
                                  return (
                                    <Selector
                                      key={field.key}
                                      label={field.label}
                                      description={field.description}
                                      value={String(rules[field.key] ?? '')}
                                      onChange={(v) =>
                                        setRules((r) => ({
                                          ...r,
                                          [field.key]: v,
                                        }))
                                      }
                                      options={field.options ?? []}
                                      status={status}
                                    />
                                  )
                                }
                                if (field.type === 'number') {
                                  const num = rules[field.key]
                                  return (
                                    <NumberInput
                                      key={field.key}
                                      label={field.label}
                                      description={field.description}
                                      value={
                                        typeof num === 'number' &&
                                        !Number.isNaN(num)
                                          ? num
                                          : null
                                      }
                                      min={field.min ?? null}
                                      max={field.max ?? null}
                                      step={field.step ?? 0.01}
                                      status={status}
                                      onChange={(v) =>
                                        setRules((r) => ({
                                          ...r,
                                          [field.key]: v ?? '',
                                        }))
                                      }
                                    />
                                  )
                                }
                                return (
                                  <TextInput
                                    key={field.key}
                                    label={field.label}
                                    description={field.description}
                                    value={String(rules[field.key] ?? '')}
                                    status={status}
                                    onChange={(v) =>
                                      setRules((r) => ({
                                        ...r,
                                        [field.key]: v,
                                      }))
                                    }
                                  />
                                )
                              })}
                            </VStack>
                          </div>
                        ))}

                        <div className="ff-ai-algo-fold-footer">
                          <HStack gap={2} wrap="wrap">
                            {dirty ? (
                              <Button
                                label="保存配置"
                                size="sm"
                                variant="ghost"
                                isDisabled={busy}
                                isLoading={saveMutation.isPending}
                                clickAction={async () => {
                                  if (!runValidation()) return
                                  try {
                                    await saveMutation.mutateAsync()
                                  } catch {
                                    /* onError */
                                  }
                                }}
                              />
                            ) : null}
                            {primaryCta ? (
                              <Button
                                label={primaryCta.label}
                                size="sm"
                                variant={
                                  primaryCta.kind === 'stop'
                                    ? 'secondary'
                                    : 'primary'
                                }
                                isDisabled={primaryCta.disabled || busy}
                                isLoading={primaryCta.loading}
                                clickAction={async () => {
                                  try {
                                    if (primaryCta.kind === 'stop') {
                                      await actionMutation.mutateAsync('stop')
                                      return
                                    }
                                    await saveThenStart()
                                  } catch {
                                    /* onError toast */
                                  }
                                }}
                              />
                            ) : null}
                            <Button
                              label="解绑"
                              size="sm"
                              variant="ghost"
                              isDisabled={busy || selected.status === 'running'}
                              clickAction={async () => {
                                if (selected.status === 'running') {
                                  toast({
                                    body: '请先停用再解绑',
                                    type: 'error',
                                  })
                                  return
                                }
                                try {
                                  await unbindMutation.mutateAsync()
                                } catch {
                                  /* onError */
                                }
                              }}
                            />
                          </HStack>
                        </div>
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </div>
          )}

          {addingBind ? (
            <div className="ff-ai-add-bind">
              <Text type="supporting">再绑定一个算法到本流</Text>
              <div className="ff-ai-add-bind-select">
                <Selector
                  label="算法"
                  placeholder="选择算法"
                  value={bindAlgId}
                  onChange={(v) => setBindAlgId(v ?? '')}
                  options={bindOptions}
                />
              </div>
              <HStack gap={2} wrap="wrap">
                <Button
                  label="绑定"
                  size="sm"
                  variant="primary"
                  isDisabled={!bindAlgId}
                  isLoading={bindMutation.isPending}
                  clickAction={async () => {
                    try {
                      await bindMutation.mutateAsync()
                    } catch {
                      /* onError */
                    }
                  }}
                />
                <Button
                  label="取消"
                  size="sm"
                  variant="ghost"
                  isDisabled={bindMutation.isPending}
                  onClick={() => {
                    setAddingBind(false)
                    setBindAlgId('')
                  }}
                />
              </HStack>
            </div>
          ) : canBind && boundList.length > 0 ? (
            <Button
              label="再绑定一个算法"
              size="sm"
              variant="ghost"
              isDisabled={busy}
              onClick={openAddBind}
            />
          ) : null}
        </aside>
      </div>
    </PageLayout>
  )
}
