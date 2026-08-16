import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@astryxdesign/core/Button'
import { Card } from '@astryxdesign/core/Card'
import { Badge } from '@astryxdesign/core/Badge'
import { FileInput } from '@astryxdesign/core/FileInput'
import { TextInput } from '@astryxdesign/core/TextInput'
import { Selector } from '@astryxdesign/core/Selector'
import { Dialog, DialogHeader } from '@astryxdesign/core/Dialog'
import { Layout } from '@astryxdesign/core/Layout'
import { Tooltip } from '@astryxdesign/core/Tooltip'
import { HStack } from '@astryxdesign/core/HStack'
import { VStack } from '@astryxdesign/core/VStack'
import { Text } from '@astryxdesign/core/Text'
import { useToast } from '@astryxdesign/core/Toast'
import { PageLayout } from '@/components/page-layout'
import { SidePanel } from '@/components/side-panel'
import { DataTable, proportional, pixel, type DataTableColumn } from '@/components/data-table'
import { LicenseStatusBadge } from '@/components/status-badge'
import {
  checkModelHealth,
  deleteModel,
  downloadModel,
  listModels,
  listModelUpdates,
  uploadModel,
} from '@/apis/models-api'
import { getErrorMessage } from '@/lib/http'
import type {
  ModelFormat,
  ModelItem,
  ModelLicenseStatus,
  ModelRegisterInput,
  ModelUpdateRecord,
} from '@/types/api'

const emptyRegister: ModelRegisterInput = {
  name: '',
  description: '',
  version: '1.0.0',
  formats: ['ONNX'],
  tags: [],
  fileName: '',
}

const MAX_MODEL_BYTES = 512 * 1024 * 1024

