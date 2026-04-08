import React, { useEffect, useRef } from 'react'
import { X } from 'lucide-react'

export default function Modal({ title, children, onClose, size = '' }) {
  const overlayRef = useRef(null)
  const firstFocusRef = useRef(null)
  const onCloseRef = useRef(onClose)
  useEffect(() => { onCloseRef.current = onClose })

  useEffect(() => {
    const prev = document.activeElement
    if (firstFocusRef.current) firstFocusRef.current.focus()

    const handleKey = (e) => {
      if (e.key === 'Escape') onCloseRef.current()
    }
    document.addEventListener('keydown', handleKey)
    return () => {
      document.removeEventListener('keydown', handleKey)
      if (prev && prev.focus) prev.focus()
    }
  }, [])

  function handleOverlayClick(e) {
    if (e.target === overlayRef.current) onClose()
  }

  return (
    <div className="modal-overlay" ref={overlayRef} onClick={handleOverlayClick} role="dialog" aria-modal="true">
      <div className={`modal ${size ? 'modal-' + size : ''}`} role="document">
        <div className="modal-header">
          <h3 className="modal-title" ref={firstFocusRef} tabIndex={-1}>{title}</h3>
          <button className="btn-icon" onClick={onClose} aria-label="Close modal">
            <X size={16} />
          </button>
        </div>
        {children}
      </div>
    </div>
  )
}
