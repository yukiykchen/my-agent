import { useState } from 'react'
import type { ToolCallLog } from '../types'

interface Props {
  calls: ToolCallLog[]
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

  return (
    <div className={`tool-call-item ${call.success ? 'success' : 'error'}`}>
      <div className="tool-call-name">
        <span className={`status-dot ${call.success ? 'success' : 'error'}`} />
        <strong>{call.tool}</strong>
        <span className="tool-server">{call.server}</span>
        <span className="tool-duration">{call.duration}ms</span>
      </div>

      <div className="tool-call-toggle" onClick={() => setOpen(!open)}>
        {open ? '收起详情 ▴' : '查看详情 ▾'}
      </div>

      {open && (
        <div className="tool-call-details">
          <div className="detail-label">参数:</div>
          <pre>{JSON.stringify(call.args, null, 2)}</pre>
          <div className="detail-label">结果:</div>
          <pre>{call.result.length > 500 ? call.result.slice(0, 500) + '...' : call.result}</pre>
        </div>
      )}
    </div>
  )
}
