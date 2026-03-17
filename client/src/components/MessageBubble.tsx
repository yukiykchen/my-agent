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

  return (
    <>
      <div className={`message ${message.role}`} onClick={handleClick}>
        <div
          className="message-content"
          dangerouslySetInnerHTML={{ __html: formatContent(message.content) }}
        />
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
