import React from 'react'

export default function StatCard({ title, value, unit, icon, color = 'var(--accent)', subtitle }) {
  return (
    <div className="stat-card">
      {icon && (
        <div className="stat-card-icon" style={{ background: color + '20', color }}>
          {icon}
        </div>
      )}
      <div className="stat-card-body">
        <div className="stat-card-value" style={{ color }}>
          {value !== undefined && value !== null ? value : '—'}
          {unit && <span style={{ fontSize: '1rem', fontWeight: 400, color: 'var(--text-muted)', marginLeft: 4 }}>{unit}</span>}
        </div>
        <div className="stat-card-label">{title}</div>
        {subtitle && <div className="stat-card-subtitle">{subtitle}</div>}
      </div>
    </div>
  )
}
