import { useState } from 'react'
import { api } from '../services/api'
import { captureLoggedInBrowserPage } from '../services/browserCapture'

interface Props {
  caseId: string
  onClose: () => void
}

export default function WebCapturePanel({ caseId, onClose }: Props) {
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [mode, setMode] = useState<'screenshot' | 'browser' | 'crawl'>('screenshot')
  const [result, setResult] = useState<any>(null)
  const [error, setError] = useState('')

  const handleCapture = async () => {
    if ((mode !== 'browser' && !url.trim()) || loading) return
    setLoading(true)
    setError('')
    setResult(null)
    try {
      if (mode === 'screenshot') {
        const data = await api.webScreenshot(caseId, url.trim())
        if (data.success) {
          setResult({ type: 'screenshot', ...data })
        } else {
          setError(data.message || '截图失败')
        }
      } else if (mode === 'browser') {
        const data = await captureLoggedInBrowserPage({
          caseId,
          pageUrl: url.trim(),
          collector: 'browser_tab_capture',
        })
        setResult({ type: 'browser', success: true, message: '当前浏览器页面截图并固化成功', ...data })
      } else {
        const data = await api.webCrawl(caseId, url.trim())
        if (data.success) {
          setResult({ type: 'crawl', ...data })
        } else {
          setError(data.message || '抓取失败')
        }
      }
    } catch (err: any) {
      setError(err.message || '请求失败')
    } finally {
      setLoading(false)
    }
  }

  const isValidUrl = (s: string) => {
    try {
      new URL(s)
      return true
    } catch {
      return false
    }
  }

  return (
    <div className="web-capture-panel">
      <div className="panel-header">
        <h3>网页取证</h3>
        <button className="close-btn" onClick={onClose}>✕</button>
      </div>

      <div className="panel-body">
        <div className="case-info">
          <span className="label">案件ID:</span>
          <code className="case-id">{caseId}</code>
        </div>

        <div className="mode-switch">
          <button
            className={mode === 'screenshot' ? 'active' : ''}
            onClick={() => { setMode('screenshot'); setResult(null); setError('') }}
          >
            无头截图
          </button>
          <button
            className={mode === 'browser' ? 'active' : ''}
            onClick={() => { setMode('browser'); setResult(null); setError('') }}
          >
            已登录截图
          </button>
          <button
            className={mode === 'crawl' ? 'active' : ''}
            onClick={() => { setMode('crawl'); setResult(null); setError('') }}
          >
            正文抓取
          </button>
        </div>

        <div className="url-input-row">
          <input
            type="text"
            placeholder={mode === 'browser' ? '可选：粘贴当前页面 URL 作为证据元数据' : mode === 'screenshot' ? '请输入目标网页 URL，如 https://item.taobao.com/xxx' : '请输入文章/商品页 URL'}
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCapture()}
          />
          <button
            className="capture-btn"
            onClick={handleCapture}
            disabled={loading || (mode !== 'browser' && !isValidUrl(url.trim()))}
          >
            {loading ? '取证中...' : mode === 'browser' ? '选择已登录标签页' : mode === 'screenshot' ? '截图取证' : '抓取取证'}
          </button>
        </div>

        {mode === 'browser' && (
          <div className="capture-tip">
            用于需要登录态的页面：点击后在浏览器弹窗中选择已经登录的目标标签页，系统会截取当前可见区域并立即固化。
          </div>
        )}

        {error && <div className="error-msg">{error}</div>}

        {result && (
          <div className="result-area">
            <div className="result-header">
              <span className="success-badge">取证成功</span>
              <span className="evidence-id">证据ID: {result.evidence?.evidenceId}</span>
            </div>

            {(result.type === 'screenshot' || result.type === 'browser') && result.screenshot && (
              <div className="screenshot-preview">
                <div className="meta-row">
                  <span>标题: {result.screenshot.pageTitle || '—'}</span>
                  <span>尺寸: {result.screenshot.viewport?.width}×{result.screenshot.viewport?.height}</span>
                </div>
                <img
                  src={result.evidence?.evidenceId ? api.evidenceFileUrl(caseId, result.evidence.evidenceId) : `/api/evidence/${caseId}/${result.screenshot.screenshotUrl?.split('/').pop()}`}
                  alt="网页截图"
                  className="screenshot-img"
                  onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                />
              </div>
            )}

            {result.type === 'crawl' && result.crawl && (
              <div className="crawl-preview">
                <div className="meta-row">
                  <span>标题: {result.crawl.title || '—'}</span>
                  <span>字数: {result.crawl.wordCount || 0}</span>
                </div>
                <div className="crawl-content">
                  {result.crawl.content?.slice(0, 800)}...
                </div>
              </div>
            )}

            <div className="hash-row">
              <span className="hash-label">SHA-256:</span>
              <code className="hash-value">{result.evidence?.hashChain?.clientHash?.slice(0, 24)}...</code>
            </div>
            <div className="hash-row">
              <span className="hash-label">TSA 序列号:</span>
              <code className="hash-value">{result.evidence?.tsaToken?.serialNumber || '—'}</code>
            </div>
            <div className="hash-row">
              <span className="hash-label">固化耗时:</span>
              <span>{result.evidence?.fixationLatencyMs} ms</span>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
