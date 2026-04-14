import React, { useEffect, useState } from 'react'

export interface RestartEvent {
  type: 'agent_restart'
  service_id: string
  restart_detected_at: string
  replay_window_minutes: number
}

interface Props {
  event: RestartEvent | null
}

export default function RestartBanner({ event }: Props) {
  const [visible, setVisible] = useState(false)
  const [currentEvent, setCurrentEvent] = useState<RestartEvent | null>(null)

  useEffect(() => {
    if (event) {
      setCurrentEvent(event)
      setVisible(true)
      const timer = setTimeout(() => setVisible(false), 60_000) // auto-hide after 60s
      return () => clearTimeout(timer)
    }
  }, [event])

  if (!visible || !currentEvent) return null

  return (
    <div className="flex items-center justify-between px-4 py-2 bg-yellow-900/40 border-b border-yellow-700/50 text-yellow-300 text-sm">
      <div className="flex items-center gap-2">
        <span className="text-yellow-400 text-base">&#x26A0;&#xFE0F;</span>
        <span>
          Agent <strong>{currentEvent.service_id}</strong> reiniciou — reenviando logs dos últimos{' '}
          {currentEvent.replay_window_minutes} min. Duplicatas são descartadas automaticamente.
        </span>
      </div>
      <button
        onClick={() => setVisible(false)}
        className="ml-4 text-yellow-500 hover:text-yellow-300 text-lg leading-none"
        title="Dispensar"
      >
        &#x2715;
      </button>
    </div>
  )
}
