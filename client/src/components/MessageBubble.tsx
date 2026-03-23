import { useState } from 'react'
import type { ChatMessage } from '../types'

interface Props {
  message: ChatMessage
}

function formatContent(content: string): string {
  // 先提取代码块，用占位符替换，避免内部被其他规则处理
  const codeBlocks: string[] = []
  let processed = content.replace(/```(\w*)\n?([\s\S]*?)```/g, (_m, lang, code) => {
    const idx = codeBlocks.length
    codeBlocks.push(`<pre><code class="lang-${escapeHtml(lang)}">${escapeHtml(code.trim())}</code></pre>`)
    return `__CODE_BLOCK_${idx}__`
  })

  // 按行处理（表格、标题等是行级语法）
  const lines = processed.split('\n')
  const result: string[] = []
  let i = 0

  while (i < lines.length) {
    const line = lines[i]

    // 检测表格：当前行包含 | 且下一行是分隔行 |---|
    if (line.includes('|') && i + 1 < lines.length && /^\|[\s\-:|]+\|$/.test(lines[i + 1].trim())) {
      const tableLines: string[] = [line]
      i++ // 跳到分隔行
      tableLines.push(lines[i])
      i++
      // 继续读取表格数据行
      while (i < lines.length && lines[i].trim().startsWith('|')) {
        tableLines.push(lines[i])
        i++
      }
      result.push(parseTable(tableLines))
      continue
    }

    // 水平分割线
    if (/^---+$/.test(line.trim()) || /^\*\*\*+$/.test(line.trim())) {
      result.push('<hr class="md-hr">')
      i++
      continue
    }

    // 标题 (h1-h4)
    const headingMatch = line.match(/^(#{1,4})\s+(.+)$/)
    if (headingMatch) {
      const level = headingMatch[1].length
      const text = formatInline(headingMatch[2])
      result.push(`<h${level} class="md-h${level}">${text}</h${level}>`)
      i++
      continue
    }

    // 引用块
    if (line.trimStart().startsWith('> ')) {
      const quoteLines: string[] = []
      while (i < lines.length && lines[i].trimStart().startsWith('> ')) {
        quoteLines.push(lines[i].replace(/^>\s?/, ''))
        i++
      }
      result.push(`<blockquote class="md-blockquote">${formatInline(quoteLines.join('<br>'))}</blockquote>`)
      continue
    }

    // 有序列表
    if (/^\d+\.\s/.test(line.trimStart())) {
      const listItems: string[] = []
      while (i < lines.length && /^\d+\.\s/.test(lines[i].trimStart())) {
        listItems.push(formatInline(lines[i].replace(/^\d+\.\s/, '')))
        i++
      }
      result.push('<ol class="md-ol">' + listItems.map(li => `<li>${li}</li>`).join('') + '</ol>')
      continue
    }

    // 无序列表
    if (/^[-*]\s/.test(line.trimStart())) {
      const listItems: string[] = []
      while (i < lines.length && /^[-*]\s/.test(lines[i].trimStart())) {
        listItems.push(formatInline(lines[i].replace(/^[-*]\s/, '')))
        i++
      }
      result.push('<ul class="md-ul">' + listItems.map(li => `<li>${li}</li>`).join('') + '</ul>')
      continue
    }

    // 空行
    if (line.trim() === '') {
      result.push('<br>')
      i++
      continue
    }

    // 普通行
    result.push(formatInline(escapeHtml(line)))
    i++
  }

  // 还原代码块
  let html = result.join('\n')
  codeBlocks.forEach((block, idx) => {
    html = html.replace(`__CODE_BLOCK_${idx}__`, block)
  })

  return html
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

function formatInline(text: string): string {
  let html = text

  // Images (Markdown syntax: ![alt](url))
  html = html.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (_m, alt, url) => {
    return `<div class="message-image-wrapper"><img src="${url}" alt="${alt}" class="message-image" data-clickable="true" /><span class="image-caption">${alt || ''}</span></div>`
  })

  // Links: [text](url)
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')

  // Inline code
  html = html.replace(/`([^`]+)`/g, '<code class="md-inline-code">$1</code>')

  // Bold + Italic
  html = html.replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')

  // Bold
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')

  // Italic
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>')

  return html
}

function parseTable(lines: string[]): string {
  if (lines.length < 2) return lines.map(l => formatInline(escapeHtml(l))).join('<br>')

  const parseCells = (line: string) =>
    line.split('|').map(c => c.trim()).filter((_, i, arr) => i > 0 && i < arr.length) // 去掉首尾空项

  const headerCells = parseCells(lines[0])
  const bodyRows = lines.slice(2) // 跳过分隔行

  let html = '<div class="md-table-wrapper"><table class="md-table">'

  // 表头
  html += '<thead><tr>'
  headerCells.forEach(cell => {
    html += `<th>${formatInline(escapeHtml(cell))}</th>`
  })
  html += '</tr></thead>'

  // 表体
  html += '<tbody>'
  bodyRows.forEach(row => {
    const cells = parseCells(row)
    html += '<tr>'
    cells.forEach(cell => {
      html += `<td>${formatInline(escapeHtml(cell))}</td>`
    })
    html += '</tr>'
  })
  html += '</tbody></table></div>'

  return html
}

export default function MessageBubble({ message }: Props) {
  const [lightboxUrl, setLightboxUrl] = useState<string | null>(null)

  const handleClick = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement
    if (target.tagName === 'IMG' && target.dataset.clickable === 'true') {
      setLightboxUrl((target as HTMLImageElement).src)
    }
  }

  const imageAttachments = message.attachments?.filter(a => a.mimeType?.startsWith('image/')) || []
  const docAttachments = message.attachments?.filter(a => !a.mimeType?.startsWith('image/')) || []

  return (
    <>
      <div className={`message ${message.role}`} onClick={handleClick}>
        {/* 附件展示区域 */}
        {message.attachments && message.attachments.length > 0 && (
          <div className="msg-attachments">
            {/* 图片网格 */}
            {imageAttachments.length > 0 && (
              <div className="msg-image-grid">
                {imageAttachments.map((att) => (
                  <div key={att.id} className="msg-image-item">
                    <img
                      src={att.previewUrl || att.url}
                      alt={att.filename}
                      className="msg-image-thumb"
                      data-clickable="true"
                      onClick={() => setLightboxUrl(att.previewUrl || att.url)}
                    />
                  </div>
                ))}
              </div>
            )}
            {/* 文档列表 */}
            {docAttachments.length > 0 && (
              <div className="msg-doc-list">
                {docAttachments.map((att) => (
                  <a
                    key={att.id}
                    className="msg-doc-item"
                    href={att.url}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <span className="doc-icon">📄</span>
                    <span className="doc-name">{att.filename}</span>
                  </a>
                ))}
              </div>
            )}
          </div>
        )}

        {/* 文本内容 */}
        {message.content && (
          <div
            className="message-content"
            dangerouslySetInnerHTML={{ __html: formatContent(message.content) }}
          />
        )}
      </div>

      {lightboxUrl && (
        <div className="image-lightbox" onClick={() => setLightboxUrl(null)}>
          <div className="lightbox-content">
            <img src={lightboxUrl} alt="放大查看" />
            <button className="lightbox-close" onClick={() => setLightboxUrl(null)}>✕</button>
          </div>
        </div>
      )}
    </>
  )
}
