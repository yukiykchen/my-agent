import { useState } from 'react'
import type { ToolCallLog, ScreenshotResult, TextCompareResult, FetchPageResult } from '../types'

interface Props {
  calls: ToolCallLog[]
}

// 尝试解析工具结果为 JSON
function tryParseJSON<T>(str: string): T | null {
  try {
    return JSON.parse(str)
  } catch {
    return null
  }
}

// 判断是否为截图工具
function isScreenshotTool(toolName: string): boolean {
  return toolName.includes('take_screenshot')
}

// 判断是否为文本比对工具
function isTextCompareTool(toolName: string): boolean {
  return toolName.includes('compare_texts')
}

// 判断是否为页面抓取工具
function isFetchPageTool(toolName: string): boolean {
  return toolName.includes('fetch_page')
}

// 截图结果渲染
function ScreenshotPreview({ result }: { result: ScreenshotResult }) {
  const [enlarged, setEnlarged] = useState(false)

  return (
    <div className="screenshot-preview">
      <div className="screenshot-meta">
        <span className="meta-label">📸 页面截图</span>
        <span className="meta-title">{result.pageTitle || '未知页面'}</span>
        <a className="meta-url" href={result.pageUrl} target="_blank" rel="noopener noreferrer">
          {result.pageUrl}
        </a>
        <span className="meta-time">🕐 {new Date(result.timestamp).toLocaleString()}</span>
      </div>
      <div className="screenshot-image-wrapper">
        <img
          src={result.screenshotUrl}
          alt={`截图: ${result.pageTitle}`}
          className={`screenshot-image ${enlarged ? 'enlarged' : ''}`}
          onClick={() => setEnlarged(!enlarged)}
        />
        <span className="screenshot-hint">点击{enlarged ? '缩小' : '放大'}</span>
      </div>
    </div>
  )
}

// 相似度进度条
function SimilarityBar({ label, value, color }: { label: string; value: number; color: string }) {
  const percent = Math.round(value * 100)
  return (
    <div className="similarity-bar-row">
      <span className="similarity-label">{label}</span>
      <div className="similarity-bar-track">
        <div
          className="similarity-bar-fill"
          style={{ width: `${percent}%`, backgroundColor: color }}
        />
      </div>
      <span className="similarity-value">{percent}%</span>
    </div>
  )
}

// 文本比对结果渲染
function TextComparePreview({ result }: { result: TextCompareResult }) {
  const overallPercent = Math.round(result.overallScore * 100)

  const getVerdictColor = (verdict: string) => {
    switch (verdict) {
      case '高度相似': return '#ef4444'
      case '中度相似': return '#f59e0b'
      case '低度相似': return '#3b82f6'
      default: return '#22c55e'
    }
  }

  const verdictColor = getVerdictColor(result.verdict)

  return (
    <div className="textcompare-preview">
      <div className="textcompare-header">
        <span className="compare-icon">📊</span>
        <span className="compare-title">文本相似度分析</span>
      </div>

      <div className="textcompare-score">
        <div className="score-circle" style={{ borderColor: verdictColor }}>
          <span className="score-number" style={{ color: verdictColor }}>{overallPercent}</span>
          <span className="score-unit">%</span>
        </div>
        <div className="score-info">
          <span className="verdict" style={{ color: verdictColor }}>{result.verdict}</span>
          <span className="score-desc">
            文本1: {result.text1Length}字 | 文本2: {result.text2Length}字 | 共同词汇: {result.commonWords}
          </span>
        </div>
      </div>

      <div className="similarity-bars">
        <SimilarityBar label="余弦相似度" value={result.cosineSimilarity} color="#8b5cf6" />
        <SimilarityBar label="Jaccard 系数" value={result.jaccardIndex} color="#3b82f6" />
        <SimilarityBar label="LCS 比率" value={result.lcsRatio} color="#06b6d4" />
      </div>

      <div className="compare-details">{result.details}</div>
    </div>
  )
}

// 页面抓取结果渲染
function FetchPagePreview({ result }: { result: FetchPageResult }) {
  const [expanded, setExpanded] = useState(false)
  const contentPreview = result.content.length > 300 && !expanded
    ? result.content.slice(0, 300) + '...'
    : result.content

  return (
    <div className="fetchpage-preview">
      <div className="fetchpage-meta">
        <span className="meta-label">📄 页面内容抓取</span>
        <span className="meta-title">{result.title || '未知标题'}</span>
        <a className="meta-url" href={result.url} target="_blank" rel="noopener noreferrer">
          {result.url}
        </a>
        {result.author && <span className="meta-author">✍️ {result.author}</span>}
        {result.publishDate && <span className="meta-time">🕐 {result.publishDate}</span>}
      </div>
      <div className="fetchpage-content">
        <pre>{contentPreview}</pre>
        {result.content.length > 300 && (
          <span className="expand-toggle" onClick={() => setExpanded(!expanded)}>
            {expanded ? '收起内容 ▴' : '展开全部 ▾'}
          </span>
        )}
      </div>
    </div>
  )
}

export default function ToolCallBlock({ calls }: Props) {
  const [collapsed, setCollapsed] = useState(false)

  return (
    <div className={`tool-calls-block ${collapsed ? 'collapsed' : ''}`}>
      <div className="tool-calls-header" onClick={() => setCollapsed(!collapsed)}>
        <span className="tool-icon">🔧</span>
        工具调用 ({calls.length})
        <span className="collapse-arrow">{collapsed ? '▸' : '▾'}</span>
      </div>

      {!collapsed && (
        <div className="tool-calls-content">
          {calls.map((call, i) => (
            <ToolCallItem key={i} call={call} />
          ))}
        </div>
      )}
    </div>
  )
}

function ToolCallItem({ call }: { call: ToolCallLog }) {
  const [open, setOpen] = useState(false)

  // 尝试解析特殊工具的结果
  const screenshotResult = isScreenshotTool(call.tool) && call.success
    ? tryParseJSON<ScreenshotResult>(call.result)
    : null
  const textCompareResult = isTextCompareTool(call.tool) && call.success
    ? tryParseJSON<TextCompareResult>(call.result)
    : null
  const fetchPageResult = isFetchPageTool(call.tool) && call.success
    ? tryParseJSON<FetchPageResult>(call.result)
    : null

  const hasRichPreview = screenshotResult || textCompareResult || fetchPageResult

  return (
    <div className={`tool-call-item ${call.success ? 'success' : 'error'}`}>
      <div className="tool-call-name">
        <span className={`status-dot ${call.success ? 'success' : 'error'}`} />
        <strong>{call.tool}</strong>
        <span className="tool-server">{call.server}</span>
        <span className="tool-duration">{call.duration}ms</span>
      </div>

      {/* 富预览区域 */}
      {hasRichPreview && (
        <div className="tool-rich-preview">
          {screenshotResult && <ScreenshotPreview result={screenshotResult} />}
          {textCompareResult && <TextComparePreview result={textCompareResult} />}
          {fetchPageResult && <FetchPagePreview result={fetchPageResult} />}
        </div>
      )}

      <div className="tool-call-toggle" onClick={() => setOpen(!open)}>
        {open ? '收起详情 ▴' : '查看详情 ▾'}
      </div>

      {open && (
        <div className="tool-call-details">
          <div className="detail-label">参数:</div>
          <pre>{JSON.stringify(call.args, null, 2)}</pre>
          <div className="detail-label">结果:</div>
          <pre>{call.result.length > 2000 ? call.result.slice(0, 2000) + '...' : call.result}</pre>
        </div>
      )}
    </div>
  )
}
