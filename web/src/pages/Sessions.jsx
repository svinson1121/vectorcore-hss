import React, { useState, useCallback, useEffect } from 'react'
import { XCircle, RefreshCw } from 'lucide-react'
import Spinner from '../components/Spinner.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getServingAPNs, getPDUSessions, getSubscribers } from '../api/client.js'

const IP_VERSION_LABELS = { 0: 'IPv4', 1: 'IPv6', 2: 'IPv4v6', 3: 'IPv4/v6' }

function formatTs(ts) {
  if (!ts) return '—'
  try { return new Date(ts).toLocaleString() } catch { return String(ts) }
}

function ServingAPNTable({ data, subMap }) {
  const rows = Array.isArray(data) ? data : []
  if (rows.length === 0) return <div className="empty-state">No active 4G PDU sessions.</div>
  return (
    <div className="table-container">
      <table>
        <thead>
          <tr>
            <th>IMSI</th>
            <th>APN</th>
            <th>UE IP</th>
            <th>IP Ver</th>
            <th>Serving PGW</th>
            <th>PGW Realm</th>
            <th>PGW Peer</th>
            <th>PCRF Session</th>
            <th>Timestamp</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => {
            const imsi = subMap[row.subscriber_id] || `sub#${row.subscriber_id}`
            return (
              <tr key={i}>
                <td className="mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>{imsi}</td>
                <td className="mono" style={{ fontSize: '0.82rem' }}>{row.apn_name || '—'}</td>
                <td className="mono" style={{ fontSize: '0.8rem' }}>{row.ue_ip || '—'}</td>
                <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  {IP_VERSION_LABELS[row.ip_version] ?? row.ip_version ?? '—'}
                </td>
                <td className="mono" style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{row.serving_pgw || '—'}</td>
                <td className="mono" style={{ fontSize: '0.72rem', color: 'var(--text-muted)', maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {row.serving_pgw_realm || '—'}
                </td>
                <td className="mono" style={{ fontSize: '0.72rem', color: 'var(--text-muted)', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {row.serving_pgw_peer || '—'}
                </td>
                <td className="mono" style={{ fontSize: '0.7rem', color: 'var(--text-muted)', maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {row.pcrf_session_id || '—'}
                </td>
                <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                  {formatTs(row.serving_pgw_timestamp)}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

function PDUSessionTable({ data }) {
  const rows = Array.isArray(data) ? data : []
  if (rows.length === 0) return <div className="empty-state">No active 5G PDU sessions.</div>
  return (
    <div className="table-container">
      <table>
        <thead>
          <tr>
            <th>IMSI</th>
            <th>PDU Session ID</th>
            <th>DNN</th>
            <th>UE IP</th>
            <th>SMF Address</th>
            <th>Timestamp</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i}>
              <td className="mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>{row.imsi || '—'}</td>
              <td className="mono">{row.pdu_session_id ?? '—'}</td>
              <td className="mono" style={{ fontSize: '0.82rem' }}>{row.dnn || '—'}</td>
              <td className="mono" style={{ fontSize: '0.8rem' }}>{row.ue_ip || '—'}</td>
              <td className="mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{row.smf_address || '—'}</td>
              <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{formatTs(row.timestamp || row.created_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default function Sessions() {
  const [activeTab, setActiveTab] = useState('4g')
  const [subMap, setSubMap] = useState({})

  const fetch4G = useCallback(getServingAPNs, [])
  const fetch5G = useCallback(getPDUSessions, [])

  const { data: apns, error: err4g, loading: loading4g, refresh: refresh4g } = usePoller(fetch4G, 10000)
  const { data: pdus, error: err5g, loading: loading5g, refresh: refresh5g } = usePoller(fetch5G, 10000)

  useEffect(() => {
    getSubscribers().then(subs => {
      const list = subs?.items ?? (Array.isArray(subs) ? subs : [])
      if (!list.length) return
      const m = {}
      list.forEach(s => { m[s.subscriber_id] = s.imsi })
      setSubMap(m)
    }).catch(() => {})
  }, [])

  const loading = loading4g || loading5g
  if (loading) {
    return (
      <div className="loading-center">
        <Spinner size="lg" />
        <span>Loading sessions...</span>
      </div>
    )
  }

  const apnList = Array.isArray(apns) ? apns : []
  const pduList = Array.isArray(pdus) ? pdus : []

  return (
    <div>
      <div className="page-header">
        <div>
          <div className="page-title">Sessions</div>
          <div className="page-subtitle">
            {apnList.length} 4G PDU session{apnList.length !== 1 ? 's' : ''} &nbsp;·&nbsp; {pduList.length} 5G PDU session{pduList.length !== 1 ? 's' : ''}
          </div>
        </div>
        <button className="btn btn-ghost" onClick={() => { refresh4g(); refresh5g() }}>
          <RefreshCw size={14} /> Refresh
        </button>
      </div>

      <div className="tabs">
        <button className={`tab-btn${activeTab === '4g' ? ' active' : ''}`} onClick={() => setActiveTab('4g')}>
          4G PDU Sessions ({apnList.length})
        </button>
        <button className={`tab-btn${activeTab === '5g' ? ' active' : ''}`} onClick={() => setActiveTab('5g')}>
          5G PDU Sessions ({pduList.length})
        </button>
      </div>

      {activeTab === '4g' && (
        err4g && apnList.length === 0
          ? <div className="error-state"><XCircle size={24} className="error-icon" /><div>{err4g}</div></div>
          : <ServingAPNTable data={apnList} subMap={subMap} />
      )}
      {activeTab === '5g' && (
        err5g && pduList.length === 0
          ? <div className="error-state"><XCircle size={24} className="error-icon" /><div>{err5g}</div></div>
          : <PDUSessionTable data={pduList} />
      )}
    </div>
  )
}
