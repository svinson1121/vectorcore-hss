import { useState, useEffect, useRef, useCallback } from 'react'

const PAGE_SIZE = 50

/**
 * useInfiniteScroll — server-side paginated infinite scroll.
 *
 * @param {Function} fetchFn  – async fn({ search, limit, offset }) → { total, items }
 * @param {string}   search   – current search string (debounced internally, 300ms)
 *
 * @returns {{ items, total, loading, loadingMore, error, sentinelRef, refresh }}
 *   sentinelRef – attach to a <div> at the bottom of your list to trigger loading
 */
export function useInfiniteScroll(fetchFn, search = '') {
  const [items, setItems]           = useState([])
  const [total, setTotal]           = useState(null)
  const [loading, setLoading]       = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError]           = useState(null)

  const offsetRef      = useRef(0)
  const searchRef      = useRef(search)   // tracks the in-flight search term
  const generationRef  = useRef(0)        // increment on each reset to discard stale responses
  const debounceRef    = useRef(null)
  const sentinelRef    = useRef(null)
  const observerRef    = useRef(null)
  const isFirstRender  = useRef(true)
  const hasLoadedRef   = useRef(false)    // true after the first successful fetch; prevents full-page spinner on search/refresh

  const fetchPage = useCallback(async (searchTerm, offset, reset) => {
    const gen = ++generationRef.current
    // Only show the full-page spinner on the very first load.
    // Search-triggered and refresh resets use loadingMore so the UI (and
    // search input focus) are never unmounted.
    const fullSpinner = reset && !hasLoadedRef.current
    if (fullSpinner) setLoading(true)
    else setLoadingMore(true)
    try {
      const result = await fetchFn({ search: searchTerm, limit: PAGE_SIZE, offset })
      if (gen !== generationRef.current) return  // stale response — a newer search superseded this
      const newItems = result.items ?? []
      setItems(prev => reset ? newItems : [...prev, ...newItems])
      setTotal(result.total ?? 0)
      offsetRef.current = offset + newItems.length
      setError(null)
      hasLoadedRef.current = true
    } catch (err) {
      if (gen !== generationRef.current) return
      setError(err.message)
    } finally {
      if (gen === generationRef.current) {
        if (fullSpinner) setLoading(false)
        else setLoadingMore(false)
      }
    }
  }, [fetchFn])

  // Initial load + debounced re-load on search change
  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false
      searchRef.current = search
      fetchPage(search, 0, true)
      return
    }
    clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      searchRef.current = search
      offsetRef.current = 0
      // Don't clear items/total here — keep old rows visible while the new
      // results are in flight so the UI doesn't flash empty and the search
      // input never loses focus. fetchPage (reset=true) replaces items on arrival.
      fetchPage(search, 0, true)
    }, 300)
    return () => clearTimeout(debounceRef.current)
  }, [search]) // eslint-disable-line react-hooks/exhaustive-deps

  // IntersectionObserver — triggers next page when sentinel scrolls into view
  const hasMore = total !== null && items.length < total

  useEffect(() => {
    if (observerRef.current) observerRef.current.disconnect()
    if (!hasMore || loading || loadingMore) return

    observerRef.current = new IntersectionObserver(entries => {
      if (entries[0].isIntersecting) {
        fetchPage(searchRef.current, offsetRef.current, false)
      }
    }, { rootMargin: '300px' })

    if (sentinelRef.current) observerRef.current.observe(sentinelRef.current)
    return () => observerRef.current?.disconnect()
  }, [hasMore, loading, loadingMore, fetchPage])

  function refresh() {
    searchRef.current = search
    offsetRef.current = 0
    setItems([])
    setTotal(null)
    fetchPage(search, 0, true)
  }

  return { items, total, loading, loadingMore, error, sentinelRef, refresh }
}
