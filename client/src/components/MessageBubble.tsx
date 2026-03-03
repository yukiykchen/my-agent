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

  // Inline code
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')

  // Bold
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')

  // Newlines
  html = html.replace(/\n/g, '<br>')

  return html
}

export default function MessageBubble({ message }: Props) {
  return (
    <div className={`message ${message.role}`}>
      <div
        className="message-content"
        dangerouslySetInnerHTML={{ __html: formatContent(message.content) }}
      />
    </div>
  )
}
