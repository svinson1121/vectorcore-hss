import React, { useState, useEffect } from 'react'
import { Sun, Moon } from 'lucide-react'
import { useTheme } from '../theme.jsx'
import { getVersion } from '../api/client.js'

export default function TopBar() {
  const { theme, toggleTheme } = useTheme()
  const [version, setVersion] = useState(null)
  const [connected, setConnected] = useState(true)

  useEffect(() => {
    let mounted = true
    let timer

    async function fetchVersion() {
      try {
        const v = await getVersion()
        if (mounted) {
          setVersion(v)
          setConnected(true)
        }
      } catch {
        if (mounted) setConnected(false)
      }
      if (mounted) timer = setTimeout(fetchVersion, 10000)
    }

    fetchVersion()
    return () => {
      mounted = false
      clearTimeout(timer)
    }
  }, [])

  return (
    <header className="topbar">
      <div className="topbar-identity">
        {version?.app_version && (
          <span className="topbar-identity-fqdn mono">VectorCore HSS v{version.app_version}</span>
        )}
      </div>

      <div className="topbar-right">
        <div className="connection-indicator">
          <div className={`connection-dot ${connected ? 'connected' : 'error'}`} />
          <span>{connected ? 'Connected' : 'Disconnected'}</span>
        </div>

        <button
          className="btn-icon"
          onClick={toggleTheme}
          aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
          title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
        >
          {theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}
        </button>
      </div>
    </header>
  )
}
