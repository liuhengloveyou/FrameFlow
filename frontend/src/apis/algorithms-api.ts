import { http } from '@/lib/http'
import type {
  AlgorithmCapability,
  AlgorithmInferTestResult,
  AlgorithmInput,
  PageResult,
} from '@/types/api'

export type AlgorithmQuery = {
  q?: string
  category?: string
  status?: string
}

/** 算法接入走真实 /api/v1 */

export async function listAlgorithms(
  query: AlgorithmQuery = {},
): Promise<PageResult<AlgorithmCapability>> {
  const { data } = await http.get('/algorithms', { params: query })
  return data.data as PageResult<AlgorithmCapability>
}

export async function getAlgorithm(id: string): Promise<AlgorithmCapability> {
  const page = await listAlgorithms()
  const item = page.items.find((a) => a.id === id)
  if (!item) throw new Error('算法不存在')
  return item
}

export async function createAlgorithm(input: AlgorithmInput): Promise<AlgorithmCapability> {
  const { data } = await http.post('/algorithms', input)
  return data.data as AlgorithmCapability
}

export async function updateAlgorithm(
  id: string,
  input: AlgorithmInput,
): Promise<AlgorithmCapability> {
  const { data } = await http.put(`/algorithms/${id}`, input)
  return data.data as AlgorithmCapability
}

export async function updateAlgorithmStatus(
  id: string,
  status: AlgorithmCapability['status'],
): Promise<AlgorithmCapability> {
  const { data } = await http.patch(`/algorithms/${id}`, { status })
  return data.data as AlgorithmCapability
}

export async function deleteAlgorithm(id: string): Promise<void> {
  await http.delete(`/algorithms/${id}`)
}

export async function runAlgorithmInferTest(
  id: string,
  input: { file: File; conf: number },
): Promise<AlgorithmInferTestResult> {
  const form = new FormData()
  form.append('file', input.file)
  form.append('imageName', input.file.name)
  form.append('conf', String(input.conf))
  const { data } = await http.post(`/algorithms/${id}/infer-test`, form, {
    timeout: 180_000,
    transformRequest: [
      (body, headers) => {
        if (body instanceof FormData && headers) {
          delete headers['Content-Type']
        }
        return body
      },
    ],
  })
  return data.data
}
