import { useState, useEffect, useCallback } from 'react'
import { api } from '../services/api'

interface Props {
  caseId: string
  onClose: () => void
}

export default function EvidenceBrowser({ caseId, onClose }: Props) {
  const [evidences, setEvidences] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [preview, setPreview] = useState<any>(null)
  const [verifying, setVerifying] = useState<string | null>(null)
  const [anchoring, setAnchoring] = useState(false)
  const [verifyResult, setVerifyResult] = useState<Record<string, any>>({})

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await api.listNotarized(caseId)
      if (data.success) {
        setEvidences(data.evidences || [])
      } else {
        setError('查询失败')
      }
    } catch (err: any) {
      setError(err.message || '请求失败')
    } finally {
      setLoading(false)
    }
  }, [caseId])

  useEffect(() => {
    load()
  }, [load])

  const handleVerify = async (evidenceId: string) => {
    setVerifying(evidenceId)
    try {
      const data = await api.verifyEvidence(caseId, evidenceId)
      if (data.success) {
        setVerifyResult(prev => ({ ...prev, [evidenceId]: data.verify }))
      }
    } catch (err: any) {
      setVerifyResult(prev => ({ ...prev, [evidenceId]: { consistent: false, reason: err.message } }))
    } finally {
      setVerifying(null)
    }
  }

  const handleAnchor = async () => {
    if (anchoring) return
    setAnchoring(true)
    try {
      await api.anchorEvidence(caseId)
      await load()
    } catch (err: any) {
      alert('上链失败: ' + err.message)
    } finally {
      setAnchoring(false)
    }
  }

  const getFileName = (filePath: string) => filePath?.split('/').pop() || 'evidence-file'
  const isImage = (filename: string) => /\.(png|jpg|jpeg|webp|gif)$/i.test(filename)
  const isVideo = (filename: string) => /\.(webm|mp4|mov|m4v)$/i.test(filename)
  const canPreview = (ev: any) => isImage(ev.filePath || '') || isVideo(ev.filePath || '')
  const formatTime = (s: string) => {
    if (!s) return '—'
    const d = new Date(s)
    return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
  }
  const shortHash = (h: string) => h ? h.slice(0, 16) + '...' : '—'

  return (
    <div className="evidence-browser">
      <div className="panel-header">
        <h3>证据列表</h3>
        <div className="header-actions">
          <button className="refresh-btn" onClick={load} disabled={loading}>
            {loading ? '刷新中...' : '刷新'}
          </button>
          <button className="close-btn" onClick={onClose}>✕</button>
        </div>
      </div>

      <div className="case-bar">
        <span className="label">案件:</span>
        <code>{caseId}</code>
        <span className="count">共 {evidences.length} 条证据</span>
        {evidences.length > 0 && (
          <button className="anchor-btn" onClick={handleAnchor} disabled={anchoring}>
            {anchoring ? '上链中...' : '全部上链'}
          </button>
        )}
      </div>

      {error && <div className="error-msg">{error}</div>}

      {evidences.length === 0 && !loading && (
        <div className="empty-state">
          <p>暂无已固化证据</p>
          <p className="sub">请通过网页取证或直播取证采集证据</p>
        </div>
      )}

      <div className="evidence-list">
        {evidences.map((ev, idx) => (
          <div key={ev.evidenceId} className={`evidence-card ${ev.integrityStatus === 'compromised' ? 'compromised' : ''}`}>
            <div className="card-top">
              <span className="ev-index">#{idx + 1}</span>
              <span className={`ev-type type-${ev.sourceType}`}>{ev.sourceType === 'web' ? '网页' : ev.sourceType === 'live_segment' ? '直播' : '其他'}</span>
              <span className="ev-time">{formatTime(ev.collectedAt)}</span>
              <span className={`ev-status status-${ev.integrityStatus}`}>
                {ev.integrityStatus === 'verified' ? '✅ 已校验' : ev.integrityStatus === 'compromised' ? '⚠️ 异常' : '⏳ 待校验'}
              </span>
            </div>

            <div className="card-meta">
              <div className="meta-row">
                <span className="meta-label">证据ID:</span>
                <code className="meta-value">{ev.evidenceId}</code>
              </div>
              <div className="meta-row">
                <span className="meta-label">文件:</span>
                <span className="meta-value">{getFileName(ev.filePath)} ({ev.fileSize ? (ev.fileSize / 1024).toFixed(1) + ' KB' : '—'})</span>
              </div>
              <div className="meta-row">
                <span className="meta-label">SHA-256:</span>
                <code className="meta-value">{shortHash(ev.hashChain?.clientHash)}</code>
              </div>
              <div className="meta-row">
                <span className="meta-label">TSA:</span>
                <code className="meta-value">{ev.tsaToken?.serialNumber || '—'}</code>
              </div>
              <div className="meta-row">
                <span className="meta-label">链上:</span>
                <code className="meta-value">{ev.blockchainTx ? shortHash(ev.blockchainTx) : '未上链'}</code>
              </div>
            </div>

            {verifyResult[ev.evidenceId] && (
              <div className={`verify-result ${verifyResult[ev.evidenceId].consistent ? 'ok' : 'fail'}`}>
                {verifyResult[ev.evidenceId].consistent ? '✅ 四值哈希链一致' : `❌ ${verifyResult[ev.evidenceId].reason}`}
              </div>
            )}

            <div className="card-actions">
              {canPreview(ev) && (
                <button className="action-btn" onClick={() => setPreview(ev)}>预览</button>
              )}
              <a
                className="action-btn"
                href={api.evidenceFileUrl(caseId, ev.evidenceId)}
                download={getFileName(ev.filePath)}
                target="_blank"
                rel="noreferrer"
              >
                下载
              </a>
              <button className="action-btn" onClick={() => handleVerify(ev.evidenceId)} disabled={verifying === ev.evidenceId}>
                {verifying === ev.evidenceId ? '校验中...' : '校验'}
              </button>
            </div>
          </div>
        ))}
      </div>

      {preview && (
        <div className="preview-overlay" onClick={(e) => { if (e.target === e.currentTarget) setPreview(null) }}>
          <div className="preview-box">
            <button className="preview-close" onClick={() => setPreview(null)}>✕</button>
            {isVideo(preview.filePath || '') ? (
              <video
                src={api.evidenceFileUrl(caseId, preview.evidenceId)}
                className="preview-video"
                controls
                autoPlay
              />
            ) : (
              <img
                src={api.evidenceFileUrl(caseId, preview.evidenceId)}
                alt="证据预览"
                className="preview-img"
              />
            )}
            <div className="preview-meta">
              <code>{preview.evidenceId}</code>
              <span>{getFileName(preview.filePath)}</span>
              <span>{formatTime(preview.collectedAt)}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
