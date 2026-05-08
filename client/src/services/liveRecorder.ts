// 直播取证 5 秒分段录制（论文 4.1.2 节）
// 基于 getDisplayMedia + MediaRecorder，每 5 秒触发 ondataavailable 产出一个 WebM 片段
// 每个片段立即通过 Web Crypto API 计算哈希并调用 /api/notarize/fixate 完成固化

import { fixateEvidence, type NotarizedEvidence } from './evidenceCrypto'

export interface LiveRecorderOptions {
  caseId: string
  segmentDurationMs?: number // 默认 5000
  collector?: string
  /** 分段视频/音频配置 */
  videoBitsPerSecond?: number
  /** 片段固化完成回调 */
  onSegmentFixed?: (seq: number, evidence: NotarizedEvidence) => void
  /** 分段错误回调 */
  onSegmentError?: (seq: number, err: Error) => void
  /** 停止回调 */
  onStopped?: (total: number) => void
  /** 状态回调 */
  onStatus?: (status: string, extra?: Record<string, unknown>) => void
}

const DEFAULT_MIME_CANDIDATES = [
  'video/webm;codecs=vp9,opus',
  'video/webm;codecs=vp8,opus',
  'video/webm;codecs=vp9',
  'video/webm;codecs=vp8',
  'video/webm',
]

function pickMimeType(): string {
  for (const t of DEFAULT_MIME_CANDIDATES) {
    if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(t)) {
      return t
    }
  }
  return 'video/webm'
}

/**
 * LiveRecorder：分段录屏器（单例风格）
 *
 * 使用：
 *   const rec = new LiveRecorder({ caseId: 'case_001', onSegmentFixed: ... })
 *   await rec.start()
 *   ...
 *   rec.stop()
 */
export class LiveRecorder {
  private stream: MediaStream | null = null
  private recorder: MediaRecorder | null = null
  private mimeType: string
  private segmentDurationMs: number
  private seq = 0
  private fixedCount = 0
  private options: LiveRecorderOptions
  private stopped = false

  constructor(options: LiveRecorderOptions) {
    this.options = options
    this.segmentDurationMs = options.segmentDurationMs ?? 5000
    this.mimeType = pickMimeType()
  }

  /** 启动取证：申请屏幕捕获权限，开始分段录制 */
  async start(): Promise<void> {
    if (!navigator.mediaDevices || !('getDisplayMedia' in navigator.mediaDevices)) {
      throw new Error('当前浏览器不支持 getDisplayMedia，请使用 Chrome/Edge 124+')
    }

    this.options.onStatus?.('requesting_permission')

    const constraints: DisplayMediaStreamOptions = {
      video: {
        frameRate: 30,
      },
      audio: true,
    }

    const stream = await (navigator.mediaDevices as MediaDevices).getDisplayMedia(constraints)
    this.stream = stream
    this.options.onStatus?.('permission_granted')

    // 监听用户手动停止共享
    stream.getVideoTracks().forEach((track) => {
      track.addEventListener('ended', () => {
        this.options.onStatus?.('share_ended_by_user')
        this.stop()
      })
    })

    const recorder = new MediaRecorder(stream, {
      mimeType: this.mimeType,
      // 取证场景优先保证可验证性而非画质，约 1 Mbps → 5 秒分段约 625 KB
      videoBitsPerSecond: this.options.videoBitsPerSecond ?? 1_000_000,
    })

    recorder.ondataavailable = (e) => {
      if (!e.data || e.data.size === 0) return
      const seq = this.seq++
      // 不阻塞录制线程，异步处理
      this.handleSegment(seq, e.data).catch((err) => {
        this.options.onSegmentError?.(seq, err as Error)
      })
    }

    recorder.onstop = () => {
      this.options.onStopped?.(this.seq)
    }

    this.recorder = recorder
    recorder.start(this.segmentDurationMs)
    this.options.onStatus?.('recording', { mimeType: this.mimeType, segmentMs: this.segmentDurationMs })
  }

  /** 停止录制 */
  stop(): void {
    if (this.stopped) return
    this.stopped = true

    if (this.recorder && this.recorder.state !== 'inactive') {
      try {
        this.recorder.stop()
      } catch {
        // noop
      }
    }
    if (this.stream) {
      this.stream.getTracks().forEach((t) => {
        try {
          t.stop()
        } catch {
          // noop
        }
      })
    }
    this.options.onStatus?.('stopped', { totalSegments: this.seq })
  }

  /** 获取状态 */
  isRecording(): boolean {
    return !!this.recorder && this.recorder.state === 'recording'
  }

  /** 获取已固化片段数 */
  getFixedCount(): number {
    return this.fixedCount
  }

  private async handleSegment(seq: number, blob: Blob): Promise<void> {
    this.options.onStatus?.('segment_received', { seq, size: blob.size })

    const filename = `live_segment_${String(seq).padStart(6, '0')}.webm`

    const evidence = await fixateEvidence(blob, {
      caseId: this.options.caseId,
      sourceType: 'live_segment',
      collector: this.options.collector,
      filename,
      meta: {
        seq: String(seq),
        durationMs: String(this.segmentDurationMs),
        mimeType: this.mimeType,
      },
      onProgress: (stage, extra) => {
        this.options.onStatus?.(`segment_${stage}`, { seq, ...extra })
      },
    })

    this.fixedCount++
    this.options.onSegmentFixed?.(seq, evidence)
  }
}
