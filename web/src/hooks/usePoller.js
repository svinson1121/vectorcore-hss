import { useState, useEffect, useCallback, useRef } from 'react'

export function usePoller(fetchFn, interval = 5000) {
  const [data, setData] = useState(null)
  const [error, setError] = useState(null)
  const [loading, setLoading] = useState(true)
  const fetchFnRef = useRef(fetchFn)
  const timerRef = useRef(null)
  const mountedRef = useRef(true)

  // Keep ref up to date without re-triggering the effect
  useEffect(() => {
    fetchFnRef.current = fetchFn
  })

  const doFetch = useCallback(async () => {
    try {
      const result = await fetchFnRef.current()
      if (mountedRef.current) {
        setData(result)
        setError(null)
        setLoading(false)
      }
    } catch (err) {
      if (mountedRef.current) {
        setError(err.message || String(err))
        setLoading(false)
      }
    }
  }, [])

  const refresh = useCallback(() => {
    doFetch()
  }, [doFetch])

  useEffect(() => {
    mountedRef.current = true

    doFetch()

    timerRef.current = setInterval(doFetch, interval)

    return () => {
      mountedRef.current = false
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [doFetch, interval])

  return { data, error, loading, refresh }
}
