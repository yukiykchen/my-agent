/** API 和 WebSocket 相关类型定义 */

export interface PromptTemplate {
  id: string
  name: string
  description: string
}

export interface Provider {
  id: string
  name: string
  available: boolean
}

export interface ToolCallLog {
  timestamp: number
  server: string
  tool: string
  args: Record<string, any>
  result: string
  duration: number
  success: boolean
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  toolCalls?: ToolCallLog[]
  timestamp: number
}

export interface WSMessage {
  type: 'connected' | 'tool_call' | 'thinking' | 'status' | 'error'
  sessionId?: string
  step?: string
  status?: string
  tool?: string
  args?: Record<string, any>
  result?: string
  success?: boolean
  duration?: number
  message?: string
}
