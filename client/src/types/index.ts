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

/** 截图工具返回结果 */
export interface ScreenshotResult {
  screenshotUrl: string
  pageTitle: string
  pageUrl: string
  timestamp: string
  viewport: { width: number; height: number }
}

/** 文本比对工具返回结果 */
export interface TextCompareResult {
  overallScore: number
  cosineSimilarity: number
  jaccardIndex: number
  lcsRatio: number
  verdict: string
  details: string
  text1Length: number
  text2Length: number
  commonWords: number
}

/** 增强版网页抓取结果 */
export interface FetchPageResult {
  title: string
  content: string
  url: string
  author?: string
  publishDate?: string
  metadata?: Record<string, string>
}
