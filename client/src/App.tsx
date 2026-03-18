import { useState, useEffect } from 'react'
import ChatPanel from './components/ChatPanel'
import './App.css'
import { api } from './services/api'

interface SessionInfo {
  sessionId: string
  provider: string
  toolCount: number
  template: string
}

export default function App() {
  const [session, setSession] = useState<SessionInfo | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    // 自动创建侵权专家 session
    api.createSession('infringement-analyst')
      .then((data) => {
        setSession({
          sessionId: data.sessionId,
          provider: data.provider,
          toolCount: data.toolCount,
          template: 'infringement-analyst',
        })
      })
      .catch((err) => setError(err.message))
  }, [])

  const handleReset = () => {
    setSession(null)
    setError(null)
    api.createSession('infringement-analyst')
      .then((data) => {
        setSession({
          sessionId: data.sessionId,
          provider: data.provider,
          toolCount: data.toolCount,
          template: 'infringement-analyst',
        })
      })
      .catch((err) => setError(err.message))
  }

  if (error) {
    return (
      <div className="app">
        {/* 高级动态背景元素 */}
        <div className="bg-grid"></div>
        <div className="bg-blob blob-1"></div>
        <div className="bg-blob blob-2"></div>
        <div className="bg-blob blob-3"></div>
        
        <div className="setup-panel">
          <div className="setup-content">
            <p className="error-text">连接失败: {error}</p>
            <button className="start-btn" onClick={handleReset}>重试</button>
          </div>
        </div>
      </div>
    )
  }

  if (!session) {
    return (
      <div className="app">
        <div className="bg-grid"></div>
        <div className="bg-blob blob-1"></div>
        <div className="bg-blob blob-2"></div>
        <div className="bg-blob blob-3"></div>

        <div className="setup-panel">
          <div className="setup-content loading-card">
            <div className="spinner"></div>
            <p className="loading-text">正在初始化侵权分析专家引擎...</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="app">
      <div className="bg-grid"></div>
      <div className="bg-blob blob-1"></div>
      <div className="bg-blob blob-2"></div>
      <div className="bg-blob blob-3"></div>
      
      <ChatPanel session={session} onBack={handleReset} />
    </div>
  )
}
