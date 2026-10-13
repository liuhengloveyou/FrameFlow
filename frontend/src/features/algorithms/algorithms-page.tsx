import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button } from '@astryxdesign/core/Button'
import { Card } from '@astryxdesign/core/Card'
import { FileInput } from '@astryxdesign/core/FileInput'
import { NumberInput } from '@astryxdesign/core/NumberInput'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import {
  CheckCircleIcon,
  LinkIcon,
  NoSymbolIcon,
  PencilSquareIcon,
} from '@heroicons/react/24/outline'
import { PageLayout } from '@/components/page-layout'
import { PageToolbar } from '@/components/page-toolbar'
import { InlineStat, InlineStatGroup } from '@/components/inline-stat'
import { ToneBadge, type Tone } from '@/components/tone-badge'
import { SidePanel } from '@/components/side-panel'
import {
  createAlgorithm,
  listAlgorithms,
  runAlgorithmInferTest,
  updateAlgorithm,
  deleteAlgorithm,
} from '@/apis/algorithms-api'
import { listModels } from '@/apis/models-api'
import { getErrorMessage } from '@/lib/http'
import type {
  AlgorithmCapability,
  AlgorithmInferTestResult,
  AlgorithmInput,
  ModelItem,
} from '@/types/api'

const STATUS_LABEL: Record<AlgorithmCapability['status'], string> = {
  ready: '可用',
  draft: '草稿',
  disabled: '已停用',
}

const STATUS_TONE: Record<AlgorithmCapability['status'], Tone> = {
  ready: 'ok',
  draft: 'warn',
  disabled: 'off',
}

const emptyForm: AlgorithmInput = {
  name: '',
  description: '',
  category: '安防检测',
  modelId: '',
  tags: [],
  status: 'draft',
}

function preferModelId(models: ModelItem[]): string {
  const ready = models.filter((m) => m.modelStatus === 'ready' || m.localVersion)
  return (ready[0] ?? models[0])?.id ?? ''
}

function fillFromModel(
  prev: AlgorithmInput,
  model: ModelItem | undefined,
  forceName: boolean,
): AlgorithmInput {
  if (!model) return { ...prev, modelId: '' }
  return {
    ...prev,
    modelId: model.id,
    name:
      forceName || !prev.name.trim()
        ? `${model.name} 算法`
        : prev.name,
    description:
      forceName || !prev.description.trim()
        ? `基于「${model.name}」的能力，可绑定到摄像头通道。检测参数在设备绑定中按路配置。`
        : prev.description,
    category: prev.category || '安防检测',
  }
}

function parseTags(text: string): string[] {
  return text
    .split(/[,，\s]+/)
    .map((t) => t.trim())
    .filter(Boolean)
}

