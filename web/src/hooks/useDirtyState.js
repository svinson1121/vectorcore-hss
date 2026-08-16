import { useCallback, useState } from 'react'

// Drop-in replacement for useState that also reports whether the setter has
// ever been called by the user, so Add/Edit forms can track "has this been
// touched" without hand-rolling a dirty flag next to every field mutator.
export function useDirtyState(initialValue) {
  const [state, setStateRaw] = useState(initialValue)
  const [dirty, setDirty] = useState(false)

  const setState = useCallback((update) => {
    setDirty(true)
    setStateRaw(update)
  }, [])

  return [state, setState, dirty]
}
