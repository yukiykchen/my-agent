import { useState, useEffect, useCallback } from 'react'
import { api } from '../services/api'

interface Props {
  caseId: string
  onClose: () => void
}

export default function CaseDashboard({ caseId, onClose }: Props) {
  const [evidenceCount, setEvidenceCount] = useState(0)
  const [reportCount, setReportCount] = useState(0)
  const [anchoredCount, setAnchoredCount] = useState(0)
  const [evidences, setEvidences] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [custodyValid, setCustodyValid] = useState<boolean | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      // 并行加载证据列表和报告列表
      const [evData, rptData] = await Promise.all([
        api.listNotarized(caseId),
        api.listReports(caseId),
      ])
      if (evData.success) {
        const evs = evData.evidences || []
        setEvidences(evs)
        setEvidenceCount(evs.length)
        setAnchoredCount(evs.filter((e: any) => e.blockchainTx).length)
      }
      if (rptData.success) {
        setReportCount(rptData.count || 0)
      }

      // 监督链校验
      try {
        const res = await fetch(`/api/custody/${caseId}/verify`)
        const data = await res.json()
        setCustodyValid(data.valid ?? false)
      } catch {
        setCustodyValid(null)
      }
    } catch (err: any) {
      console.error('Dashboard load error:', err)
    } finally {
      setLoading(false)
    }
  }, [caseId])

  useEffect(() => {
    load()
  }, [load])

  // 证据类型分布
  const typeCounts: Record<string, number> = {}
  for (const ev of evidences) {
    const t = ev.sourceType || 'other'
    typeCounts[t] = (typeCounts[t] || 0) + 1
  }

  // 固化延迟统计
  const latencies = evidences.map((e: any) => e.fixationLatencyMs || 0).filter((l: number) => l > 0)
  const avgLatency = latencies.length > 0
    ? Math.round(latencies.reduce((a: number, b: number) => a + b, 0) / latencies.length)
    : 0

  const formatTime = (s: string) => {
    if (!s) return '—'
    const d = new Date(s)
    return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}`
  }

  const typeLabel: Record<string, string> = {
    web: '网页', live_segment: '直播', short_video: '短视频', document: '文档', misc: '其他',
  }

  return (
    <div className="case-dashboard">
      <div className="panel-header">
        <h3>案件概览</h3>
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
      </div>

      {loading ? (
        <div className="loading-state">加载中...</div>
      ) : (
        <>
          {/* 核心指标卡片 */}
          <div className="stats-grid">
            <div className="stat-card stat-evidence">
              <div className="stat-value">{evidenceCount}</div>
              <div className="stat-label">已固化证据</div>
            </div>
            <div className="stat-card stat-anchor">
              <div className="stat-value">{anchoredCount}<span className="stat-total">/{evidenceCount}</span></div>
              <div className="stat-label">已上链</div>
            </div>
            <div className="stat-card stat-report">
              <div className="stat-value">{reportCount}</div>
              <div className="stat-label">分析报告</div>
            </div>
            <div className="stat-card stat-latency">
              <div className="stat-value">{avgLatency}<span className="stat-unit">ms</span></div>
              <div className="stat-label">平均固化延迟</div>
            </div>
          </div>

          {/* 监督链状态 */}
          <div className="custody-status">
            <span className="custody-label">监督链:</span>
            {custodyValid === null ? (
              <span className="custody-unknown">未验证</span>
            ) : custodyValid ? (
              <span className="custody-valid">✅ 完整</span>
            ) : (
              <span className="custody-invalid">❌ 异常</span>
            )}
          </div>

          {/* 证据类型分布 */}
          {Object.keys(typeCounts).length > 0 && (
            <div className="type-distribution">
              <h4>证据类型分布</h4>
              <div className="type-bars">
                {Object.entries(typeCounts).map(([type, count]) => (
                  <div key={type} className="type-bar-row">
                    <span className="type-name">{typeLabel[type] || type}</span>
                    <div className="type-bar-track">
                      <div
                        className="type-bar-fill"
                        style={{ width: `${(count / evidenceCount) * 100}%` }}
                      />
                    </div>
                    <span className="type-count">{count}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* 最近证据时间线 */}
          {evidences.length > 0 && (
            <div className="recent-timeline">
              <h4>最近固化</h4>
              {evidences.slice(-5).reverse().map((ev, idx) => (
                <div key={ev.evidenceId} className="timeline-item">
                  <div className="timeline-dot" />
                  <div className="timeline-content">
                    <span className="timeline-type">{typeLabel[ev.sourceType] || ev.sourceType}</span>
                    <span className="timeline-time">{formatTime(ev.fixedAt || ev.collectedAt)}</span>
                    <span className="timeline-hash">{(ev.hashChain?.clientHash || '').slice(0, 12)}...</span>
                    {ev.blockchainTx && <span className="timeline-chained">🔗</span>}
                  </div>
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  )
}
