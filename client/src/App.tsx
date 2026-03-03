import { useState } from 'react'
import SetupPanel from './components/SetupPanel'
import ChatPanel from './components/ChatPanel'
import './App.css'

interface SessionInfo {
  sessionId: string
  provider: string
  toolCount: number
  template: string
}

export default function App() {
  const [session, setSession] = useState<SessionInfo | null>(null)

  const handleSessionCreated = (info: SessionInfo) => {
    setSession(info)
  }

  const handleBackToSetup = () => {
    setSession(null)
  }

  return (
    <div className="app">
      {!session ? (
        <SetupPanel onSessionCreated={handleSessionCreated} />
      ) : (
        <ChatPanel session={session} onBack={handleBackToSetup} />
      )}
    </div>
  )
}
