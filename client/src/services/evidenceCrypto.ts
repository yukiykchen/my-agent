// 证据前端即时哈希工具（论文 4.2.1 节 Web Crypto API）
// 在文件/Blob 落盘前立即计算 SHA-256，尽可能压缩篡改窗口

// API base：与 services/api.ts 保持一致，使用相对路径，由 Vite dev server 代理到后端 :3001
// 生产构建时若前后端不同域，可通过 VITE_API_URL 覆盖
const API_BASE = (import.meta.env.VITE_API_URL as string | undefined) || ''

/**
 * 计算 Blob 或 ArrayBuffer 的 SHA-256 hex
 */
export async function sha256Hex(input: Blob | ArrayBuffer | Uint8Array): Promise<string> {
  let buffer: ArrayBuffer
  if (input instanceof Blob) {
    buffer = await input.arrayBuffer()
  } else if (input instanceof Uint8Array) {
    // slice 返回的 ArrayBufferLike 强制断言为 ArrayBuffer（Uint8Array 底层就是 ArrayBuffer）
    const ab = input.buffer.slice(input.byteOffset, input.byteOffset + input.byteLength)
    buffer = ab as ArrayBuffer
  } else {
    buffer = input
  }
  const digest = await crypto.subtle.digest('SHA-256', buffer)
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

export interface FixateOptions {
  caseId: string
  sourceType: 'web' | 'live_segment' | 'short_video' | 'document' | 'misc'
  collector?: string
  evidenceId?: string
  filename?: string
  meta?: Record<string, string>
  /**
   * 进度回调
   * - hashing: 正在计算前端哈希
   * - uploading: 正在上传
   * - timestamping: 服务端正在申请 TSA 时间戳
   * - done: 固化完成
   */
  onProgress?: (stage: string, extra?: Record<string, unknown>) => void
}

export interface NotarizedEvidence {
  evidenceId: string
  caseId: string
  sourceType: string
  filePath: string
  fileSize: number
  collectedAt: string
  fixedAt: string
  hashChain: {
    clientHash: string
    serverHash: string
    tsaHash: string
    blockchainHash?: string
  }
  tsaToken?: {
    serialNumber: string
    genTime: string
    tsaName: string
  }
  integrityStatus: 'verified' | 'compromised' | 'pending'
  fixationLatencyMs: number
}

/**
 * 执行完整的即时固化流水线
 *
 * 时序：
 *   T0 收到 Blob
 *   T1 Web Crypto API 计算 SHA-256（< 100ms）
 *   T2 上传至 /api/notarize/fixate（携带 clientHash）
 *   T3 服务端校验哈希 + 落盘 + 申请 TSA + chmod 444
 *   T4 返回 NotarizedEvidence
 */
export async function fixateEvidence(
  blob: Blob,
  options: FixateOptions
): Promise<NotarizedEvidence> {
  const { caseId, sourceType, collector, evidenceId, filename, meta, onProgress } = options

  // 阶段 1：前端即时哈希（关键：在落盘前完成）
  onProgress?.('hashing')
  const t0 = performance.now()
  const clientHash = await sha256Hex(blob)
  const hashMs = performance.now() - t0
  onProgress?.('hashing_done', { clientHash, ms: hashMs.toFixed(1) })

  // 阶段 2：上传
  onProgress?.('uploading', { clientHash })
  const form = new FormData()
  const fname = filename || `evidence_${Date.now()}.bin`
  form.append('file', blob, fname)
  form.append('caseId', caseId)
  form.append('sourceType', sourceType)
  form.append('clientHash', clientHash)
  if (collector) form.append('collector', collector)
  if (evidenceId) form.append('evidenceId', evidenceId)
  if (meta) form.append('meta', JSON.stringify(meta))

  const resp = await fetch(`${API_BASE}/api/notarize/fixate`, {
    method: 'POST',
    body: form,
    credentials: 'include',
  })

  if (!resp.ok) {
    const text = await resp.text()
    throw new Error(`固化失败: ${resp.status} ${text}`)
  }

  const data = await resp.json()
  if (!data.success) {
    throw new Error(data.error || '固化失败')
  }

  onProgress?.('done', { latencyMs: data.evidence?.fixationLatencyMs })
  return data.evidence as NotarizedEvidence
}

/**
 * 请求服务端复验证据完整性
 */
export async function verifyEvidence(
  caseId: string,
  evidenceId: string
): Promise<{ consistent: boolean; reason: string; evidence: NotarizedEvidence }> {
  const resp = await fetch(`${API_BASE}/api/notarize/${caseId}/${evidenceId}/verify`, {
    method: 'POST',
    credentials: 'include',
  })
  const data = await resp.json()
  return {
    consistent: data.verify?.consistent ?? false,
    reason: data.verify?.reason ?? '',
    evidence: data.evidence,
  }
}

/**
 * 列出一个案件的所有固化证据
 */
export async function listNotarized(caseId: string): Promise<NotarizedEvidence[]> {
  const resp = await fetch(`${API_BASE}/api/notarize/${caseId}`, {
    credentials: 'include',
  })
  const data = await resp.json()
  return (data.evidences as NotarizedEvidence[]) || []
}
