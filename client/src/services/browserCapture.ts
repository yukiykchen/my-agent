import { fixateEvidence, type NotarizedEvidence } from './evidenceCrypto'

interface BrowserPageCaptureOptions {
  caseId: string
  pageUrl?: string
  collector?: string
  onProgress?: (stage: string, extra?: Record<string, unknown>) => void
}

interface BrowserPageCaptureResult {
  evidence: NotarizedEvidence
  screenshot: {
    screenshotUrl: string
    pageTitle: string
    pageUrl: string
    timestamp: string
    viewport: {
      width: number
      height: number
    }
  }
}

async function captureSelectedTabFrame(): Promise<{ blob: Blob; width: number; height: number }> {
  if (!navigator.mediaDevices || !('getDisplayMedia' in navigator.mediaDevices)) {
    throw new Error('当前浏览器不支持屏幕捕获，请使用 Chrome 或 Edge')
  }

  const stream = await navigator.mediaDevices.getDisplayMedia({
    video: {
      frameRate: 1,
      displaySurface: 'browser',
    } as MediaTrackConstraints,
    audio: false,
  })

  try {
    const video = document.createElement('video')
    video.srcObject = stream
    video.muted = true
    video.playsInline = true

    await new Promise<void>((resolve, reject) => {
      video.onloadedmetadata = () => resolve()
      video.onerror = () => reject(new Error('读取所选页面画面失败'))
    })
    await video.play()

    await new Promise((resolve) => requestAnimationFrame(resolve))

    const width = video.videoWidth
    const height = video.videoHeight
    if (!width || !height) {
      throw new Error('未获取到有效画面，请确认选择的是目标网页标签页')
    }

    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      throw new Error('当前浏览器不支持 Canvas 截图')
    }
    ctx.drawImage(video, 0, 0, width, height)

    const blob = await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob((b) => {
        if (b) resolve(b)
        else reject(new Error('生成截图文件失败'))
      }, 'image/png')
    })

    return { blob, width, height }
  } finally {
    stream.getTracks().forEach((track) => track.stop())
  }
}

export async function captureLoggedInBrowserPage(options: BrowserPageCaptureOptions): Promise<BrowserPageCaptureResult> {
  options.onProgress?.('requesting_permission')
  const { blob, width, height } = await captureSelectedTabFrame()
  const timestamp = new Date().toISOString()
  const filename = `browser_capture_${Date.now()}.png`

  const evidence = await fixateEvidence(blob, {
    caseId: options.caseId,
    sourceType: 'web',
    collector: options.collector || 'browser_tab_capture',
    filename,
    meta: {
      pageURL: options.pageUrl || '',
      captureMethod: 'getDisplayMedia_current_tab',
      timestamp,
      viewportWidth: String(width),
      viewportHeight: String(height),
    },
    onProgress: options.onProgress,
  })

  return {
    evidence,
    screenshot: {
      screenshotUrl: `/api/notarize/${encodeURIComponent(options.caseId)}/${encodeURIComponent(evidence.evidenceId)}/file`,
      pageTitle: '当前浏览器标签页截图',
      pageUrl: options.pageUrl || '',
      timestamp,
      viewport: { width, height },
    },
  }
}
