import { useState, useEffect } from 'react'
import { api } from '../services/api'

interface Props {
  onSessionCreated: (info: {
    sessionId: string
    provider: string
    toolCount: number
    template: string
  }) => void
}

interface Template {
  id: string
  name: string
  description: string
}

export default function SetupPanel({ onSessionCreated }: Props) {
  const [templates, setTemplates] = useState<Template[]>([])
  const [selected, setSelected] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.getPrompts().then((data) => {
      setTemplates(data)
      if (data.length > 0) setSelected(data[0].id)
    }).catch((err) => setError(err.message))
  }, [])

  const handleStart = async () => {
    if (!selected) return
    setLoading(true)
    setError(null)
    try {
      const data = await api.createSession(selected)
      onSessionCreated({
        sessionId: data.sessionId,
        provider: data.provider,
        toolCount: data.toolCount,
        template: selected,
      })
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="setup-panel">
      <div className="setup-content">
        <div className="setup-header">
          <h1>⚖️ 网络侵权证据智能分析系统</h1>
          <p className="subtitle">基于 ReAct Agent + Legal Syllogism 的智能侵权分析</p>
        </div>

        <div className="form-group">
          <label>选择分析模式</label>
          <div className="template-grid">
            {templates.map((t) => (
              <div
                key={t.id}
                className={`template-card ${selected === t.id ? 'selected' : ''}`}
                onClick={() => setSelected(t.id)}
              >
                <div className="template-name">{t.name}</div>
                <div className="template-desc">{t.description}</div>
              </div>
            ))}
          </div>
        </div>

        <button
          className="start-btn"
          onClick={handleStart}
          disabled={!selected || loading}
        >
          {loading ? '连接中...' : '开始分析'}
        </button>

        {error && <p className="error-text">{error}</p>}
      </div>
    </div>
  )
}
