import { useState, useRef, useEffect, useCallback } from 'react'
import { api } from '../services/api'
import { useWebSocket } from '../hooks/useWebSocket'
import type { ChatMessage, ToolCallLog, Attachment } from '../types'
import ToolCallBlock from './ToolCallBlock'
import MessageBubble from './MessageBubble'
import ThinkingIndicator from './ThinkingIndicator'
import RobotIcon from './RobotIcon'

interface Props {
  session: {
    sessionId: string
    provider: string
    toolCount: number
    template: string
  }
  onBack: () => void
}

/** 支持的图片 MIME */
const IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/bmp']
/** 支持的文档 MIME */
const DOC_TYPES = ['text/plain', 'text/markdown', 'text/csv', 'application/pdf',
  'application/msword',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document']
const ALL_ACCEPT = [...IMAGE_TYPES, ...DOC_TYPES].join(',')
const MAX_FILE_SIZE = 20 * 1024 * 1024 // 20MB

export default function ChatPanel({ session, onBack }: Props) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [uploading, setUploading] = useState(false)
  const chatEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { thinkingStep, toolEvents, status, clearToolEvents } = useWebSocket(session.sessionId)

  const scrollToBottom = useCallback(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [messages, thinkingStep, toolEvents, scrollToBottom])

  // ============ 文件上传逻辑 ============

  const handleFileUpload = useCallback(async (files: FileList | File[]) => {
    const fileArray = Array.from(files)
    for (const file of fileArray) {
      // 校验大小
      if (file.size > MAX_FILE_SIZE) {
        alert(`文件 "${file.name}" 超过 20MB 限制`)
        continue
      }
      // 校验类型
      const isImage = IMAGE_TYPES.includes(file.type)
      const isDoc = DOC_TYPES.includes(file.type)
      if (!isImage && !isDoc) {
        alert(`不支持的文件类型: ${file.type || '未知'}`)
        continue
      }

      setUploading(true)
      try {
        const result = await api.uploadFile(file)
        if (result.success) {
          const att = result.attachment
          // 为图片生成本地预览
          if (isImage) {
            att.previewUrl = URL.createObjectURL(file)
          }
          setAttachments(prev => [...prev, att])
        }
      } catch (err: any) {
        alert(`上传失败: ${err.message}`)
      } finally {
        setUploading(false)
      }
    }
  }, [])

  // 粘贴图片
  const handlePaste = useCallback((e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items
    if (!items) return

    const files: File[] = []
    for (let i = 0; i < items.length; i++) {
      const item = items[i]
      if (item.kind === 'file') {
        const file = item.getAsFile()
        if (file) files.push(file)
      }
    }
    if (files.length > 0) {
      e.preventDefault()
      handleFileUpload(files)
    }
  }, [handleFileUpload])

  // 拖拽上传
  const [isDragging, setIsDragging] = useState(false)

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(true)
  }, [])

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)
  }, [])

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)
    if (e.dataTransfer.files.length > 0) {
      handleFileUpload(e.dataTransfer.files)
    }
  }, [handleFileUpload])

  const removeAttachment = (id: string) => {
    setAttachments(prev => {
      const att = prev.find(a => a.id === id)
      if (att?.previewUrl) URL.revokeObjectURL(att.previewUrl)
      return prev.filter(a => a.id !== id)
    })
  }

  // ============ 发送消息 ============

  const sendMessage = async () => {
    const text = input.trim()
    if ((!text && attachments.length === 0) || isLoading) return

    const userMsg: ChatMessage = {
      id: `msg_${Date.now()}`,
      role: 'user',
      content: text,
      attachments: attachments.length > 0 ? [...attachments] : undefined,
      timestamp: Date.now(),
    }

    setMessages((prev) => [...prev, userMsg])
    const currentAttachments = [...attachments]
    setInput('')
    setAttachments([])
    setIsLoading(true)
    clearToolEvents()

    try {
      const data = await api.sendMessage(
        session.sessionId,
        text,
        currentAttachments.length > 0 ? currentAttachments : undefined,
      )

      const assistantMsg: ChatMessage = {
        id: `msg_${Date.now()}_ai`,
        role: 'assistant',
        content: data.response,
        toolCalls: data.toolCalls,
        timestamp: Date.now(),
      }
      setMessages((prev) => [...prev, assistantMsg])
    } catch (err: any) {
      const errorMsg: ChatMessage = {
        id: `msg_${Date.now()}_err`,
        role: 'assistant',
        content: `错误: ${err.message}`,
        timestamp: Date.now(),
      }
      setMessages((prev) => [...prev, errorMsg])
    } finally {
      setIsLoading(false)
      inputRef.current?.focus()
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage()
    }
  }

  const handleReset = async () => {
    await api.resetSession(session.sessionId)
    setMessages([])
    setAttachments([])
    clearToolEvents()
  }

  const handleBack = async () => {
    await api.deleteSession(session.sessionId).catch(() => {})
    onBack()
  }

  // ============ 文件大小格式化 ============

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  return (
    <div
      className="chat-panel"
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {/* 拖拽覆盖层 */}
      {isDragging && (
        <div className="drag-overlay">
          <div className="drag-overlay-content">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="17 8 12 3 7 8" />
              <line x1="12" y1="3" x2="12" y2="15" />
            </svg>
            <p>释放以上传文件</p>
          </div>
        </div>
      )}

      {/* 高级感背光 */}
      <div style={{
        position: 'absolute', top: '10%', left: '50%', transform: 'translate(-50%, -50%)',
        width: '600px', height: '600px', background: 'radial-gradient(circle, rgba(139, 92, 246, 0.08) 0%, transparent 70%)',
        borderRadius: '50%', pointerEvents: 'none', zIndex: 0
      }} />

      <header className="chat-header">
        <div className="header-left">
          <span className="badge tool-badge">{session.toolCount} tools</span>
        </div>
        <div className="header-right">
          <button className="icon-btn" onClick={handleReset} title="重置对话">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/></svg>
          </button>
          <button className="icon-btn" onClick={handleBack} title="新对话">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 5v14"/><path d="M5 12h14"/></svg>
          </button>
        </div>
      </header>

      <div className="chat-container">
        {messages.length === 0 && (
          <div className="welcome">
            <div className="welcome-icon">
              <RobotIcon />
            </div>
            <h2>智能侵权证据分析系统</h2>
            <p>基于大模型和 MCP 架构的自动化取证agent</p>
            <div className="welcome-examples">
              <div className="example" onClick={() => setInput('请帮我分析 https://example.com/article 是否侵犯了我的原创文章')}>
                "请帮我分析某网页是否侵犯了我的原创文章"
              </div>
              <div className="example" onClick={() => setInput('请分析以下两段文字的相似度，并判断是否构成侵权')}>
                "请分析两段文字的相似度，判断是否构成侵权"
              </div>
              <div className="example" onClick={() => setInput('请帮我检索关于信息网络传播权侵权的相关法条')}>
                "检索关于信息网络传播权侵权的相关法条"
              </div>
            </div>
          </div>
        )}

        {messages.map((msg) => (
          <div key={msg.id}>
            {msg.role === 'assistant' && msg.toolCalls && msg.toolCalls.length > 0 && (
              <ToolCallBlock calls={msg.toolCalls} />
            )}
            <MessageBubble message={msg} />
          </div>
        ))}

        {isLoading && (
          <ThinkingIndicator
            step={thinkingStep}
            toolEvents={toolEvents}
            status={status}
          />
        )}

        <div ref={chatEndRef} />
      </div>

      {/* 隐藏的文件输入 */}
      <input
        ref={fileInputRef}
        type="file"
        accept={ALL_ACCEPT}
        multiple
        style={{ display: 'none' }}
        onChange={(e) => {
          if (e.target.files) handleFileUpload(e.target.files)
          e.target.value = '' // 允许重复选择同一文件
        }}
      />

      <div className="input-area">
        {/* 附件预览栏 */}
        {attachments.length > 0 && (
          <div className="attachments-preview">
            {attachments.map((att) => (
              <div key={att.id} className="attachment-item">
                {att.previewUrl ? (
                  <img src={att.previewUrl} alt={att.filename} className="attachment-thumb" />
                ) : (
                  <div className="attachment-icon">
                    {att.mimeType === 'application/pdf' ? '📕' : '📄'}
                  </div>
                )}
                <div className="attachment-info">
                  <span className="attachment-name">{att.filename}</span>
                  <span className="attachment-size">
                    {formatSize(att.size)}
                    {att.textContent && ' · ✅ 已解析'}
                  </span>
                </div>
                <button
                  className="attachment-remove"
                  onClick={() => removeAttachment(att.id)}
                  title="移除"
                >✕</button>
              </div>
            ))}
          </div>
        )}

        <div className="input-container">
          {/* 上传按钮 */}
          <button
            className="upload-btn"
            onClick={() => fileInputRef.current?.click()}
            disabled={isLoading || uploading}
            title="上传图片或文档"
          >
            {uploading ? (
              <div className="upload-spinner" />
            ) : (
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48" />
              </svg>
            )}
          </button>

          <textarea
            ref={inputRef}
            className="chat-input"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            onPaste={handlePaste}
            placeholder={attachments.length > 0
              ? "添加描述或直接发送附件... (Enter 发送)"
              : "输入消息，可粘贴图片或拖拽文件... (Enter 发送, Shift+Enter 换行)"
            }
            rows={1}
            disabled={isLoading}
          />
          <button
            className="send-btn"
            onClick={sendMessage}
            disabled={(!input.trim() && attachments.length === 0) || isLoading}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M22 2L11 13M22 2L15 22L11 13M11 13L2 9L22 2" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  )
}
