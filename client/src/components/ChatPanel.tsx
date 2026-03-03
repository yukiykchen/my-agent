import { useState, useRef, useEffect, useCallback } from 'react'
import { api } from '../services/api'
import { useWebSocket } from '../hooks/useWebSocket'
import type { ChatMessage, ToolCallLog } from '../types'
import ToolCallBlock from './ToolCallBlock'
import MessageBubble from './MessageBubble'
import ThinkingIndicator from './ThinkingIndicator'

interface Props {
  session: {
    sessionId: string
    provider: string
    toolCount: number
    template: string
  }
  onBack: () => void
}

export default function ChatPanel({ session, onBack }: Props) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const chatEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const { thinkingStep, toolEvents, status, clearToolEvents } = useWebSocket(session.sessionId)

  const scrollToBottom = useCallback(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [messages, thinkingStep, toolEvents, scrollToBottom])

  const sendMessage = async () => {
    const text = input.trim()
    if (!text || isLoading) return

    const userMsg: ChatMessage = {
      id: `msg_${Date.now()}`,
      role: 'user',
      content: text,
      timestamp: Date.now(),
    }

    setMessages((prev) => [...prev, userMsg])
    setInput('')
    setIsLoading(true)
    clearToolEvents()

    try {
      const data = await api.sendMessage(session.sessionId, text)

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
    clearToolEvents()
  }

  const handleBack = async () => {
    await api.deleteSession(session.sessionId).catch(() => {})
    onBack()
  }

  return (
    <div className="chat-panel">
      <header className="chat-header">
        <div className="header-left">
          <h1>⚖️ 侵权分析</h1>
          <span className="badge provider-badge">{session.provider}</span>
          <span className="badge tool-badge">{session.toolCount} tools</span>
        </div>
        <div className="header-right">
          <button className="icon-btn" onClick={handleReset} title="重置对话">🔄</button>
          <button className="icon-btn" onClick={handleBack} title="新对话">🆕</button>
        </div>
      </header>

      <div className="chat-container">
        {messages.length === 0 && (
          <div className="welcome">
            <div className="welcome-icon">⚖️</div>
            <h2>网络侵权证据智能分析系统</h2>
            <p>请输入需要分析的侵权线索，例如：</p>
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

      <div className="input-container">
        <textarea
          ref={inputRef}
          className="chat-input"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入侵权分析请求... (Enter 发送, Shift+Enter 换行)"
          rows={1}
          disabled={isLoading}
        />
        <button
          className="send-btn"
          onClick={sendMessage}
          disabled={!input.trim() || isLoading}
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M22 2L11 13M22 2L15 22L11 13M11 13L2 9L22 2" />
          </svg>
        </button>
      </div>
    </div>
  )
}
