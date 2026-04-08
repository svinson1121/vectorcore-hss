import { useState, useMemo } from 'react'

/**
 * useSort – client-side sort state for a list of objects.
 *
 * @param {Array}  items      – the array to sort (filtered or raw)
 * @param {string} defaultKey – initial sort column key (or null for no sort)
 * @param {string} defaultDir – 'asc' | 'desc'
 * @returns {{ sorted, sortKey, sortDir, handleSort }}
 */
export function useSort(items, defaultKey = null, defaultDir = 'asc') {
  const [sortKey, setSortKey] = useState(defaultKey)
  const [sortDir, setSortDir] = useState(defaultDir)

  function handleSort(key) {
    if (key === sortKey) {
      setSortDir(d => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir('asc')
    }
  }

  const sorted = useMemo(() => {
    if (!sortKey) return items
    return [...items].sort((a, b) => {
      const av = a[sortKey] ?? ''
      const bv = b[sortKey] ?? ''
      let cmp
      if (typeof av === 'number' && typeof bv === 'number') {
        cmp = av - bv
      } else if (typeof av === 'boolean' && typeof bv === 'boolean') {
        cmp = (av === bv) ? 0 : av ? 1 : -1
      } else {
        cmp = String(av).localeCompare(String(bv), undefined, { numeric: true })
      }
      return sortDir === 'asc' ? cmp : -cmp
    })
  }, [items, sortKey, sortDir])

  return { sorted, sortKey, sortDir, handleSort }
}
