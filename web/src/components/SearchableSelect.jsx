import React, { useState, useRef, useEffect, useMemo } from 'react'
import { createPortal } from 'react-dom'
import { Check, ChevronsUpDown } from 'lucide-react'

/**
 * SearchableSelect – a text-filterable dropdown, drop-in replacement for a
 * native <select> when the option list can grow large (e.g. every SIM/AUC
 * or every subscriber). Filters client-side against label + sublabel.
 *
 * @param {{value: string, label: string, sublabel?: string}[]} options
 */
export default function SearchableSelect({
  options,
  value,
  onChange,
  placeholder = '— Select —',
  disabled = false,
  id,
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [highlight, setHighlight] = useState(0)
  const [menuRect, setMenuRect] = useState(null)
  const wrapRef = useRef(null)
  const menuRef = useRef(null)
  const inputRef = useRef(null)

  const selected = options.find(o => String(o.value) === String(value)) || null

  useEffect(() => {
    function onDocMouseDown(e) {
      const insideWrap = wrapRef.current && wrapRef.current.contains(e.target)
      const insideMenu = menuRef.current && menuRef.current.contains(e.target)
      if (!insideWrap && !insideMenu) {
        setOpen(false)
        setQuery('')
      }
    }
    document.addEventListener('mousedown', onDocMouseDown)
    return () => document.removeEventListener('mousedown', onDocMouseDown)
  }, [])

  // The menu is portaled to <body> and fixed-positioned off the trigger's
  // rect — rendering it in-flow inside a scrollable modal body would grow
  // that ancestor's scrollHeight and produce a second, nested scrollbar.
  useEffect(() => {
    if (!open) return
    function updateRect() {
      if (!wrapRef.current) return
      const r = wrapRef.current.getBoundingClientRect()
      setMenuRect({ top: r.bottom + 4, left: r.left, width: r.width })
    }
    updateRect()
    window.addEventListener('scroll', updateRect, true)
    window.addEventListener('resize', updateRect)
    return () => {
      window.removeEventListener('scroll', updateRect, true)
      window.removeEventListener('resize', updateRect)
    }
  }, [open])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return options
    return options.filter(o =>
      (o.label && String(o.label).toLowerCase().includes(q)) ||
      (o.sublabel && String(o.sublabel).toLowerCase().includes(q))
    )
  }, [options, query])

  useEffect(() => { setHighlight(0) }, [query, open])

  function selectOption(opt) {
    onChange(String(opt.value))
    setOpen(false)
    setQuery('')
    inputRef.current?.blur()
  }

  function handleKeyDown(e) {
    if (e.key === 'Escape') {
      if (open) {
        // Stop this from also bubbling up to the enclosing Modal's Escape
        // handler — a first Escape should only close the dropdown.
        e.stopPropagation()
        setOpen(false)
        setQuery('')
      }
      return
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (!open) { setOpen(true); return }
      setHighlight(h => Math.min(h + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (!open) { setOpen(true); return }
      setHighlight(h => Math.max(h - 1, 0))
    } else if (e.key === 'Enter') {
      if (open && filtered[highlight]) {
        e.preventDefault()
        selectOption(filtered[highlight])
      }
    } else if (e.key === 'Tab') {
      setOpen(false)
      setQuery('')
    }
  }

  const displayValue = open ? query : (selected ? selected.label : '')

  return (
    <div className="searchable-select" ref={wrapRef}>
      <input
        id={id}
        ref={inputRef}
        className="input"
        role="combobox"
        aria-expanded={open}
        aria-autocomplete="list"
        autoComplete="off"
        disabled={disabled}
        value={displayValue}
        placeholder={placeholder}
        onFocus={() => { setQuery(''); setOpen(true) }}
        onClick={() => setOpen(true)}
        onChange={e => { setQuery(e.target.value); setOpen(true) }}
        onKeyDown={handleKeyDown}
      />
      <ChevronsUpDown size={13} className="searchable-select-caret" />
      {open && !disabled && menuRect && createPortal(
        <div
          ref={menuRef}
          className="searchable-select-menu"
          style={{ top: menuRect.top, left: menuRect.left, width: menuRect.width }}
        >
          {filtered.length === 0 && (
            <div className="searchable-select-empty">No matches</div>
          )}
          {filtered.map((opt, i) => (
            <div
              key={opt.value}
              className={`searchable-select-option${i === highlight ? ' highlighted' : ''}`}
              onMouseDown={e => { e.preventDefault(); selectOption(opt) }}
              onMouseEnter={() => setHighlight(i)}
            >
              <span>{opt.label}</span>
              {String(opt.value) === String(value) && <Check size={13} />}
            </div>
          ))}
        </div>,
        document.body
      )}
    </div>
  )
}
