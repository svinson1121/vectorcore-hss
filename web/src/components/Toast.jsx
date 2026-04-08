import React, { createContext, useContext, useState, useCallback, useEffect } from 'react'
import { CheckCircle, XCircle, AlertTriangle, Info, X } from 'lucide-react'

const ToastContext = createContext(null)
let toastId = 0

const ICONS = {
  success: <CheckCircle size={16} color="var(--success)" />,
  error: <XCircle size={16} color="var(--danger)" />,
  warning: <AlertTriangle size={16} color="var(--warning)" />,
  info: <Info size={16} color="var(--info)" />,
}

function ToastItem({ id, type, title, message, onDismiss }) {
  useEffect(() => {
    const t = setTimeout(() => onDismiss(id), 4000)
    return () => clearTimeout(t)
  }, [id, onDismiss])

  return (
    <div className={`toast ${type}`} role="alert">
      <div className="toast-icon">{ICONS[type] || ICONS.info}</div>
      <div className="toast-body">
        <div className="toast-title">{title}</div>
        {message && <div className="toast-message">{message}</div>}
      </div>
      <button className="toast-close" onClick={() => onDismiss(id)} aria-label="Dismiss">
        <X size={14} />
      </button>
    </div>
  )
}

export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([])

  const dismiss = useCallback((id) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  const addToast = useCallback((type, title, message) => {
    const id = ++toastId
    setToasts(prev => [...prev.slice(-4), { id, type, title, message }])
  }, [])

  const toast = {
    success: (title, message) => addToast('success', title, message),
    error: (title, message) => addToast('error', title, message),
    warning: (title, message) => addToast('warning', title, message),
    info: (title, message) => addToast('info', title, message),
  }

  return (
    <ToastContext.Provider value={toast}>
      {children}
      <div className="toast-container">
        {toasts.map(t => (
          <ToastItem key={t.id} {...t} onDismiss={dismiss} />
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
