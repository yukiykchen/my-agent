/** WebSocket 连接管理 Hook */

import { useEffect, useRef, useCallback, useState } from 'react'
import type { WSMessage } from '../types'

export function useWebSocket(sessionId: string | null) {
  const wsRef = useRef<WebSocket | null>(null)
  const [connected, setConnected] = useState(false)
  const [thinkingStep, setThinkingStep] = useState<string | null>(null)
  const [toolEvents, setToolEvents] = useState<WSMessage[]>([])
  const [status, setStatus] = useState<string>('idle')

  const connect = useCallback(() => {
    if (!sessionId) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const ws = new WebSocket(`${protocol}//${host}/ws?sessionId=${sessionId}`)

    ws.onopen = () => setConnected(true)

    ws.onmessage = (event) => {
      try {
        const data: WSMessage = JSON.parse(event.data)
        switch (data.type) {
          case 'connected':
            break
          case 'thinking':
            setThinkingStep(data.step || null)
            break
          case 'tool_call':
            setToolEvents((prev) => [...prev, data])
            break
          case 'status':
            setStatus(data.status || 'idle')
            if (data.status === 'done' || data.status === 'error') {
              setThinkingStep(null)
            }
            break
          case 'error':
            console.error('WS error:', data.message)
            break
        }
      } catch {
        // ignore parse errors
      }
    }

    ws.onclose = () => {
      setConnected(false)
    }

    wsRef.current = ws
  }, [sessionId])

  const disconnect = useCallback(() => {
    wsRef.current?.close()
    wsRef.current = null
    setConnected(false)
  }, [])

  const clearToolEvents = useCallback(() => {
    setToolEvents([])
  }, [])

  useEffect(() => {
    connect()
    return () => disconnect()
  }, [connect, disconnect])

  return { connected, thinkingStep, toolEvents, status, clearToolEvents }
}