function formatBytes(n?: number) {
  if (n == null || n <= 0) return '—'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

function pickFormat(fileName: string, fallback: ModelFormat): ModelFormat {
  const lower = fileName.toLowerCase()
  if (lower.endsWith('.onnx')) return 'ONNX'
  if (lower.endsWith('.rknn')) return 'RKNN'
  if (
    lower.endsWith('.engine') ||
    lower.endsWith('.plan') ||
    lower.endsWith('.trt')
  ) {
    return 'TensorRT'
  }
  return fallback
}

/** 根据文件名启发式提示权重与配套文件可能不匹配（不阻断上传） */
function companionMismatchHint(
  modelName?: string | null,
  labelsName?: string | null,
): string | null {
  if (!modelName || !labelsName) return null
  const m = modelName.toLowerCase()
  const l = labelsName.toLowerCase()
  const modelVoc = /voc/.test(m)
  const modelCoco = /coco/.test(m)
  const labelsVoc = /voc/.test(l)
  const labelsCoco = /coco/.test(l)
  if (modelVoc && labelsCoco) {
    return '权重名含 voc，配套文件却是 coco.names，类别名可能错乱；建议改用 voc.names'
  }
  if (modelCoco && labelsVoc) {
    return '权重名含 coco，配套文件却是 voc.names，类别名可能错乱；建议改用 coco.names'
  }
  if (/ocr|ppocr|rec/.test(m) && /\.names$/.test(l) && !/key|dict|ppocr/.test(l)) {
    return 'OCR 类权重通常应配字符字典（如 ppocr_keys_v1.txt），而不是检测类别 .names'
  }
  return null
}

const UPDATE_STATUS_LABEL: Record<ModelUpdateRecord['status'], string> = {
  success: '成功',
  failed: '失败',
  running: '进行中',
}

export function ModelsPage() {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [license, setLicense] = useState('')
  const [format, setFormat] = useState('')
  const [updateStatus, setUpdateStatus] = useState('')
  const [loadingId, setLoadingId] = useState<string | null>(null)
  const [errorOpen, setErrorOpen] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')
  const [detail, setDetail] = useState<ModelItem | null>(null)
  const [registerOpen, setRegisterOpen] = useState(false)
  const [registerForm, setRegisterForm] = useState<ModelRegisterInput>(emptyRegister)
  const [tagText, setTagText] = useState('')
  const [formatPick, setFormatPick] = useState<ModelFormat>('ONNX')
  const [registerFile, setRegisterFile] = useState<File | null>(null)
  const [labelsFile, setLabelsFile] = useState<File | null>(null)
  const [uploadPct, setUploadPct] = useState(0)

  const modelsQuery = useQuery({ queryKey: ['models'], queryFn: listModels })
  const updatesQuery = useQuery({ queryKey: ['model-updates'], queryFn: listModelUpdates })

  const companionHint = useMemo(
    () => companionMismatchHint(registerFile?.name, labelsFile?.name),
    [registerFile, labelsFile],
  )

  const filteredModels = useMemo(() => {
    let items = modelsQuery.data?.items ?? []
    if (name) {
      const q = name.toLowerCase()
      items = items.filter(
        (m) =>
          m.name.toLowerCase().includes(q) ||
          m.description.toLowerCase().includes(q) ||
          m.tags.some((t) => t.toLowerCase().includes(q)),
      )
    }
    if (license) items = items.filter((m) => m.licenseStatus === license)
    if (format) items = items.filter((m) => m.formats.includes(format as ModelItem['formats'][number]))
    return items
  }, [modelsQuery.data?.items, name, license, format])

  const filteredUpdates = useMemo(() => {
    let rows = updatesQuery.data ?? []
    if (updateStatus) rows = rows.filter((r) => r.status === updateStatus)
    return rows
  }, [updatesQuery.data, updateStatus])

  const downloadMutation = useMutation({
    mutationFn: (id: string) => downloadModel(id),
    onMutate: (id) => setLoadingId(id),
    onSuccess: async (model) => {
      toast({
        body: `「${model.name}」本地文件已校验通过（${model.localVersion}）`,
        type: 'info',
      })
      await queryClient.invalidateQueries({ queryKey: ['models'] })
      await queryClient.invalidateQueries({ queryKey: ['model-updates'] })
    },
    onError: (error) => {
      const msg = getErrorMessage(error)
      setErrorMsg(msg)
      setErrorOpen(true)
      toast({ body: msg, type: 'error' })
    },
    onSettled: () => setLoadingId(null),
  })

  const registerMutation = useMutation({
    mutationFn: () => {
      if (!registerFile) throw new Error('请选择模型文件')
      setUploadPct(0)
      const tags = tagText
        .split(/[,，\s]+/)
        .map((t) => t.trim())
        .filter(Boolean)
      return uploadModel(
        {
          ...registerForm,
          name:
            registerForm.name.trim() ||
            registerFile.name.replace(/\.[^.]+$/, ''),
          formats: registerForm.formats.length
            ? registerForm.formats
            : [formatPick],
          tags,
          fileName: registerFile.name,
          fileSizeBytes: registerFile.size,
        },
        registerFile,
        labelsFile,
        setUploadPct,
      )
    },
    onSuccess: async (model) => {
      toast({
        body: `「${model.name}」已接入仓库（${formatBytes(model.fileSizeBytes)}）`,
        type: 'info',
      })
      setRegisterOpen(false)
      setRegisterForm(emptyRegister)
      setTagText('')
      setRegisterFile(null)
      setLabelsFile(null)
      setUploadPct(0)
      await queryClient.invalidateQueries({ queryKey: ['models'] })
      await queryClient.invalidateQueries({ queryKey: ['model-updates'] })
      setDetail(model)
    },
    onError: (error) => {
      setUploadPct(0)
      toast({ body: getErrorMessage(error), type: 'error' })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteModel(id),
    onSuccess: async () => {
      toast({ body: '模型已从仓库删除', type: 'info' })
      setDetail(null)
      await queryClient.invalidateQueries({ queryKey: ['models'] })
      await queryClient.invalidateQueries({ queryKey: ['model-updates'] })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  const healthMutation = useMutation({
    mutationFn: (id: string) => checkModelHealth(id),
    onSuccess: (res) => {
      toast({
        body: res.ok
          ? `权重文件可用 · ${res.latencyMs}ms`
          : res.message,
        type: res.ok ? 'info' : 'error',
      })
    },
    onError: (error) => toast({ body: getErrorMessage(error), type: 'error' }),
  })

  return (
    <PageLayout
      title="模型仓库"
      actions={
        <HStack gap={2} vAlign="center">
          <Button
            label="刷新"
            variant="secondary"
            size="sm"
            isLoading={modelsQuery.isFetching}
            clickAction={async () => {
              await modelsQuery.refetch()
              await updatesQuery.refetch()
            }}
          />
          <Button
            label="接入模型"
            variant="primary"
            size="sm"
            onClick={() => {
              setRegisterForm(emptyRegister)
              setTagText('')
              setFormatPick('ONNX')
              setRegisterFile(null)
              setLabelsFile(null)
              setUploadPct(0)
              setRegisterOpen(true)
            }}
          />
        </HStack>
      }
    >

      <div className="ff-toolbar">
        <TextInput
          label="搜索"
          isLabelHidden
          placeholder="名称 / 描述 / 标签"
          value={name}
          onChange={setName}
          hasClear
        />
        <Selector
          label="授权状态"
          isLabelHidden
          placeholder="全部授权状态"
          hasClear
          value={license}
          onChange={(v) => setLicense(v ?? '')}
          options={[
            { label: '已授权', value: 'authorized' },
            { label: '已到期', value: 'expired' },
            { label: '已关闭', value: 'disabled' },
            { label: '不可用', value: 'unavailable' },
          ] satisfies Array<{ label: string; value: ModelLicenseStatus }>}
        />
        <Selector
          label="格式"
          isLabelHidden
          placeholder="全部格式"
          hasClear
          value={format}
          onChange={(v) => setFormat(v ?? '')}
          options={[
            { label: 'ONNX', value: 'ONNX' },
            { label: 'RKNN', value: 'RKNN' },
            { label: 'TensorRT', value: 'TensorRT' },
          ]}
        />
      </div>

      <div className="ff-model-grid">
        {filteredModels.map((model) => (
          <Card key={model.id} padding={4}>
            <VStack gap={3} className="ff-model-card">
              <div>
                <Tooltip
                  content={`${model.description}\n标签: ${model.tags.join(', ') || '-'}`}
                  placement="below"
                >
                  <strong className="ff-model-card-title">{model.name}</strong>
                </Tooltip>
                <p className="ff-model-card-desc">{model.description}</p>
              </div>
              <div className="ff-model-card-badges">
                <LicenseStatusBadge status={model.licenseStatus} />
                <Badge label={model.modelStatus} variant="neutral" />
                {model.formats.map((f) => (
                  <Badge key={f} label={f} variant="teal" />
                ))}
              </div>
              <div className="ff-model-card-meta">
                <div className="ff-model-card-meta-row">
                  <span className="ff-model-card-meta-label">发布</span>
                  <span className="ff-model-card-meta-value">
                    {new Date(model.publishedAt).toLocaleString()}
                  </span>
                </div>
                {model.localUpdatedAt ? (
                  <div className="ff-model-card-meta-row">
                    <span className="ff-model-card-meta-label">校验</span>
                    <span className="ff-model-card-meta-value">
                      {new Date(model.localUpdatedAt).toLocaleString()}
                    </span>
                  </div>
                ) : null}
              </div>
              <div className="ff-model-card-versions">
                <span>版本 {model.version}</span>
                <span>{model.localVersion ? '可用' : '未就绪'}</span>
              </div>
              <div className="ff-model-card-actions">
                <Button
                  label="详情"
                  size="sm"
                  variant="secondary"
                  onClick={() => setDetail(model)}
                />
                {!model.localVersion ? (
                  <Button
                    label="校验并启用"
                    size="sm"
                    variant="primary"
                    isLoading={loadingId === model.id}
                    clickAction={async () => {
                      try {
                        await downloadMutation.mutateAsync(model.id)
                      } catch {
                        /* onError toast */
                      }
                    }}
                  />
                ) : null}
              </div>
            </VStack>
          </Card>
        ))}
      </div>
      {filteredModels.length === 0 ? <div className="ff-placeholder">无匹配模型</div> : null}

      <VStack gap={2}>
        <HStack justify="between" vAlign="center">
          <h2 className="ff-page-title" style={{ fontSize: '1.1rem' }}>
            最近更新记录
          </h2>
          <Selector
            label="更新状态"
            isLabelHidden
            placeholder="全部状态"
            hasClear
            value={updateStatus}
            onChange={(v) => setUpdateStatus(v ?? '')}
            options={[
              { label: '成功', value: 'success' },
              { label: '失败', value: 'failed' },
              { label: '进行中', value: 'running' },
            ] satisfies Array<{ label: string; value: ModelUpdateRecord['status'] }>}
          />
        </HStack>
        <div className="ff-table-wrap">
          <DataTable
            data={filteredUpdates}
            idKey="id"
            groupBy="status"
            groupLabel={(key) => UPDATE_STATUS_LABEL[key as ModelUpdateRecord['status']] ?? key}
            emptyLabel="暂无更新记录"
            columns={
              [
                {
                  key: 'id',
                  header: 'ID',
                  width: pixel(80),
                },
                {
                  key: 'modelName',
                  header: '模型名称',
                  width: proportional(1.2),
                },
                {
                  key: 'currentVersion',
                  header: '当前版本',
                  width: pixel(100),
                },
                {
                  key: 'targetVersion',
                  header: '目标版本',
                  width: pixel(100),
                },
                {
                  key: 'operatedAt',
                  header: '操作时间',
                  width: pixel(160),
                  render: (row) => new Date(row.operatedAt).toLocaleString(),
                },
                {
                  key: 'status',
                  header: '状态',
                  width: pixel(96),
                  render: (row) => (
                    <Badge
                      label={UPDATE_STATUS_LABEL[row.status]}
                      variant={
                        row.status === 'success'
                          ? 'success'
                          : row.status === 'failed'
                            ? 'error'
                            : 'warning'
                      }
                    />
                  ),
                },
                {
                  key: 'message',
                  header: '信息',
                  width: proportional(1.4),
                },
              ] satisfies DataTableColumn<ModelUpdateRecord>[]
            }
          />
        </div>
      </VStack>

      <Dialog isOpen={errorOpen} onOpenChange={setErrorOpen} purpose="info" width={420} padding={5}>
        <Layout
          header={<DialogHeader title="模型下载失败" onOpenChange={setErrorOpen} />}
          content={<p className="ff-page-desc">{errorMsg}</p>}
          footer={
            <HStack justify="end">
              <Button label="知道了" variant="primary" onClick={() => setErrorOpen(false)} />
            </HStack>
          }
        />
      </Dialog>

      <Dialog
        isOpen={registerOpen}
        onOpenChange={setRegisterOpen}
        purpose="form"
        width={480}
        padding={6}
      >
        <Layout
          header={<DialogHeader title="接入模型" onOpenChange={setRegisterOpen} />}
          content={
            <div className="ff-dialog-body">
            <VStack gap={3}>
              <p className="ff-page-desc" style={{ margin: 0 }}>
                上传推理权重，并可附带字典 / 类别表等配套文件。检测、OCR、人脸等模型均可接入。
              </p>

              <div className="ff-field-required">
                <FileInput
                  label="模型权重"
                  accept=".onnx,.rknn,.engine,.plan,.trt,.pt,.bin"
                  maxSize={MAX_MODEL_BYTES}
                  value={registerFile}
                  onChange={(f) => {
                    const picked = Array.isArray(f) ? f[0] ?? null : f
                    setRegisterFile(picked)
                    if (!picked) return
                    const fmt = pickFormat(picked.name, formatPick)
                    setFormatPick(fmt)
                    const base = picked.name.replace(/\.[^.]+$/, '')
                    const lower = picked.name.toLowerCase()
                    setRegisterForm((prev) => ({
                      ...prev,
                      formats: [fmt],
                      fileName: picked.name,
                      fileSizeBytes: picked.size,
                      name: prev.name.trim() || base,
                      description:
                        prev.description.trim() || `本地上传 · ${picked.name}`,
                    }))
                    if (!tagText.trim()) {
                      if (lower.includes('yolo') || lower.includes('detect')) {
                        setTagText('detection')
                      } else if (lower.includes('ocr') || lower.includes('ppocr')) {
                        setTagText('ocr')
                      } else if (lower.includes('face')) {
                        setTagText('face')
                      }
                    }
                  }}
                />
              </div>
              {registerFile ? (
                <Text type="supporting" color="secondary">
                  {registerFile.name} · {formatBytes(registerFile.size)}
                </Text>
              ) : (
                <Text type="supporting" color="secondary">
                  必选 · ONNX / RKNN / TensorRT 等，单文件 ≤ 512MB
                </Text>
              )}

              <FileInput
                label="配套文件"
                accept=".names,.txt"
                value={labelsFile}
                onChange={(f) => {
                  const picked = Array.isArray(f) ? f[0] ?? null : f
                  setLabelsFile(picked)
                }}
              />
              {labelsFile ? (
                <Text type="supporting" color="secondary">
                  {labelsFile.name} · {formatBytes(labelsFile.size)}
                </Text>
              ) : (
                <Text type="supporting" color="secondary">
                  可选 · 类别表（.names）或字符字典（.txt），需与权重训练类别一致
                </Text>
              )}
              {companionHint ? (
                <Text type="supporting" color="secondary" style={{ color: '#b45309' }}>
                  {companionHint}
                </Text>
              ) : null}

              <TextInput
                label="名称"
                value={registerForm.name}
                onChange={(v) => setRegisterForm((f) => ({ ...f, name: v }))}
                placeholder="选文件后自动填充"
              />

              {registerMutation.isPending ? (
                <div className="ff-upload-bar">
                  <div className="ff-upload-bar-track">
                    <span style={{ width: `${Math.max(uploadPct, 2)}%` }} />
                  </div>
                  <Text type="supporting" color="secondary">
                    {uploadPct >= 100 ? '登记中…' : `上传中 ${uploadPct}%`}
                  </Text>
                </div>
              ) : null}

              <details className="ff-form-more">
                <summary>更多选项（描述、版本、检索标签）</summary>
                <div className="ff-form-more-body">
                  <VStack gap={3}>
                    <TextInput
                      label="描述"
                      value={registerForm.description}
                      onChange={(v) =>
                        setRegisterForm((f) => ({ ...f, description: v }))
                      }
                    />
                    <TextInput
                      label="版本"
                      value={registerForm.version}
                      onChange={(v) =>
                        setRegisterForm((f) => ({ ...f, version: v }))
                      }
                    />
                    <Selector
                      label="模型格式"
                      value={formatPick}
                      onChange={(v) => {
                        const fmt = (v as ModelFormat) || 'ONNX'
                        setFormatPick(fmt)
                        setRegisterForm((f) => ({ ...f, formats: [fmt] }))
                      }}
                      options={[
                        { label: 'ONNX', value: 'ONNX' },
                        { label: 'RKNN', value: 'RKNN' },
                        { label: 'TensorRT', value: 'TensorRT' },
                      ]}
                    />
                    <TextInput
                      label="检索标签"
                      value={tagText}
                      onChange={setTagText}
                      placeholder="detection, ocr, face"
                      description="逗号分隔，便于在仓库中筛选"
                    />
                  </VStack>
                </div>
              </details>
            </VStack>
            </div>
          }
          footer={
            <div className="ff-dialog-footer">
            <HStack justify="end" gap={2}>
              <Button
                label="取消"
                variant="secondary"
                isDisabled={registerMutation.isPending}
                onClick={() => setRegisterOpen(false)}
              />
              <Button
                label={registerMutation.isPending ? '接入中…' : '接入'}
                variant="primary"
                isLoading={registerMutation.isPending}
                isDisabled={!registerFile}
                clickAction={async () => {
                  if (companionHint) {
                    const ok = window.confirm(
                      `${companionHint}\n\n仍要继续接入吗？`,
                    )
                    if (!ok) return
                  }
                  try {
                    await registerMutation.mutateAsync()
                  } catch {
                    /* onError toast */
                  }
                }}
              />
            </HStack>
            </div>
          }
        />
      </Dialog>

      <SidePanel open={Boolean(detail)} title={detail?.name ?? '模型详情'} onClose={() => setDetail(null)}>
        {detail ? (
          <VStack gap={3}>
            <p>{detail.description}</p>
            <HStack gap={1}>
              {detail.tags.map((t) => (
                <Badge key={t} label={t} variant="purple" />
              ))}
            </HStack>
            <LicenseStatusBadge status={detail.licenseStatus} />
            <div>
              <p className="ff-page-desc">版本与文件</p>
              <ul className="ff-page-desc">
                <li>版本：{detail.version}</li>
                <li>接入时间：{new Date(detail.publishedAt).toLocaleString()}</li>
                <li>状态：{detail.localVersion ? '可用' : '未就绪'}</li>
                <li>
                  最近校验：
                  {detail.localUpdatedAt
                    ? new Date(detail.localUpdatedAt).toLocaleString()
                    : '-'}
                </li>
                <li>文件名：{detail.fileName || '-'}</li>
                <li>
                  大小：
                  {detail.fileSizeBytes
                    ? `${(detail.fileSizeBytes / 1024 / 1024).toFixed(2)} MB`
                    : '-'}
                </li>
                <li>权重路径：{detail.localPath || '-'}</li>
                <li>标签路径：{detail.labelsPath || '-'}</li>
              </ul>
            </div>
            <HStack gap={2} wrap="wrap">
              <Button
                label="检查文件"
                size="sm"
                variant="secondary"
                isLoading={healthMutation.isPending}
                clickAction={async () => {
                  try {
                    await healthMutation.mutateAsync(detail.id)
                  } catch {
                    /* onError toast */
                  }
                }}
              />
              <Button
                label={detail.localVersion ? '重新校验' : '校验并启用'}
                size="sm"
                variant="secondary"
                isLoading={loadingId === detail.id}
                clickAction={async () => {
                  try {
                    const next = await downloadMutation.mutateAsync(detail.id)
                    setDetail(next)
                  } catch {
                    /* onError toast */
                  }
                }}
              />
              <Button
                label="删除模型"
                size="sm"
                variant="destructive"
                isLoading={deleteMutation.isPending}
                clickAction={async () => {
                  if (
                    !window.confirm(
                      `确认删除模型「${detail.name}」？将同时删除仓库记录与本地权重文件。`,
                    )
                  ) {
                    return
                  }
                  try {
                    await deleteMutation.mutateAsync(detail.id)
                  } catch {
                    /* onError toast：如仍有算法引用会返回 409 */
                  }
                }}
              />
            </HStack>
          </VStack>
        ) : null}
      </SidePanel>
    </PageLayout>
  )
}
