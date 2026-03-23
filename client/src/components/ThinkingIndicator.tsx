import type { WSMessage } from '../types'

interface Props {
  step: string | null
  toolEvents: WSMessage[]
  status: string
}

export default function ThinkingIndicator({ step, toolEvents, status }: Props) {
  // 检测是否正在生成阶段性总结
  const isSummarizing = step?.includes('阶段性总结') || step?.includes('推理上限')

  return (
    <div className={`thinking-indicator ${isSummarizing ? 'summarizing' : ''}`}>
      <div className="thinking-dots">
        <span /><span /><span />
      </div>
      <div className="thinking-info">
        {step && (
          <div className={`thinking-step ${isSummarizing ? 'thinking-step-warning' : ''}`}>
            {isSummarizing && '⚠️ '}{step}
          </div>
        )}
        {toolEvents.length > 0 && (
          <div className="thinking-tools">
            {toolEvents.slice(-3).map((evt, i) => (
              <div key={i} className="thinking-tool-item">
                🔧 {evt.tool}
                {evt.duration != null && <span className="tool-duration">{evt.duration}ms</span>}
              </div>
            ))}
          </div>
        )}
        {status === 'thinking' && !step && (
          <div className="thinking-step">思考中...</div>
        )}
      </div>
    </div>
  )
}
