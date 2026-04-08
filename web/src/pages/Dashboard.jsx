import React, { useState, useEffect, useRef, useCallback } from 'react'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts'
import { Users, Activity, Database, Wifi, XCircle, Shield, Cpu } from 'lucide-react'
import StatCard from '../components/StatCard.jsx'
import Spinner from '../components/Spinner.jsx'
import {
  getSubscribers, getServingAPNs, getPDUSessions, getDiameterPeers,
  getPrometheusText, parsePrometheusText, sumMetric,
} from '../api/client.js'

const CustomTooltip = ({ active, payload, label }) => {
  if (!active || !payload || !payload.length) return null
  return (
    <div style={{
      background: 'var(--bg-elevated)', border: '1px solid var(--border)',
      borderRadius: 'var(--radius-sm)', padding: '8px 12px', fontSize: '0.75rem',
    }}>
      <div style={{ color: 'var(--text-muted)', marginBottom: 4 }}>{label}</div>
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

export default function Dashboard() {
  const [subscribers, setSubscribers] = useState([])
  const [servingAPNs, setServingAPNs] = useState([])
  const [pduSessions, setPDUSessions] = useState([])
  const [peers, setPeers] = useState([])
  const [metrics, setMetrics] = useState({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const timerRef = useRef(null)
  const mountedRef = useRef(true)

  const fetchAll = useCallback(async () => {
    try {
      const [subs, apns, pdus, promText, peersData] = await Promise.all([
        getSubscribers().catch(() => []),
        getServingAPNs().catch(() => []),
        getPDUSessions().catch(() => []),
        getPrometheusText().catch(() => ''),
        getDiameterPeers().catch(() => []),
      ])
      if (!mountedRef.current) return
      setSubscribers(Array.isArray(subs?.items) ? subs.items : (Array.isArray(subs) ? subs : []))
      setServingAPNs(Array.isArray(apns) ? apns : [])
      setPDUSessions(Array.isArray(pdus) ? pdus : [])
      setPeers(Array.isArray(peersData) ? peersData : [])
      setMetrics(parsePrometheusText(promText || ''))
      setError(null)
      setLoading(false)
    } catch (err) {
      if (!mountedRef.current) return
      setError(err.message || 'Failed to load data')
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    mountedRef.current = true
    fetchAll()
    timerRef.current = setInterval(fetchAll, 5000)
    return () => {
      mountedRef.current = false
      clearInterval(timerRef.current)
    }
  }, [fetchAll])

  if (loading) {
    return (
      <div className="loading-center">
        <Spinner size="lg" />
        <span>Loading dashboard...</span>
      </div>
    )
  }

  if (error && subscribers.length === 0) {
    return (
      <div className="error-state">
        <XCircle size={32} className="error-icon" />
        <div>{error}</div>
        <button className="btn btn-ghost mt-12" onClick={fetchAll}>Retry</button>
      </div>
    )
  }

  const totalRequests = sumMetric(metrics, 'hss_diameter_requests_total')
  const cacheHits    = sumMetric(metrics, 'hss_cache_hits_total')
  const apiTotal     = sumMetric(metrics, 'hss_api_requests_total')
  const cmdData      = buildCommandData(metrics)

  return (
    <div>
      <div className="page-header">
        <div>
          <div className="page-title">Dashboard</div>
          <div className="page-subtitle">Home Subscriber Server — real-time overview</div>
        </div>
      </div>

      {/* Subscriber / Session stats */}
      <div className="stats-grid">
        <StatCard title="Total Subscribers"       value={subscribers.length.toLocaleString()} icon={<Users size={18} />}    color="var(--accent)"  />
        <StatCard title="Active 4G PDU Sessions"   value={servingAPNs.length.toLocaleString()} icon={<Activity size={18} />} color="var(--success)" />
        <StatCard title="Active 5G PDU Sessions"  value={pduSessions.length.toLocaleString()} icon={<Database size={18} />} color="var(--warning)" />
        <StatCard title="Connected Diameter Peers" value={peers.length.toLocaleString()}       icon={<Wifi size={18} />}     color="var(--info)"   />
      </div>

      {/* Prometheus / Metrics block */}
      <div className="stats-grid" style={{ marginTop: 0 }}>
        <StatCard title="Diameter Requests" value={totalRequests.toLocaleString()} icon={<Activity size={18} />} color="var(--accent)"  />
        <StatCard title="Cache Hits"        value={cacheHits.toLocaleString()}     icon={<Shield size={18} />}   color="var(--success)" />
        <StatCard title="API Requests"      value={apiTotal.toLocaleString()}       icon={<Database size={18} />} color="var(--warning)" />
        <StatCard title="HSS Metric Series" value={Object.keys(metrics).filter(k => k.startsWith('hss_')).length} icon={<Cpu size={18} />} color="var(--info)" />
      </div>

      {/* Diameter by command chart */}
      {cmdData.length > 0 && (
        <div className="chart-card mb-16">
          <div className="chart-title">Diameter Requests by Command</div>
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={cmdData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border-subtle)" />
              <XAxis dataKey="command" tick={{ fontSize: 11, fill: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }} />
              <YAxis tick={{ fontSize: 10, fill: 'var(--text-muted)' }} width={48} allowDecimals={false} />
              <Tooltip content={<CustomTooltip />} />
              <Bar dataKey="total" name="Total" fill="var(--accent)" radius={[3, 3, 0, 0]} maxBarSize={48} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Connected Diameter Peers table */}
      {peers.length > 0 && (
        <>
          <div className="section-title">Connected Diameter Peers</div>
          <div className="table-container mb-16">
            <table>
              <thead>
                <tr>
                  <th>Origin Host</th>
                  <th>Origin Realm</th>
                  <th>Remote Address</th>
                  <th>Transport</th>
                </tr>
              </thead>
              <tbody>
                {peers.map((peer, i) => (
                  <tr key={i}>
                    <td className="mono" style={{ fontSize: '0.8rem' }}>{peer.origin_host || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{peer.origin_realm || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.8rem' }}>{peer.remote_addr || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.8rem', color: 'var(--accent)' }}>{peer.transport || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  )
}
