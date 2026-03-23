import { useState } from 'react'
import type { ChatMessage } from '../types'

interface Props {
  message: ChatMessage
}

function formatContent(content: string): string {
  let html = content
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

  // Code blocks
  html = html.replace(/```(\w*)\n?([\s\S]*?)```/g, (_m, lang, code) => {
    return `<pre><code class="lang-${lang}">${code.trim()}</code></pre>`
  })

  // Images (Markdown syntax: ![alt](url))
  html = html.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (_m, alt, url) => {
    return `<div class="message-image-wrapper"><img src="${url}" alt="${alt}" class="message-image" data-clickable="true" /><span class="image-caption">${alt || ''}</span></div>`
  })

  // Inline code
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')

  // Bold
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')

  // Newlines
  html = html.replace(/\n/g, '<br>')

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
