import { useCallback, useState } from 'react'

// Gates a modal's close action behind an in-app confirmation when the form
// is dirty, instead of the browser's window.confirm() — which always shows
// its own unstyleable "<origin> says" chrome. Exposes the open/confirm/cancel
// state for a <DiscardConfirm /> to render.
export function useConfirmClose(dirty, onClose) {
  const [confirming, setConfirming] = useState(false)

  const requestClose = useCallback(() => {
    if (dirty) setConfirming(true)
    else onClose()
  }, [dirty, onClose])

  const confirmDiscard = useCallback(() => {
    setConfirming(false)
    onClose()
  }, [onClose])

  const cancelDiscard = useCallback(() => setConfirming(false), [])

  return { requestClose, confirming, confirmDiscard, cancelDiscard }
}
