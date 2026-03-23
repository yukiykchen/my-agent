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

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      document.documentElement.style.setProperty('--mouse-x', `${e.clientX}px`)
      document.documentElement.style.setProperty('--mouse-y', `${e.clientY}px`)
    }
    window.addEventListener('mousemove', handleMouseMove)
    return () => window.removeEventListener('mousemove', handleMouseMove)
  }, [])

  // 生成随机粒子
  const particles = Array.from({ length: 20 }).map((_, i) => ({
    id: i,
    top: `${Math.random() * 100}%`,
    left: `${Math.random() * 100}%`,
    delay: `${Math.random() * 5}s`,
    duration: `${10 + Math.random() * 20}s`
  }))

  const BackgroundEffects = () => (
    <>
      <div className="bg-grid"></div>
      <div className="cursor-glow"></div>
      <div className="bg-blob blob-1"></div>
      <div className="bg-blob blob-2"></div>
      <div className="bg-blob blob-3"></div>
      <div className="particles">
        {particles.map((p) => (
          <div
            key={p.id}
            className="particle"
            style={{
              top: p.top,
              left: p.left,
              animationDelay: p.delay,
              animationDuration: p.duration
            }}
          />
        ))}
      </div>
    </>
  )

  if (error) {
    return (
      <div className="app">
        <BackgroundEffects />
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
        <BackgroundEffects />
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
      <BackgroundEffects />
      <ChatPanel session={session} onBack={handleReset} />
    </div>
  )
}
