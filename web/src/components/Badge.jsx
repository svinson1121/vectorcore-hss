import React from 'react'

const STATE_MAP = {
  enabled: { cls: 'badge-open', label: 'enabled' },
  disabled: { cls: 'badge-disabled', label: 'disabled' },
  REGISTERED: { cls: 'badge-open', label: 'REGISTERED' },
  PURGED: { cls: 'badge-disabled', label: 'PURGED' },
  OPEN: { cls: 'badge-open', label: 'OPEN' },
  CLOSED: { cls: 'badge-closed', label: 'CLOSED' },
  connected: { cls: 'badge-open', label: 'connected' },
  disconnected: { cls: 'badge-closed', label: 'disconnected' },
  '4G': { cls: 'badge-info', label: '4G' },
  LTE: { cls: 'badge-info', label: 'LTE' },
  '5G': { cls: 'badge-active', label: '5G' },
  NR: { cls: 'badge-active', label: 'NR' },
}

export default function Badge({ state, label: labelOverride }) {
  if (!state) return null
  const entry = STATE_MAP[state] || { cls: 'badge-disabled', label: state }
  return (
    <span className={`badge ${entry.cls}`}>
      {labelOverride || entry.label}
    </span>
  )
}
