import { http } from '@/lib/http'
import type { ModelItem, ModelRegisterInput, ModelUpdateRecord } from '@/types/api'

/** 模型仓库走真实 /api/v1 */

export async function listModels(): Promise<{ items: ModelItem[]; total: number }> {
  const { data } = await http.get('/models')
  return data.data
}

/** JSON 登记（仅元数据；文件须已在服务端路径） */
export async function registerModel(input: ModelRegisterInput): Promise<ModelItem> {
  const { data } = await http.post('/models/register', input)
  return data.data
}

export type UploadProgress = (percent: number) => void

/** 上传权重文件并登记到模型仓库 */
export async function uploadModel(
  input: ModelRegisterInput,
  file: File,
  labelsFile?: File | null,
  onProgress?: UploadProgress,
): Promise<ModelItem> {
  const form = new FormData()
  form.append('file', file)
  if (labelsFile) form.append('labels', labelsFile)
  form.append('name', input.name?.trim() || file.name.replace(/\.[^.]+$/, ''))
  form.append('description', input.description ?? '')
  form.append('version', input.version || '1.0.0')
  form.append('formats', (input.formats?.length ? input.formats : ['ONNX']).join(','))
  form.append('tags', (input.tags ?? []).join(','))
  form.append('fileSizeBytes', String(file.size))

  const { data } = await http.post('/models/upload', form, {
    timeout: 10 * 60 * 1000,
    transformRequest: [
      (body, headers) => {
        if (body instanceof FormData && headers) {
          delete headers['Content-Type']
        }
        return body
      },
    ],
    onUploadProgress: (evt) => {
      if (!onProgress || !evt.total) return
      const pct = Math.min(100, Math.round((evt.loaded / evt.total) * 100))
      onProgress(pct)
    },
  })
  return data.data
}

export async function downloadModel(id: string): Promise<ModelItem> {
  const { data } = await http.post(`/models/${id}/download`)
  return data.data
}

export async function removeLocalModel(id: string): Promise<ModelItem> {
  const { data } = await http.delete(`/models/${id}/local`)
  return data.data
}

export async function checkModelHealth(id: string): Promise<{
  ok: boolean
  message: string
  latencyMs: number
}> {
  const { data } = await http.post(`/models/${id}/health`)
  return data.data
}

export async function scaleModel(id: string, scale: number): Promise<ModelItem> {
  const { data } = await http.post(`/models/${id}/scale`, { scale })
  return data.data
}

export async function deleteModel(id: string): Promise<void> {
  await http.delete(`/models/${id}`)
}

export async function listModelUpdates(): Promise<ModelUpdateRecord[]> {
  const { data } = await http.get('/models/updates')
  return data.data
}
