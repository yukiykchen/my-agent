// 直播取证面板（论文 4.1.2 节）
// 基于 MediaRecorder 5 秒分段录制 + Web Crypto API 即时哈希 + 服务端 TSA 固化
import { useEffect, useRef, useState } from 'react'
import { LiveRecorder } from '../services/liveRecorder'
import type { NotarizedEvidence } from '../services/evidenceCrypto'

interface Props {
  caseId: string
  onClose: () => void
}

interface SegmentRecord {
  seq: number
  status: 'hashing' | 'uploading' | 'timestamping' | 'done' | 'error'
  clientHash?: string
  tsaSerial?: string
  latencyMs?: number
  fileSize?: number
  error?: string
}

export default function LiveCapturePanel({ caseId, onClose }: Props) {
  const [recording, setRecording] = useState(false)
  const [segments, setSegments] = useState<SegmentRecord[]>([])
  const [startedAt, setStartedAt] = useState<number | null>(null)
  const [elapsed, setElapsed] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const recorderRef = useRef<LiveRecorder | null>(null)
  const segmentsRef = useRef<SegmentRecord[]>([])

  useEffect(() => {
    if (!recording) return
    const timer = setInterval(() => {
      if (startedAt) {
        setElapsed(Math.floor((Date.now() - startedAt) / 1000))
      }
    }, 1000)
    return () => clearInterval(timer)
  }, [recording, startedAt])

  const updateSegment = (seq: number, patch: Partial<SegmentRecord>) => {
    segmentsRef.current = segmentsRef.current.map((s) =>
      s.seq === seq ? { ...s, ...patch } : s
    )
    setSegments([...segmentsRef.current])
  }

  const addSegment = (rec: SegmentRecord) => {
    // 避免重复
    if (segmentsRef.current.find((s) => s.seq === rec.seq)) {
      updateSegment(rec.seq, rec)
      return
    }
    segmentsRef.current = [rec, ...segmentsRef.current]
    setSegments([...segmentsRef.current])
  }

  const handleStart = async () => {
    setError(null)
    segmentsRef.current = []
    setSegments([])
    try {
      const rec = new LiveRecorder({
        caseId,
        collector: 'operator',
        onStatus: (status, extra) => {
          // 根据分段阶段更新 UI
          if (!extra) return
          const seq = (extra as { seq?: number }).seq
          if (typeof seq !== 'number') return
          if (status === 'segment_received') {
            addSegment({ seq, status: 'hashing', fileSize: (extra as { size?: number }).size })
          } else if (status === 'segment_hashing_done') {
            updateSegment(seq, { status: 'uploading', clientHash: (extra as { clientHash?: string }).clientHash })
          } else if (status === 'segment_uploading') {
            updateSegment(seq, { status: 'timestamping' })
          }
        },
        onSegmentFixed: (seq: number, ev: NotarizedEvidence) => {
          updateSegment(seq, {
            status: 'done',
            tsaSerial: ev.tsaToken?.serialNumber,
            latencyMs: ev.fixationLatencyMs,
            fileSize: ev.fileSize,
          })
        },
        onSegmentError: (seq: number, err: Error) => {
          updateSegment(seq, { status: 'error', error: err.message })
        },
        onStopped: () => {
          setRecording(false)
        },
      })
      recorderRef.current = rec
      await rec.start()
      setStartedAt(Date.now())
      setRecording(true)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const handleStop = () => {
    recorderRef.current?.stop()
  }

  useEffect(() => {
    return () => {
      recorderRef.current?.stop()
    }
  }, [])

  const fixedCount = segments.filter((s) => s.status === 'done').length
  const avgLatency =
    fixedCount > 0
      ? Math.round(
          segments
            .filter((s) => s.latencyMs)
            .reduce((a, b) => a + (b.latencyMs || 0), 0) / fixedCount
        )
      : 0

  return (
    <div className="live-capture-panel">
      <div className="live-capture-header">
        <h3>🔴 直播取证（5 秒分段即时固化）</h3>
        <button className="close-btn" onClick={onClose}>✕</button>
      </div>

      <div className="live-capture-info">
        <div className="info-row">
          <span>案件 ID:</span> <code>{caseId}</code>
        </div>
        <div className="info-row">
          <span>已录制:</span>{' '}
          <strong>
            {Math.floor(elapsed / 60)}:{String(elapsed % 60).padStart(2, '0')}
          </strong>
        </div>
        <div className="info-row">
          <span>分段总数:</span> <strong>{segments.length}</strong>
        </div>
        <div className="info-row">
          <span>已固化:</span>{' '}
          <strong className="text-green">{fixedCount}</strong>
        </div>
        <div className="info-row">
          <span>平均固化延时:</span>{' '}
          <strong>{avgLatency} ms</strong>
        </div>
      </div>

      {error && <div className="live-capture-error">⚠️ {error}</div>}

      <div className="live-capture-actions">
        {!recording ? (
          <button className="btn-primary" onClick={handleStart}>
            🎬 开始取证
          </button>
        ) : (
          <button className="btn-danger" onClick={handleStop}>
            ⏹ 停止取证
          </button>
        )}
      </div>

      <div className="live-capture-segments">
        <div className="segments-header">
          <span>#</span>
          <span>状态</span>
          <span>客户端哈希</span>
          <span>TSA 序列号</span>
          <span>大小</span>
          <span>固化耗时</span>
        </div>
        {segments.length === 0 && (
          <div className="segments-empty">暂无分段数据，点击"开始取证"启动录制</div>
        )}
        {segments.map((s) => (
          <div key={s.seq} className={`segment-row status-${s.status}`}>
            <span>#{s.seq}</span>
            <span>
              {s.status === 'hashing' && '🔐 哈希中'}
              {s.status === 'uploading' && '📤 上传中'}
              {s.status === 'timestamping' && '⏱ TSA 中'}
              {s.status === 'done' && '✅ 已固化'}
              {s.status === 'error' && `❌ ${s.error}`}
            </span>
            <span className="mono">{s.clientHash?.slice(0, 12) || '—'}</span>
            <span className="mono">{s.tsaSerial || '—'}</span>
            <span>{s.fileSize ? `${(s.fileSize / 1024).toFixed(1)} KB` : '—'}</span>
            <span>{s.latencyMs ? `${s.latencyMs} ms` : '—'}</span>
          </div>
        ))}
      </div>

      <style>{`
        .live-capture-panel {
          background: rgba(10, 15, 25, 0.95);
          border: 1px solid rgba(100, 200, 255, 0.3);
          border-radius: 12px;
          padding: 20px;
          color: #e8f0ff;
          max-height: 80vh;
          overflow-y: auto;
        }
        .live-capture-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          margin-bottom: 16px;
          border-bottom: 1px solid rgba(100, 200, 255, 0.2);
          padding-bottom: 12px;
        }
        .live-capture-header h3 { margin: 0; font-size: 18px; }
        .close-btn {
          background: transparent; border: 1px solid #555;
          color: #ccc; padding: 4px 10px; border-radius: 6px;
          cursor: pointer;
        }
        .live-capture-info {
          display: grid;
          grid-template-columns: repeat(2, 1fr);
          gap: 8px 16px;
          margin-bottom: 16px;
          background: rgba(255,255,255,0.03);
          padding: 12px;
          border-radius: 8px;
        }
        .info-row { font-size: 13px; }
        .info-row span:first-child { color: #8fa3bd; margin-right: 8px; }
        .info-row code { color: #7fd1ff; font-size: 12px; }
        .live-capture-error {
          background: rgba(255, 80, 80, 0.15);
          border: 1px solid rgba(255, 80, 80, 0.4);
          padding: 10px; border-radius: 6px; margin-bottom: 12px;
          font-size: 13px;
        }
        .live-capture-actions { margin-bottom: 16px; }
        .btn-primary {
          background: linear-gradient(135deg, #3b82f6, #1d4ed8);
          color: white; border: none; padding: 10px 24px;
          border-radius: 6px; cursor: pointer; font-size: 14px;
          font-weight: 500;
        }
        .btn-danger {
          background: linear-gradient(135deg, #ef4444, #b91c1c);
          color: white; border: none; padding: 10px 24px;
          border-radius: 6px; cursor: pointer; font-size: 14px;
        }
        .live-capture-segments {
          background: rgba(0,0,0,0.3);
          border-radius: 8px;
          overflow: hidden;
        }
        .segments-header, .segment-row {
          display: grid;
          grid-template-columns: 50px 100px 1.2fr 1fr 90px 90px;
          gap: 8px;
          padding: 8px 12px;
          font-size: 12px;
          border-bottom: 1px solid rgba(255,255,255,0.05);
        }
        .segments-header { background: rgba(255,255,255,0.06); font-weight: 600; color: #8fa3bd; }
        .segments-empty { padding: 20px; text-align: center; color: #6b7a90; font-size: 13px; }
        .segment-row.status-done { color: #6ee7b7; }
        .segment-row.status-error { color: #fca5a5; }
        .segment-row.status-hashing, .segment-row.status-uploading, .segment-row.status-timestamping {
          color: #fcd34d;
        }
        .mono { font-family: 'SF Mono', Consolas, monospace; }
        .text-green { color: #4ade80; }
      `}</style>
    </div>
  )
}
