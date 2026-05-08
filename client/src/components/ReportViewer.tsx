import { useState, useEffect, useCallback } from 'react'
import { api } from '../services/api'

interface Props {
  caseId: string
  onClose: () => void
}

/** 简易 Markdown → HTML（覆盖标题/表格/列表/加粗/代码/分割线等常用元素） */
function renderMarkdown(md: string): string {
  let html = md
    // 代码块
    .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code class="lang-$1">$2</code></pre>')
    // 标题
    .replace(/^######\s+(.+)$/gm, '<h6>$1</h6>')
    .replace(/^#####\s+(.+)$/gm, '<h5>$1</h5>')
    .replace(/^####\s+(.+)$/gm, '<h4>$1</h4>')
    .replace(/^###\s+(.+)$/gm, '<h3>$1</h3>')
    .replace(/^##\s+(.+)$/gm, '<h2>$1</h2>')
    .replace(/^#\s+(.+)$/gm, '<h1>$1</h1>')
    // 分割线
    .replace(/^---+$/gm, '<hr />')
    // 加粗
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    // 行内代码
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    // 引用块
    .replace(/^>\s?(.+)$/gm, '<blockquote>$1</blockquote>')

  // 表格
  const lines = html.split('\n')
  const result: string[] = []
  let inTable = false
  let tableRows: string[] = []

  for (const line of lines) {
    const trimmed = line.trim()
    if (trimmed.startsWith('|') && trimmed.endsWith('|')) {
      // 分隔行跳过
      if (/^\|[\s\-:|]+\|$/.test(trimmed)) {
        continue
      }
      inTable = true
      tableRows.push(trimmed)
    } else {
      if (inTable) {
        result.push(renderTable(tableRows))
        tableRows = []
        inTable = false
      }
      result.push(line)
    }
  }
  if (inTable) {
    result.push(renderTable(tableRows))
  }

  // 段落 & 列表
  let output = result.join('\n')
  // 无序列表
  output = output.replace(/^[\-\*]\s+(.+)$/gm, '<li>$1</li>')
  output = output.replace(/(<li>[\s\S]*?<\/li>)/g, (match) => match)
  // 连续 li 包裹 ul
  output = output.replace(/((?:<li>.*<\/li>\n?)+)/g, '<ul>$1</ul>')
  // 有序列表
  output = output.replace(/^\d+\.\s+(.+)$/gm, '<li>$1</li>')
  // 段落（非标签行包裹 p）
  output = output.replace(/^(?!<[a-z/])(.+)$/gm, '<p>$1</p>')
  // 清理空 p
  output = output.replace(/<p>\s*<\/p>/g, '')
  return output
}

function renderTable(rows: string[]): string {
  if (rows.length === 0) return ''
  const parseCells = (row: string) =>
    row.split('|').filter(c => c.trim() !== '').map(c => c.trim())

  const headerCells = parseCells(rows[0])
  let html = '<table><thead><tr>'
  for (const cell of headerCells) {
    html += `<th>${cell}</th>`
  }
  html += '</tr></thead><tbody>'

  for (let i = 1; i < rows.length; i++) {
    const cells = parseCells(rows[i])
    html += '<tr>'
    for (const cell of cells) {
      html += `<td>${cell}</td>`
    }
    html += '</tr>'
  }
  html += '</tbody></table>'
  return html
}

const formatTime = (s: string) => {
  if (!s) return '—'
  const d = new Date(s)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

export default function ReportViewer({ caseId, onClose }: Props) {
  const [reports, setReports] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [activeReport, setActiveReport] = useState<{ meta: any; markdown: string } | null>(null)
  const [loadingReport, setLoadingReport] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await api.listReports(caseId)
      if (data.success) {
        setReports(data.reports || [])
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

  const openReport = async (reportId: string) => {
    setLoadingReport(true)
    try {
      const data = await api.getReport(caseId, reportId)
      if (data.success) {
        setActiveReport({ meta: data.meta, markdown: data.markdown })
      }
    } catch (err: any) {
      alert('加载报告失败: ' + err.message)
    } finally {
      setLoadingReport(false)
    }
  }

  const shortHash = (h: string) => h ? h.slice(0, 16) + '...' : '—'

  return (
    <div className="report-viewer">
      <div className="panel-header">
        <h3>法律规范化报告</h3>
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
        <span className="count">共 {reports.length} 份报告</span>
      </div>

      {error && <div className="error-msg">{error}</div>}

      {/* 报告列表 */}
      {!activeReport && (
        <>
          {reports.length === 0 && !loading && (
            <div className="empty-state">
              <p>暂无报告</p>
              <p className="sub">请先采集证据并通过 Agent 生成分析报告</p>
            </div>
          )}
          <div className="report-list">
            {reports.map((r: any, idx: number) => (
              <div key={r.reportId} className="report-card" onClick={() => openReport(r.reportId)}>
                <div className="card-top">
                  <span className="ev-index">#{idx + 1}</span>
                  <span className="ev-type type-report">报告</span>
                  <span className="ev-time">{formatTime(r.generatedAt)}</span>
                </div>
                <div className="card-meta">
                  <div className="meta-row">
                    <span className="meta-label">报告ID:</span>
                    <code className="meta-value">{r.reportId}</code>
                  </div>
                  <div className="meta-row">
                    <span className="meta-label">证据数:</span>
                    <span className="meta-value">{r.evidenceCount} 条</span>
                  </div>
                  <div className="meta-row">
                    <span className="meta-label">整体哈希:</span>
                    <code className="meta-value">{shortHash(r.overallHash)}</code>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      {/* 报告详情 */}
      {activeReport && (
        <div className="report-detail">
          <div className="report-detail-header">
            <button className="back-btn" onClick={() => setActiveReport(null)}>
              ← 返回列表
            </button>
            <span className="report-id">{activeReport.meta.reportId}</span>
          </div>
          {loadingReport ? (
            <div className="loading-state">加载中...</div>
          ) : (
            <div
              className="report-content"
              dangerouslySetInnerHTML={{ __html: renderMarkdown(activeReport.markdown) }}
            />
          )}
        </div>
      )}
    </div>
  )
}
