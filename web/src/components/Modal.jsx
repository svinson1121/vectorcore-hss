import React, { useEffect, useRef } from 'react'
import { X } from 'lucide-react'

function isEditableTarget(el) {
  if (!el) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable
}

export default function Modal({ title, children, onClose, size = '', closeOnBackdrop = true, closeOnEscape = true }) {
  const overlayRef = useRef(null)
  const firstFocusRef = useRef(null)
  const onCloseRef = useRef(onClose)
  const closeOnEscapeRef = useRef(closeOnEscape)
  useEffect(() => { onCloseRef.current = onClose })
  useEffect(() => { closeOnEscapeRef.current = closeOnEscape })

  useEffect(() => {
    const prev = document.activeElement
    if (firstFocusRef.current) firstFocusRef.current.focus()

    const handleKey = (e) => {
      if (e.key === 'Escape') {
        if (closeOnEscapeRef.current) onCloseRef.current()
        return
      }
      // The browser's legacy "Backspace navigates back" behavior fires
      // whenever focus sits on a non-editable element (e.g. this dialog's
      // own title, which is focused on open) — silently unmounting the
      // dialog via history navigation. Suppress it outside editable controls
      // so Backspace only ever does its normal thing inside inputs.
      if (e.key === 'Backspace' && !isEditableTarget(e.target)) {
        e.preventDefault()
      }
    }
    document.addEventListener('keydown', handleKey)
    return () => {
      document.removeEventListener('keydown', handleKey)
      if (prev && prev.focus) prev.focus()
    }
  }, [])

  function handleOverlayClick(e) {
    if (e.target === overlayRef.current && closeOnBackdrop) onClose()
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
