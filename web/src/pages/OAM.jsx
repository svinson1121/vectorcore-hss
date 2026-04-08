import React, { useState, useCallback, useEffect, useRef } from 'react'
import { RefreshCw, CheckCircle, XCircle, Activity, Shield, Wifi, HardDrive, Upload, Download, AlertTriangle, RotateCcw, Radio, ChevronUp, ChevronDown, ChevronsUpDown, Eye } from 'lucide-react'
import { useSort } from '../hooks/useSort.js'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getVersion, getHealth, getDiameterPeers, getSubscribers, sendCLR, getEmergencySessions, getOperationLogs, rollbackOperation } from '../api/client.js'
import { useToast } from '../components/Toast.jsx'

function formatUptime(seconds) {
  if (!seconds && seconds !== 0) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (d > 0) return `${d}d ${h}h ${m}m ${s}s`
  if (h > 0) return `${h}h ${m}m ${s}s`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

function formatTs(ts) {
  if (!ts) return '—'
  try { return new Date(ts).toLocaleString() } catch { return String(ts) }
}

function parseChangeRecord(changesStr) {
  if (!changesStr) return null
  try { return JSON.parse(changesStr) } catch { return null }
}

function changesSummary(log) {
  const cr = parseChangeRecord(log.changes)
  if (!cr) return '—'
  const op = (log.operation || '').toLowerCase()
  if (op === 'create') {
    const n = cr.after ? Object.keys(cr.after).length : 0
    return `Created (${n} fields)`
  }
  if (op === 'delete') {
    const n = cr.before ? Object.keys(cr.before).length : 0
    return `Deleted (${n} fields)`
  }
  if (op === 'update') {
    const before = cr.before || {}
    const after = cr.after || {}
    const allKeys = new Set([...Object.keys(before), ...Object.keys(after)])
    let changed = 0
    allKeys.forEach(k => { if (JSON.stringify(before[k]) !== JSON.stringify(after[k])) changed++ })
    return `${changed} field${changed !== 1 ? 's' : ''} changed`
  }
  return '—'
}

function ChangesModal({ log, onClose }) {
  const cr = parseChangeRecord(log.changes)
  const op = (log.operation || '').toLowerCase()

  let rows = []
  if (cr) {
    if (op === 'update') {
      const before = cr.before || {}
      const after = cr.after || {}
      const allKeys = [...new Set([...Object.keys(before), ...Object.keys(after)])].sort()
      rows = allKeys
        .filter(k => JSON.stringify(before[k]) !== JSON.stringify(after[k]))
        .map(k => ({ field: k, before: before[k], after: after[k] }))
    } else if (op === 'create' && cr.after) {
      rows = Object.keys(cr.after).sort().map(k => ({ field: k, before: null, after: cr.after[k] }))
    } else if (op === 'delete' && cr.before) {
      rows = Object.keys(cr.before).sort().map(k => ({ field: k, before: cr.before[k], after: null }))
    }
  }

  const opColor = op === 'delete' ? 'var(--danger)' : op === 'create' ? 'var(--success)' : 'var(--accent)'

  return (
    <Modal title={`Changes — #${log.id} ${log.table_name}`} onClose={onClose} size="xl">
      <div className="modal-body" style={{ padding: 0 }}>
        <div style={{ padding: '10px 16px 8px', display: 'flex', gap: 12, alignItems: 'center', borderBottom: '1px solid var(--border-subtle)' }}>
          <span style={{ fontSize: '0.72rem', fontWeight: 600, padding: '2px 8px', borderRadius: 4, background: `color-mix(in srgb, ${opColor} 12%, transparent)`, color: opColor, textTransform: 'uppercase' }}>{log.operation}</span>
          <span className="mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{log.table_name} #{log.item_id}</span>
          <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginLeft: 'auto' }}>{formatTs(log.timestamp)}</span>
        </div>
        {rows.length === 0 ? (
          <div style={{ padding: '24px 16px', color: 'var(--text-muted)', fontSize: '0.82rem' }}>No change detail available.</div>
        ) : (
          <div style={{ overflowY: 'auto', maxHeight: '60vh' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8rem' }}>
              <thead>
                <tr>
                  <th style={{ textAlign: 'left', padding: '8px 16px', fontSize: '0.7rem', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)', whiteSpace: 'nowrap' }}>Field</th>
                  {op !== 'create' && <th style={{ textAlign: 'left', padding: '8px 16px', fontSize: '0.7rem', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--danger)', borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)', whiteSpace: 'nowrap' }}>Before</th>}
                  {op !== 'delete' && <th style={{ textAlign: 'left', padding: '8px 16px', fontSize: '0.7rem', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--success)', borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)', whiteSpace: 'nowrap' }}>After</th>}
                </tr>
              </thead>
              <tbody>
                {rows.map(({ field, before, after }) => (
                  <tr key={field} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                    <td className="mono" style={{ padding: '8px 16px', fontWeight: 600, color: 'var(--accent)', whiteSpace: 'nowrap', verticalAlign: 'top' }}>{field}</td>
                    {op !== 'create' && (
                      <td className="mono" style={{ padding: '8px 16px', color: 'var(--danger)', wordBreak: 'break-all', maxWidth: 320, verticalAlign: 'top' }}>
                        {before === null || before === undefined ? <span style={{ color: 'var(--text-muted)' }}>null</span> : String(before)}
                      </td>
                    )}
                    {op !== 'delete' && (
                      <td className="mono" style={{ padding: '8px 16px', color: 'var(--success)', wordBreak: 'break-all', maxWidth: 320, verticalAlign: 'top' }}>
                        {after === null || after === undefined ? <span style={{ color: 'var(--text-muted)' }}>null</span> : String(after)}
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
      <div className="modal-footer">
        <button className="btn btn-ghost" onClick={onClose}>Close</button>
      </div>
    </Modal>
  )
}

function OperationLogTable({ logs, loading, onRefresh, onRollback, onViewChanges }) {
  const { sorted, sortKey, sortDir, handleSort } = useSort(logs, 'timestamp', 'desc')

  function SortIcon({ col }) {
    if (sortKey !== col) return <span className="sort-icon"><ChevronsUpDown size={11} /></span>
    return <span className="sort-icon">{sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />}</span>
  }

  return (
    <div className="modal-body" style={{ padding: 0 }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end', padding: '8px 16px 0' }}>
        <button className="btn btn-ghost" style={{ fontSize: '0.78rem' }} onClick={onRefresh} disabled={loading}>
          {loading ? <Spinner size="sm" /> : <RefreshCw size={12} />} Refresh
        </button>
      </div>
      {loading && logs.length === 0 ? (
        <div className="loading-center" style={{ padding: 32 }}><Spinner size="lg" /></div>
      ) : logs.length === 0 ? (
        <div style={{ padding: '24px 16px', color: 'var(--text-muted)', fontSize: '0.82rem' }}>No operation log entries found.</div>
      ) : (
        <div className="table-container" style={{ margin: '8px 0 0', borderRadius: 0, border: 'none' }}>
          <table>
            <thead>
              <tr>
                <th>#</th>
                <th className={`sortable${sortKey === 'table_name' ? ' sort-active' : ''}`} onClick={() => handleSort('table_name')}>Table<SortIcon col="table_name" /></th>
                <th className={`sortable${sortKey === 'operation' ? ' sort-active' : ''}`} onClick={() => handleSort('operation')}>Operation<SortIcon col="operation" /></th>
                <th className={`sortable${sortKey === 'item_id' ? ' sort-active' : ''}`} onClick={() => handleSort('item_id')}>Item ID<SortIcon col="item_id" /></th>
                <th className={`sortable${sortKey === 'timestamp' ? ' sort-active' : ''}`} onClick={() => handleSort('timestamp')}>Timestamp<SortIcon col="timestamp" /></th>
                <th>Changes</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {sorted.map(log => {
                const op = (log.operation || '').toLowerCase()
                const opColor = op === 'delete' ? 'var(--danger)' : op === 'create' ? 'var(--success)' : 'var(--accent)'
                const opBg = op === 'delete' ? 'rgba(239,68,68,0.12)' : op === 'create' ? 'rgba(34,197,94,0.12)' : 'rgba(99,102,241,0.12)'
                return (
                  <tr key={log.id}>
                    <td className="mono" style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{log.id}</td>
                    <td className="mono" style={{ fontSize: '0.78rem' }}>{log.table_name}</td>
                    <td>
                      <span style={{ fontSize: '0.72rem', fontWeight: 600, padding: '2px 6px', borderRadius: 4, background: opBg, color: opColor, textTransform: 'uppercase' }}>
                        {log.operation}
                      </span>
                    </td>
                    <td className="mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{log.item_id}</td>
                    <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>{formatTs(log.timestamp)}</td>
                    <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span>{changesSummary(log)}</span>
                        {log.changes && (
                          <button className="btn-icon" title="View changes" onClick={() => onViewChanges(log)}>
                            <Eye size={13} />
                          </button>
                        )}
                      </div>
                    </td>
                    <td>
                      <button className="btn-icon" title="Rollback" onClick={() => onRollback(log)}>
                        <RotateCcw size={13} />
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

export default function OAM() {
  const toast = useToast()
  const fetchFn = useCallback(getVersion, [])
  const { data: version, error: versionError, loading, refresh } = usePoller(fetchFn, 5000)

  const [health, setHealth] = useState(null)
  const [healthError, setHealthError] = useState(null)
  const [peers, setPeers] = useState([])
  const [subscribers, setSubscribers] = useState([])
  const [emergencySessions, setEmergencySessions] = useState([])
  const [operationLogs, setOperationLogs] = useState([])
  const [logsLoading, setLogsLoading] = useState(false)
  const [logsModalOpen, setLogsModalOpen] = useState(false)
  const healthTimerRef = useRef(null)
  const mountedRef = useRef(true)
  const restoreInputRef = useRef(null)

  const [backupRunning, setBackupRunning] = useState(false)
  const [backupResult, setBackupResult] = useState(null)
  const [restoreRunning, setRestoreRunning] = useState(false)

  // CLR state
  const [clrImsi, setClrImsi] = useState('')
  const [clrCustom, setClrCustom] = useState('')
  const [clrRunning, setClrRunning] = useState(false)

  // Rollback state
  const [rollbackConfirm, setRollbackConfirm] = useState(null)
  const [rollbackRunning, setRollbackRunning] = useState(false)

  // Operation log sort + changes detail
  const [viewChanges, setViewChanges] = useState(null)

  const fetchHealth = useCallback(async () => {
    try {
      const h = await getHealth()
      if (mountedRef.current) { setHealth(h); setHealthError(null) }
    } catch (err) {
      if (mountedRef.current) { setHealth(null); setHealthError(err.message || 'Health check failed') }
    }
  }, [])

  const fetchPeers = useCallback(async () => {
    try {
      const p = await getDiameterPeers()
      if (mountedRef.current) setPeers(Array.isArray(p) ? p : [])
    } catch {}
  }, [])

  const fetchEmergency = useCallback(async () => {
    try {
      const e = await getEmergencySessions()
      if (mountedRef.current) setEmergencySessions(Array.isArray(e) ? e : [])
    } catch {}
  }, [])

  useEffect(() => {
    mountedRef.current = true
    fetchHealth(); fetchPeers(); fetchEmergency()
    getSubscribers().then(d => { if (mountedRef.current) setSubscribers(Array.isArray(d?.items) ? d.items : []) }).catch(() => {})
    healthTimerRef.current = setInterval(() => { fetchHealth(); fetchPeers(); fetchEmergency() }, 5000)
    return () => { mountedRef.current = false; clearInterval(healthTimerRef.current) }
  }, [fetchHealth, fetchPeers, fetchEmergency])

  async function fetchLogs() {
    setLogsLoading(true)
    try {
      const logs = await getOperationLogs()
      setOperationLogs(Array.isArray(logs) ? logs : [])
    } catch (err) { toast.error('Logs failed', err.message) }
    finally { setLogsLoading(false) }
  }

  async function openLogs() {
    setLogsModalOpen(true)
    if (operationLogs.length === 0) await fetchLogs()
  }

  async function doBackup() {
    setBackupRunning(true); setBackupResult(null)
    try {
      const res = await fetch('/api/v1/oam/backup', { method: 'GET' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const blob = await res.blob()
      const cd = res.headers.get('Content-Disposition') || ''
      const match = cd.match(/filename="([^"]+)"/)
      const filename = match ? match[1] : 'hss-backup.json'
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url; a.download = filename; a.click()
      URL.revokeObjectURL(url)
      toast.success('Backup', 'Backup downloaded')
      setBackupResult({ downloaded: true })
    } catch (err) { toast.error('Backup failed', err.message) }
    finally { setBackupRunning(false) }
  }

  async function onRestoreFileChange(e) {
    const file = e.target.files && e.target.files[0]
    if (!file) return
    if (!window.confirm('Restore database from this backup? This will overwrite all current data.')) {
      if (restoreInputRef.current) restoreInputRef.current.value = ''; return
    }
    setRestoreRunning(true)
    try {
      const fd = new FormData(); fd.append('file', file)
      const res = await fetch('/api/v1/oam/restore', { method: 'POST', body: fd })
      if (!res.ok) { const t = await res.text(); throw new Error(t) }
      const result = await res.json()
      toast.success('Restore', result?.message || 'Database restored successfully')
    } catch (err) { toast.error('Restore failed', err.message) }
    finally { setRestoreRunning(false); if (restoreInputRef.current) restoreInputRef.current.value = '' }
  }

  async function doSendCLR() {
    const imsi = clrCustom.trim() || clrImsi
    if (!imsi) { toast.error('CLR', 'Select or enter an IMSI'); return }
    setClrRunning(true)
    try {
      const r = await sendCLR(imsi)
      toast.success('CLR Sent', r?.message || `CLR sent to ${imsi}`)
      setClrImsi(''); setClrCustom('')
    } catch (err) {
      const msg = err.message || ''
      if (msg.includes('no serving MME')) {
        toast.error('CLR not sent', 'Subscriber is not currently attached to any LTE/4G MME. CLR requires an active 4G session (serving_mme must be set by a ULR).')
      } else if (msg.includes('not connected')) {
        toast.error('CLR not sent', `The subscriber's MME is not currently connected to this HSS as a Diameter peer.`)
      } else {
        toast.error('CLR failed', msg)
      }
    }
    finally { setClrRunning(false) }
  }

  async function doRollback(log) {
    setRollbackRunning(true)
    try {
      await rollbackOperation(log.id)
      toast.success('Rolled back', `Operation #${log.id} on ${log.table_name} reversed`)
      setRollbackConfirm(null)
      fetchLogs()
    } catch (err) { toast.error('Rollback failed', err.message) }
    finally { setRollbackRunning(false) }
  }

  if (loading && !version) {
    return <div className="loading-center"><Spinner size="lg" /><span>Loading OAM...</span></div>
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <div className="page-title">OAM</div>
          <div className="page-subtitle">Operations, administration, and maintenance</div>
        </div>
        <button className="btn btn-ghost" onClick={() => { refresh(); fetchHealth(); fetchPeers() }}>
          <RefreshCw size={14} /> Refresh
        </button>
      </div>

      {/* System Identity */}
      <div className="oam-section">
        <div className="flex items-center gap-8 mb-16">
          <Shield size={16} style={{ color: 'var(--accent)' }} />
          <h3 className="card-title">System Identity</h3>
        </div>
        {versionError && !version ? (
          <div className="error-state" style={{ padding: '20px 0' }}>
            <XCircle size={20} className="error-icon" /><div>{versionError}</div>
          </div>
        ) : (
          <div className="detail-grid">
            <div className="detail-row"><span className="detail-label">Application</span><span className="detail-value mono">{version?.app_name || 'VectorCore HSS'}</span></div>
            <div className="detail-row"><span className="detail-label">App Version</span><span className="detail-value mono">{version?.app_version || '—'}</span></div>
            <div className="detail-row"><span className="detail-label">API Version</span><span className="detail-value mono">{version?.api_version || '—'}</span></div>
          </div>
        )}
      </div>

      {/* Health */}
      <div className="oam-section">
        <div className="flex items-center gap-8 mb-16">
          <Activity size={16} style={{ color: healthError ? 'var(--danger)' : 'var(--success)' }} />
          <h3 className="card-title">Health</h3>
          <button className="btn-icon btn-sm" onClick={fetchHealth} title="Refresh health"><RefreshCw size={12} /></button>
        </div>
        <div className="flex items-center gap-12">
          {healthError ? (
            <><XCircle size={20} style={{ color: 'var(--danger)' }} /><div><div style={{ fontWeight: 600, color: 'var(--danger)' }}>UNHEALTHY</div><div className="text-muted text-sm">{healthError}</div></div></>
          ) : health ? (
            <><CheckCircle size={20} style={{ color: 'var(--success)' }} /><div><div style={{ fontWeight: 600, color: 'var(--success)' }}>{health.status?.toUpperCase() || 'OK'}</div></div></>
          ) : (
            <div className="flex items-center gap-8 text-muted text-sm"><Spinner size="sm" /> Checking...</div>
          )}
        </div>
      </div>

      {/* Connected Diameter Peers */}
      <div className="oam-section">
        <div className="flex items-center gap-8 mb-16">
          <Wifi size={16} style={{ color: 'var(--accent)' }} />
          <h3 className="card-title">Connected Diameter Peers</h3>
          <button className="btn-icon btn-sm" onClick={fetchPeers} title="Refresh peers"><RefreshCw size={12} /></button>
        </div>
        {peers.length === 0 ? (
          <div className="text-muted text-sm">No Diameter peers currently connected.</div>
        ) : (
          <div className="table-container">
            <table>
              <thead><tr><th>Origin Host</th><th>Origin Realm</th><th>Remote Address</th><th>Transport</th></tr></thead>
              <tbody>
                {peers.map((peer, i) => (
                  <tr key={i}>
                    <td className="mono" style={{ fontSize: '0.8rem' }}>{peer.origin_host || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{peer.origin_realm || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.8rem' }}>{peer.remote_addr || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.78rem', color: 'var(--accent)' }}>{peer.transport || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Send Cancel Location Request */}
      <div className="oam-section">
        <div className="flex items-center gap-8 mb-16">
          <Radio size={16} style={{ color: 'var(--warning)' }} />
          <h3 className="card-title">Send Cancel Location Request (CLR)</h3>
        </div>
        <div style={{ fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: 12 }}>
          Forces a UE to detach by sending a Diameter CLR to the serving MME. Use for subscriber disable, network changes, or roaming UE management.
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'flex-end' }}>
          <div className="form-group" style={{ margin: 0, minWidth: 240 }}>
            <label className="form-label" style={{ fontSize: '0.75rem' }}>Subscriber (known IMSI)</label>
            <select className="select" value={clrImsi} onChange={e => { setClrImsi(e.target.value); setClrCustom('') }}>
              <option value="">— Select subscriber —</option>
              {subscribers.map(s => (
                <option key={s.subscriber_id} value={s.imsi}>{s.imsi}{s.msisdn ? ` (${s.msisdn})` : ''}</option>
              ))}
            </select>
          </div>
          <div style={{ fontSize: '0.78rem', color: 'var(--text-muted)', alignSelf: 'center', paddingBottom: 4 }}>or</div>
          <div className="form-group" style={{ margin: 0, minWidth: 200 }}>
            <label className="form-label" style={{ fontSize: '0.75rem' }}>Manual IMSI (roaming UE)</label>
            <input className="input mono" value={clrCustom} onChange={e => { setClrCustom(e.target.value); setClrImsi('') }} placeholder="001010000000001" maxLength={15} />
          </div>
          <button className="btn btn-primary" onClick={doSendCLR} disabled={clrRunning || (!clrImsi && !clrCustom.trim())} style={{ flexShrink: 0 }}>
            {clrRunning ? <Spinner size="sm" /> : <Radio size={13} />} Send CLR
          </button>
        </div>
      </div>

      {/* Active Emergency Sessions */}
      <div className="oam-section">
        <div className="flex items-center gap-8 mb-16">
          <AlertTriangle size={16} style={{ color: 'var(--danger)' }} />
          <h3 className="card-title">Active Emergency Sessions</h3>
          <button className="btn-icon btn-sm" onClick={fetchEmergency} title="Refresh"><RefreshCw size={12} /></button>
        </div>
        {emergencySessions.length === 0 ? (
          <div className="text-muted text-sm">No active emergency sessions.</div>
        ) : (
          <div className="table-container">
            <table>
              <thead><tr><th>IMSI</th><th>IMEI</th><th>MNC</th><th>MCC</th><th>APN</th><th>IP</th><th>Timestamp</th></tr></thead>
              <tbody>
                {emergencySessions.map((e, i) => (
                  <tr key={i}>
                    <td className="mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>{e.imsi || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.78rem' }}>{e.imei || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{e.mnc || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{e.mcc || '—'}</td>
                    <td style={{ fontSize: '0.78rem' }}>{e.apn || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.78rem' }}>{e.serving_pgw_ip || e.ue_ip || '—'}</td>
                    <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{formatTs(e.serving_pgw_timestamp || e.timestamp)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Operation Log */}
      <div className="oam-section">
        <div className="flex items-center gap-8 mb-16">
          <RotateCcw size={16} style={{ color: 'var(--accent)' }} />
          <h3 className="card-title">Operation Log</h3>
        </div>
        <div className="text-muted text-sm" style={{ marginBottom: 10 }}>
          Audit log of all provisioning changes with rollback support.
        </div>
        <button className="btn btn-ghost" onClick={openLogs} disabled={logsLoading}>
          {logsLoading ? <Spinner size="sm" /> : <RotateCcw size={14} />} View Operation Log
        </button>
      </div>

      {/* Database Backup & Restore */}
      <div className="oam-section">
        <div className="flex items-center gap-8 mb-16">
          <HardDrive size={16} style={{ color: 'var(--accent)' }} />
          <h3 className="card-title">Database Backup &amp; Restore</h3>
        </div>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center' }}>
          <button className="btn btn-ghost" onClick={doBackup} disabled={backupRunning}>
            {backupRunning ? <Spinner size="sm" /> : <Download size={14} />} Backup Database
          </button>
          <input ref={restoreInputRef} type="file" accept=".zip,.sql,.db,.sqlite,.json" style={{ display: 'none' }} onChange={onRestoreFileChange} />
          <button className="btn btn-ghost" onClick={() => restoreInputRef.current && restoreInputRef.current.click()} disabled={restoreRunning}>
            {restoreRunning ? <Spinner size="sm" /> : <Upload size={14} />} Restore Database
          </button>
        </div>
        <div style={{ marginTop: 8, fontSize: '0.75rem', color: 'var(--text-muted)' }}>
          Backup exports all provisioning data as JSON. Restore will overwrite all current data — use with caution.
        </div>
      </div>

      {/* Operation Log modal */}
      {logsModalOpen && (
        <Modal title="Operation Log" onClose={() => setLogsModalOpen(false)} size="xl">
          <OperationLogTable
            logs={operationLogs}
            loading={logsLoading}
            onRefresh={fetchLogs}
            onRollback={setRollbackConfirm}
            onViewChanges={setViewChanges}
          />
        </Modal>
      )}

      {/* Rollback confirm */}
      {rollbackConfirm && (
        <Modal title="Confirm Rollback" onClose={() => setRollbackConfirm(null)}>
          <div className="modal-body">
            <p>Roll back operation <strong>#{rollbackConfirm.id}</strong> ({rollbackConfirm.operation} on <span className="mono">{rollbackConfirm.table_name}</span>, item #{rollbackConfirm.item_id})?</p>
            <p style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>This will attempt to restore the previous state. The action cannot itself be rolled back.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setRollbackConfirm(null)}>Cancel</button>
            <button className="btn btn-danger" onClick={() => doRollback(rollbackConfirm)} disabled={rollbackRunning}>
              {rollbackRunning ? <Spinner size="sm" /> : 'Rollback'}
            </button>
          </div>
        </Modal>
      )}

      {/* Changes detail modal */}
      {viewChanges && <ChangesModal log={viewChanges} onClose={() => setViewChanges(null)} />}
    </div>
  )
}
