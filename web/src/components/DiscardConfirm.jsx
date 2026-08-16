import React, { useEffect } from 'react'

// Small in-app "Discard unsaved changes?" prompt, styled like the rest of
// the app instead of the browser's native confirm() dialog. Stacks above
// the Add/Edit modal it belongs to.
export default function DiscardConfirm({ open, onDiscard, onCancel }) {
  useEffect(() => {
    if (!open) return
    const handleKey = (e) => { if (e.key === 'Escape') onCancel() }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, onCancel])

  if (!open) return null

  function handleOverlayClick(e) {
    if (e.target === e.currentTarget) onCancel()
  }

  return (
    <div className="modal-overlay" style={{ zIndex: 1100 }} onClick={handleOverlayClick} role="alertdialog" aria-modal="true">
      <div className="modal" style={{ maxWidth: 380 }} role="document">
        <div className="modal-body" style={{ paddingTop: 20 }}>
          <p style={{ margin: 0 }}>Discard unsaved changes?</p>
        </div>
        <div className="modal-footer">
          <button type="button" className="btn btn-ghost" onClick={onCancel}>Cancel</button>
          <button type="button" className="btn btn-danger" onClick={onDiscard}>Discard</button>
        </div>
      </div>
    </div>
  )
}
