/** 后端 API 服务 */

import type { Attachment } from '../types'

const API_BASE = '/api'

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${url}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

export const api = {
  getPrompts: () => request<{ id: string; name: string; description: string }[]>('/prompts'),

  getProviders: () => request<{ id: string; name: string; available: boolean }[]>('/providers'),

  createSession: (promptTemplate: string, provider?: string) =>
    request<{ success: boolean; sessionId: string; provider: string; toolCount: number }>(
      '/session',
      { method: 'POST', body: JSON.stringify({ promptTemplate, provider }) },
    ),

  sendMessage: (sessionId: string, message: string, attachments?: Attachment[]) =>
    request<{ success: boolean; response: string; toolCalls: any[] }>(
      '/chat',
      {
        method: 'POST',
        body: JSON.stringify({
          sessionId,
          message,
          attachments: attachments?.map(a => ({
            id: a.id,
            filename: a.filename,
            mimeType: a.mimeType,
            size: a.size,
            url: a.url,
            dataURI: a.dataURI,
            textContent: a.textContent,
          })),
        }),
      },
    ),

  /** 上传文件 */
  uploadFile: async (file: File): Promise<{ success: boolean; attachment: Attachment }> => {
    const formData = new FormData()
    formData.append('file', file)

    const res = await fetch(`${API_BASE}/upload`, {
      method: 'POST',
      body: formData,
      // 注意：不要设置 Content-Type，浏览器会自动设置 multipart boundary
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(err.error || res.statusText)
    }
    return res.json()
  },

  resetSession: (sessionId: string) =>
    request<{ success: boolean }>('/reset', {
      method: 'POST',
      body: JSON.stringify({ sessionId }),
    }),

  deleteSession: (sessionId: string) =>
    request<{ success: boolean }>(`/session/${sessionId}`, { method: 'DELETE' }),

  getCases: () => request<any[]>('/cases'),

  getCaseDetail: (caseId: string) => request<any>(`/cases/${caseId}`),
}
