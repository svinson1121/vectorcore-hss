import React, { useCallback } from 'react'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts'
import { Activity, Shield, Database, Cpu, XCircle, RefreshCw } from 'lucide-react'
import StatCard from '../components/StatCard.jsx'
import Spinner from '../components/Spinner.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getPrometheusText, parsePrometheusText, sumMetric } from '../api/client.js'

const CustomTooltip = ({ active, payload, label }) => {
  if (!active || !payload || !payload.length) return null
  return (
    <div style={{
      background: 'var(--bg-elevated)', border: '1px solid var(--border)',
      borderRadius: 'var(--radius-sm)', padding: '8px 12px', fontSize: '0.75rem',
    }}>
      <div style={{ color: 'var(--text-muted)', marginBottom: 4, fontFamily: 'var(--font-mono)', fontSize: '0.7rem' }}>
        {label}
      </div>
      {payload.map(p => (
        <div key={p.dataKey} style={{ color: p.fill || p.color }}>
          {p.name}: <strong>{p.value}</strong>
        </div>
      ))}
    </div>
  )
}

function buildCommandData(metrics) {
  const m = metrics['hss_diameter_requests_total']
  if (!m) return []
  const byCmd = {}
  for (const s of m.samples) {
    const cmd = s.labels.command || '?'
    byCmd[cmd] = (byCmd[cmd] || 0) + (isNaN(s.value) ? 0 : s.value)
  }
  return Object.entries(byCmd)
    .map(([command, total]) => ({ command, total }))
    .sort((a, b) => b.total - a.total)
}

function buildHssMetricsTable(metrics) {
  return Object.values(metrics)
    .filter(m => m.name.startsWith('hss_'))
    .map(m => {
      const total = m.samples.reduce((acc, s) => acc + (isNaN(s.value) ? 0 : s.value), 0)
      return { name: m.name, help: m.help, type: m.type, total }
    })
    .sort((a, b) => a.name.localeCompare(b.name))
}

export default function Metrics() {
  const fetchFn = useCallback(getPrometheusText, [])
  const { data: rawText, error, loading, refresh } = usePoller(fetchFn, 10000)

  if (loading) {
    return (
      <div className="loading-center">
        <Spinner size="lg" />
        <span>Loading metrics...</span>
      </div>
    )
  }

  if (error && !rawText) {
    return (
      <div className="error-state">
        <XCircle size={32} className="error-icon" />
        <div>{error}</div>
        <button className="btn btn-ghost mt-12" onClick={refresh}>
          <RefreshCw size={14} /> Retry
        </button>
      </div>
    )
  }

  const metrics = parsePrometheusText(rawText || '')
  const totalRequests = sumMetric(metrics, 'hss_diameter_requests_total')
  const cacheHits = sumMetric(metrics, 'hss_cache_hits_total')
  const apiTotal = sumMetric(metrics, 'hss_api_requests_total')
  const cmdData = buildCommandData(metrics)
  const hssMetrics = buildHssMetricsTable(metrics)

  return (
    <div>
      <div className="page-header">
        <div>
          <div className="page-title">Metrics</div>
          <div className="page-subtitle">Prometheus metrics — 10s polling</div>
        </div>
        <button className="btn btn-ghost" onClick={refresh}>
          <RefreshCw size={14} /> Refresh
        </button>
      </div>

      <div className="stats-grid">
        <StatCard title="Diameter Requests" value={totalRequests.toLocaleString()} icon={<Activity size={18} />} color="var(--accent)" />
        <StatCard title="Cache Hits" value={cacheHits.toLocaleString()} icon={<Shield size={18} />} color="var(--success)" />
        <StatCard title="API Requests" value={apiTotal.toLocaleString()} icon={<Database size={18} />} color="var(--warning)" />
        <StatCard title="HSS Metrics" value={hssMetrics.length} icon={<Cpu size={18} />} color="var(--info)" />
      </div>

      {cmdData.length > 0 && (
        <div className="chart-card mb-16">
          <div className="chart-title">Diameter Requests by Command</div>
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={cmdData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border-subtle)" />
              <XAxis
                dataKey="command"
                tick={{ fontSize: 11, fill: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}
              />
              <YAxis tick={{ fontSize: 10, fill: 'var(--text-muted)' }} width={52} allowDecimals={false} />
              <Tooltip content={<CustomTooltip />} />
              <Bar dataKey="total" name="Total" fill="var(--accent)" radius={[3, 3, 0, 0]} maxBarSize={48} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      <div className="section-title mt-20">All HSS Metrics</div>
      {hssMetrics.length === 0 ? (
        <div className="empty-state">No hss_ metrics available yet.</div>
      ) : (
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Metric</th>
                <th>Type</th>
                <th>Total / Sum</th>
                <th>Help</th>
              </tr>
            </thead>
            <tbody>
              {hssMetrics.map(m => (
                <tr key={m.name}>
                  <td className="mono" style={{ fontSize: '0.78rem', color: 'var(--accent)' }}>{m.name}</td>
                  <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{m.type}</td>
                  <td className="mono" style={{ fontSize: '0.82rem' }}>{m.total.toLocaleString()}</td>
                  <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{m.help || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