export function AlgorithmsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [q, setQ] = useState('')
  const [category, setCategory] = useState('')
  const [status, setStatus] = useState('')
  const [selected, setSelected] = useState<AlgorithmCapability | null>(null)
  /** 侧栏：编辑已有 / 新建（详情与编辑合并为同一表单） */
  const [panelMode, setPanelMode] = useState<'edit' | 'create'>('edit')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [form, setForm] = useState<AlgorithmInput>(emptyForm)
  const [tagText, setTagText] = useState('')
  const [testSample, setTestSample] = useState<File | null>(null)
  const [testConf, setTestConf] = useState(0.4)
  const [inferResult, setInferResult] = useState<AlgorithmInferTestResult | null>(null)
  const [testPanelOpen, setTestPanelOpen] = useState(false)

  const testLocalPreviewUrl = useMemo(() => {
    if (!testSample || !testSample.type.startsWith('image/')) return null
    return URL.createObjectURL(testSample)
  }, [testSample])

  const testPreviewUrl = useMemo(() => {
    if (inferResult?.previewJpegBase64) {
      return `data:image/jpeg;base64,${inferResult.previewJpegBase64}`
    }
    return testLocalPreviewUrl
  }, [inferResult?.previewJpegBase64, testLocalPreviewUrl])

  useEffect(() => {
    return () => {
      if (testLocalPreviewUrl) URL.revokeObjectURL(testLocalPreviewUrl)
    }
  }, [testLocalPreviewUrl])

  const query = useQuery({
    queryKey: ['algorithms', { q, category, status }],
    queryFn: () =>
      listAlgorithms({
        q: q || undefined,
        category: category || undefined,
        status: status || undefined,
      }),
  })

  const modelsQuery = useQuery({
    queryKey: ['models'],
    queryFn: listModels,
  })

  const models = modelsQuery.data?.items ?? []
  const panelOpen = panelMode === 'create' || Boolean(selected)

  const closePanel = () => {
    setSelected(null)
    setPanelMode('edit')
    setEditingId(null)
    setInferResult(null)
    setTestSample(null)
    setTestPanelOpen(false)
  }

  const fillFormFromAlgo = (alg: AlgorithmCapability) => {
    setForm({
      name: alg.name,
      description: alg.description,
      category: alg.category,
      modelId: alg.modelId,
      tags: [...alg.tags],
      status: alg.status,
    })
    setTagText(alg.tags.join(', '))
    setTestConf(0.4)
  }

  const openAlgorithm = (alg: AlgorithmCapability) => {
    setSelected(alg)
    setEditingId(alg.id)
    fillFormFromAlgo(alg)
    setInferResult(null)
    setTestSample(null)
    setPanelMode('edit')
  }

  const openCreate = () => {
    const modelId = preferModelId(models)
    const model = models.find((m) => m.id === modelId)
    const next = fillFromModel({ ...emptyForm }, model, true)
    setSelected(null)
    setEditingId(null)
    setForm(next)
    setTagText('detection')
    setInferResult(null)
    setTestSample(null)
    setPanelMode('create')
  }

  const createMutation = useMutation({
    mutationFn: () =>
      createAlgorithm({
        ...form,
        tags: parseTags(tagText),
      }),
    onSuccess: async (algo) => {
      toast({ body: `算法「${algo.name}」已创建`, type: 'info' })
      await queryClient.invalidateQueries({ queryKey: ['algorithms'] })
      openAlgorithm(algo)
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editingId) throw new Error('未选择要编辑的算法')
      return updateAlgorithm(editingId, {
        ...form,
        tags: parseTags(tagText),
      })
    },
    onSuccess: async (algo) => {
      toast({ body: `算法「${algo.name}」已保存`, type: 'info' })
      setSelected(algo)
      setEditingId(algo.id)
      fillFormFromAlgo(algo)
      await queryClient.invalidateQueries({ queryKey: ['algorithms'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteAlgorithm(id),
    onSuccess: async () => {
      toast({ body: '算法已删除', type: 'info' })
      closePanel()
      await queryClient.invalidateQueries({ queryKey: ['algorithms'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const inferMutation = useMutation({
    mutationFn: () => {
      if (!testSample) throw new Error('请选择测试图片或视频')
      return runAlgorithmInferTest(selected!.id, {
        file: testSample,
        conf: testConf,
      })
    },
    onSuccess: (res) => {
      setInferResult(res)
      toast({
        body: `${res.message} · ${res.detections.length} 个目标 · ${res.latencyMs}ms`,
        type: 'info',
      })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const categories = useMemo(() => {
    const set = new Set((query.data?.items ?? []).map((a) => a.category))
    return Array.from(set)
  }, [query.data?.items])

  const allCategories = useMemo(() => {
    const base = ['安防检测', '行为分析', 'OCR', '人脸']
    return Array.from(new Set([...base, ...categories]))
  }, [categories])

  const modelOptions = useMemo(() => {
    const opts = models.map((m) => ({
      label: `${m.name} · ${m.version}${m.modelStatus === 'ready' ? '' : ` (${m.modelStatus})`}`,
      value: m.id,
    }))
    if (form.modelId && !opts.some((o) => o.value === form.modelId)) {
      const orphanName =
        selected?.modelId === form.modelId ? selected.modelName : form.modelId
      opts.unshift({
        label: `${orphanName}（仓库中已不存在）`,
        value: form.modelId,
      })
    }
    return opts
  }, [models, form.modelId, selected])

  const stats = useMemo(() => {
    const items = query.data?.items ?? []
    return {
      total: query.data?.total ?? items.length,
      ready: items.filter((a) => a.status === 'ready').length,
      draft: items.filter((a) => a.status === 'draft').length,
      disabled: items.filter((a) => a.status === 'disabled').length,
      bound: items.reduce((n, a) => n + a.boundCameraCount, 0),
    }
  }, [query.data])

  const algorithmFormFields = (mode: 'create' | 'edit') => (
    <VStack gap={3}>
      <Text type="supporting" color="secondary">
        {mode === 'create'
          ? '选择模型仓库中的权重，封装为可绑定到摄像头的检测能力。'
          : '修改后点保存；已运行的流需重新启动算法后才会用上新参数。'}
      </Text>
      <TextInput
        label="名称"
        value={form.name}
        onChange={(v) => setForm((f) => ({ ...f, name: v }))}
        placeholder="例如：周界入侵检测"
      />
      <TextInput
        label="描述"
        value={form.description}
        onChange={(v) => setForm((f) => ({ ...f, description: v }))}
        placeholder="简要说明检测目标与场景"
      />
      <Selector
        label="关联模型"
        placeholder={models.length ? '选择模型' : '暂无模型，请先接入'}
        value={form.modelId}
        onChange={(v) => {
          const id = v ?? ''
          if (mode === 'edit') {
            setForm((f) => ({ ...f, modelId: id }))
            return
          }
          const model = models.find((m) => m.id === id)
          setForm((f) => fillFromModel(f, model, !f.name.trim()))
        }}
        options={modelOptions}
      />
      {!models.length ? (
        <Text type="supporting" color="secondary">
          模型仓库为空。请先到 <Link to="/models">模型仓库</Link> 上传权重后再新建算法。
        </Text>
      ) : null}
      <Selector
        label="分类"
        value={form.category}
        onChange={(v) => setForm((f) => ({ ...f, category: v || '安防检测' }))}
        options={allCategories.map((c) => ({ label: c, value: c }))}
      />
      <Selector
        label="状态"
        value={form.status}
        onChange={(v) =>
          setForm((f) => ({
            ...f,
            status: (v as AlgorithmCapability['status']) || 'draft',
          }))
        }
        options={[
          { label: '可用', value: 'ready' },
          { label: '草稿', value: 'draft' },
          { label: '已停用', value: 'disabled' },
        ]}
      />
      <Text type="supporting" color="secondary">
        置信度、NMS、取样间隔等检测参数在「视频 AI」设备绑定中按路配置，不在算法上统一设置。
      </Text>
      <TextInput
        label="标签"
        value={tagText}
        onChange={setTagText}
        placeholder="detection, ocr, region"
        description="逗号分隔，便于筛选"
      />
      {mode === 'edit' && selected ? (
        <>
          <Text type="supporting" color="secondary">
            已绑定{' '}
            {selected.boundCameraCount > 0
              ? `${selected.boundCameraCount} 路`
              : '0 路'}
            {' · '}
            <Link to="/devices">去设备绑定</Link>
            {' · '}
            <Link to="/models">查看模型</Link>
            {selected.updatedAt
              ? ` · 更新于 ${new Date(selected.updatedAt).toLocaleString()}`
              : null}
          </Text>
          <details
            className="ff-algo-detail-test"
            open={testPanelOpen || Boolean(inferResult)}
            onToggle={(e) => setTestPanelOpen((e.target as HTMLDetailsElement).open)}
          >
            <summary>试跑检测（可选）</summary>
            <div className="ff-algo-detail-test-body">
              <Text type="supporting" color="secondary">
                上传图片或短视频，用本算法关联模型做本地检测（视频会抽一帧推理），确认配置是否可用。
              </Text>
              <FileInput
                label="测试样张"
                accept="image/jpeg,image/jpg,image/png,.jpg,.jpeg,.png,video/mp4,video/webm,video/quicktime,.mp4,.webm,.mov,.m4v"
                value={testSample}
                onChange={(f) => {
                  const picked = Array.isArray(f) ? f[0] ?? null : f
                  setTestSample(picked)
                  setInferResult(null)
                  if (picked) setTestPanelOpen(true)
                }}
              />
              <NumberInput
                label="试跑置信度"
                value={testConf}
                min={0}
                max={1}
                step={0.05}
                onChange={(v) => setTestConf(v ?? 0.4)}
              />
              <Button
                label="运行试跑"
                size="sm"
                variant="secondary"
                isDisabled={form.status !== 'ready' || !testSample}
                isLoading={inferMutation.isPending}
                clickAction={async () => {
                  try {
                    setTestPanelOpen(true)
                    await inferMutation.mutateAsync()
                  } catch {
                    /* onError toast */
                  }
                }}
              />
              {form.status !== 'ready' ? (
                <Text type="supporting" color="secondary">
                  算法需为「可用」才能试跑（可先改状态并保存）
                </Text>
              ) : null}
              {inferResult ? (
                <div className="ff-infer-result">
                  <Text type="supporting">
                    {inferResult.message} · {inferResult.latencyMs}ms ·{' '}
                    {inferResult.detections.length} 个目标
                    {inferResult.sampleKind === 'video' ? ' · 视频抽帧' : null}
                  </Text>
                  {testPreviewUrl ? (
                    <div className="ff-infer-preview">
                      <img
                        src={testPreviewUrl}
                        alt="试跑预览"
                        className="ff-infer-preview-img"
                      />
                      <div className="ff-infer-preview-overlay" aria-hidden>
                        {inferResult.detections.map((d, i) => {
                          const [x1, y1, x2, y2] = d.bbox
                          const left = (x1 / 800) * 100
                          const top = (y1 / 520) * 100
                          const width = ((x2 - x1) / 800) * 100
                          const height = ((y2 - y1) / 520) * 100
                          return (
                            <div
                              key={i}
                              className="ff-infer-box-item"
                              style={{
                                left: `${left}%`,
                                top: `${top}%`,
                                width: `${Math.max(width, 0)}%`,
                                height: `${Math.max(height, 0)}%`,
                              }}
                            >
                              {d.cls || `目标${i + 1}`}{' '}
                              {(d.conf * 100).toFixed(0)}%
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  ) : null}
                  <ul className="ff-algo-detail-detect-list">
                    {inferResult.detections.map((d, i) => (
                      <li key={i}>
                        {d.cls || `class_${i}`} · {(d.conf * 100).toFixed(0)}%
                      </li>
                    ))}
                    {!inferResult.detections.length ? (
                      <li>没有超过阈值的目标</li>
                    ) : null}
                  </ul>
                </div>
              ) : null}
            </div>
          </details>
        </>
      ) : null}
    </VStack>
  )

  return (
    <PageLayout
      title="算法接入"
      badge={`可用 ${stats.ready}/${stats.total}`}
    >
          <PageToolbar
            filters={
              <>
                <TextInput
                  label="搜索"
                  isLabelHidden
                  placeholder="名称 / 描述 / 标签"
                  value={q}
                  onChange={setQ}
                  hasClear
                />
                <Selector
                  label="分类"
                  isLabelHidden
                  placeholder="全部分类"
                  hasClear
                  value={category}
                  onChange={(v) => setCategory(v ?? '')}
                  options={allCategories.map((c) => ({ label: c, value: c }))}
                />
                <Selector
                  label="状态"
                  isLabelHidden
                  placeholder="全部状态"
                  hasClear
                  value={status}
                  onChange={(v) => setStatus(v ?? '')}
                  options={[
                    { label: '可用', value: 'ready' },
                    { label: '草稿', value: 'draft' },
                    { label: '已停用', value: 'disabled' },
                  ]}
                />
              </>
            }
            stats={
              <InlineStatGroup>
                <InlineStat
                  icon={<CheckCircleIcon />}
                  value={stats.ready}
                  label="可用算法"
                  tone="ok"
                />
                <InlineStat
                  icon={<PencilSquareIcon />}
                  value={stats.draft}
                  label="草稿"
                  tone="warn"
                />
                <InlineStat
                  icon={<NoSymbolIcon />}
                  value={stats.disabled}
                  label="已停用"
                />
                <InlineStat
                  icon={<LinkIcon />}
                  value={stats.bound}
                  label="已绑定通道"
                  tone="teal"
                />
              </InlineStatGroup>
            }
            actions={
              <Button
                label="新建算法"
                variant="primary"
                size="sm"
                onClick={openCreate}
              />
            }
          />

          <div className="ff-model-grid">
            {(query.data?.items ?? []).map((alg) => (
              <Card key={alg.id} padding={4}>
                <VStack gap={3} className="ff-model-card">
                  <div>
                    <strong className="ff-model-card-title">{alg.name}</strong>
                    <p className="ff-model-card-desc">{alg.description}</p>
                  </div>
                  <div className="ff-model-card-badges">
                    <ToneBadge tone={STATUS_TONE[alg.status] ?? 'off'}>
                      {STATUS_LABEL[alg.status]}
                    </ToneBadge>
                    <ToneBadge tone="teal">{alg.category}</ToneBadge>
                    {alg.tags.slice(0, 3).map((t) => (
                      <ToneBadge key={t} tone="off">
                        {t}
                      </ToneBadge>
                    ))}
                  </div>
                  <div className="ff-model-card-meta">
                    <div className="ff-model-card-meta-row">
                      <span className="ff-model-card-meta-label">模型</span>
                      <span className="ff-model-card-meta-value">{alg.modelName}</span>
                    </div>
                    <div className="ff-model-card-meta-row">
                      <span className="ff-model-card-meta-label">绑定</span>
                      <span className="ff-model-card-meta-value">
                        {alg.boundCameraCount} 路 · 检测参数在设备绑定中配置
                      </span>
                    </div>
                  </div>
                  <div className="ff-model-card-versions">
                    <span>绑定 {alg.boundCameraCount} 路</span>
                    <span>更新 {new Date(alg.updatedAt).toLocaleDateString()}</span>
                  </div>
                  <div className="ff-model-card-actions">
                    <Button
                      label="配置"
                      size="sm"
                      variant="secondary"
                      onClick={() => openAlgorithm(alg)}
                    />
                    <Button
                      label="删除"
                      size="sm"
                      variant="ghost"
                      isLoading={deleteMutation.isPending}
                      clickAction={async () => {
                        if (alg.boundCameraCount > 0) {
                          toast({
                            body: `仍有 ${alg.boundCameraCount} 路绑定，请先在设备侧解绑`,
                            type: 'error',
                          })
                          return
                        }
                        if (!window.confirm(`确认删除算法「${alg.name}」？`)) return
                        try {
                          await deleteMutation.mutateAsync(alg.id)
                        } catch {
                          /* onError toast */
                        }
                      }}
                    />
                  </div>
                </VStack>
              </Card>
            ))}
          </div>
          {!query.isLoading && (query.data?.items.length ?? 0) === 0 ? (
            <div className="ff-placeholder">
              无匹配算法。可先到{' '}
              <Link to="/models">模型仓库</Link> 接入权重，再点击「新建算法」。
            </div>
          ) : null}

          <SidePanel
            open={panelOpen}
            title={
              panelMode === 'create'
                ? '新建算法'
                : (selected?.name ?? '算法配置')
            }
            onClose={closePanel}
            width="min(440px, 100vw)"
            footer={
              <HStack gap={2} wrap="wrap" justify="end">
                {panelMode === 'edit' && selected ? (
                  <Button
                    label="删除"
                    size="sm"
                    variant="destructive"
                    isLoading={deleteMutation.isPending}
                    clickAction={async () => {
                      if (selected.boundCameraCount > 0) {
                        toast({
                          body: `仍有 ${selected.boundCameraCount} 路绑定，请先在设备侧解绑`,
                          type: 'error',
                        })
                        return
                      }
                      if (
                        !window.confirm(
                          `确认删除算法「${selected.name}」？此操作不可恢复。`,
                        )
                      ) {
                        return
                      }
                      try {
                        await deleteMutation.mutateAsync(selected.id)
                      } catch {
                        /* onError toast */
                      }
                    }}
                  />
                ) : null}
                <Button label="取消" size="sm" variant="secondary" onClick={closePanel} />
                <Button
                  label={panelMode === 'create' ? '创建' : '保存'}
                  size="sm"
                  variant="primary"
                  isLoading={
                    panelMode === 'create'
                      ? createMutation.isPending
                      : updateMutation.isPending
                  }
                  isDisabled={!form.name.trim() || !form.modelId}
                  clickAction={async () => {
                    try {
                      if (panelMode === 'create') {
                        await createMutation.mutateAsync()
                      } else {
                        await updateMutation.mutateAsync()
                      }
                    } catch {
                      /* onError toast */
                    }
                  }}
                />
              </HStack>
            }
          >
            {algorithmFormFields(panelMode)}
          </SidePanel>
    </PageLayout>
  )
}
