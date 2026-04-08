import React, { useState, useCallback, useEffect } from 'react'
import { Plus, Pencil, Trash2, RefreshCw, GitBranch, ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import { useSort } from '../hooks/useSort.js'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getSubscriberRoutings, createSubscriberRouting, updateSubscriberRouting, deleteSubscriberRouting, getSubscribers, getAPNs } from '../api/client.js'

const IP_VERSION_LABELS = { 0: 'IPv4', 1: 'IPv6', 2: 'IPv4v6', 3: 'IPv4 or v6' }

function RoutingModal({ routing, onClose, onSaved, subscribers, apns }) {
  const toast = useToast()
  const isEdit = !!routing
  const [form, setForm] = useState(isEdit ? {
    subscriber_id: String(routing.subscriber_id ?? ''),
    apn_id: String(routing.apn_id ?? ''),
    ip_version: routing.ip_version ?? 0,
    ip_address: routing.ip_address || '',
  } : { subscriber_id: '', apn_id: '', ip_version: 0, ip_address: '' })
  const [saving, setSaving] = useState(false)

  function set(k, v) { setForm(p => ({ ...p, [k]: v })) }

  async function handleSubmit(e) {
    e.preventDefault()
    if (!form.subscriber_id) { toast.error('Validation', 'Subscriber is required'); return }
    if (!form.apn_id) { toast.error('Validation', 'APN is required'); return }
    setSaving(true)
    try {
      const payload = {
        subscriber_id: Number(form.subscriber_id),
        apn_id: Number(form.apn_id),
        ip_version: Number(form.ip_version),
        ...(form.ip_address.trim() && { ip_address: form.ip_address.trim() }),
      }
      if (isEdit) {
        await updateSubscriberRouting(routing.subscriber_routing_id, payload)
        toast.success('Updated', 'Subscriber routing updated')
      } else {
        await createSubscriberRouting(payload)
        toast.success('Created', 'Subscriber routing created')
      }
      onSaved(); onClose()
    } catch (err) { toast.error('Save failed', err.message) }
    finally { setSaving(false) }
  }

  return (
    <Modal title={isEdit ? 'Edit Subscriber Routing' : 'Add Subscriber Routing'} onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">Subscriber <span style={{ color: 'var(--danger)' }}>*</span></label>
            <select className="select" value={form.subscriber_id} onChange={e => set('subscriber_id', e.target.value)} disabled={isEdit} required>
              <option value="">— Select subscriber —</option>
              {subscribers.map(s => (
                <option key={s.subscriber_id} value={String(s.subscriber_id)}>
                  {s.imsi}{s.msisdn ? ` (${s.msisdn})` : ''}
                </option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label className="form-label">APN <span style={{ color: 'var(--danger)' }}>*</span></label>
            <select className="select" value={form.apn_id} onChange={e => set('apn_id', e.target.value)} disabled={isEdit} required>
              <option value="">— Select APN —</option>
              {apns.map(a => (
                <option key={a.apn_id} value={String(a.apn_id)}>{a.apn}</option>
              ))}
            </select>
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">IP Version</label>
              <select className="select" value={form.ip_version} onChange={e => set('ip_version', e.target.value)}>
                {Object.entries(IP_VERSION_LABELS).map(([v, label]) => (
                  <option key={v} value={v}>{v} — {label}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label className="form-label">Static IP Address (optional)</label>
              <input className="input mono" value={form.ip_address} onChange={e => set('ip_address', e.target.value)} placeholder="10.0.0.1" />
            </div>
          </div>
        </div>
        <div className="modal-footer">
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={saving}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? <Spinner size="sm" /> : null}{isEdit ? 'Save' : 'Create'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

export default function SubscriberRoutings({ compact = false }) {
  const toast = useToast()
  const fetchFn = useCallback(getSubscriberRoutings, [])
  const { data, error, loading, refresh } = usePoller(fetchFn, 30000)
  const [subscribers, setSubscribers] = useState([])
  const [apns, setApns] = useState([])
  const [modal, setModal] = useState(null)
  const [delConfirm, setDelConfirm] = useState(null)
  const [deleting, setDeleting] = useState(null)

  useEffect(() => {
    getSubscribers().then(d => setSubscribers(Array.isArray(d?.items) ? d.items : [])).catch(() => {})
    getAPNs().then(d => setApns(Array.isArray(d) ? d : [])).catch(() => {})
  }, [])

  const subMap = {}; subscribers.forEach(s => { subMap[s.subscriber_id] = s.imsi })
  const apnMap = {}; apns.forEach(a => { apnMap[a.apn_id] = a.apn })

  const rows = Array.isArray(data) ? data : []
  const enriched = rows.map(r => ({ ...r, _imsi: subMap[r.subscriber_id] || '', _apn: apnMap[r.apn_id] || '' }))
  const { sorted, sortKey, sortDir, handleSort } = useSort(enriched, '_imsi')

  function SortIcon({ col }) {
    if (sortKey !== col) return <span className="sort-icon"><ChevronsUpDown size={11} /></span>
    return <span className="sort-icon">{sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />}</span>
  }

  async function handleDelete(row) {
    setDeleting(row.subscriber_routing_id)
    try {
      await deleteSubscriberRouting(row.subscriber_routing_id)
      toast.success('Deleted', 'Subscriber routing deleted')
      setDelConfirm(null); refresh()
    } catch (err) { toast.error('Delete failed', err.message) }
    finally { setDeleting(null) }
  }

  if (loading) return <div className="loading-center"><Spinner size="lg" /><span>Loading routings...</span></div>
  if (error && rows.length === 0) return (
    <div className="error-state">
      <div>{error}</div>
      <button className="btn btn-ghost mt-12" onClick={refresh}><RefreshCw size={14} /> Retry</button>
    </div>
  )

  return (
    <div>
      {!compact && (
        <div className="page-header">
          <div>
            <div className="page-title">Subscriber Routings</div>
            <div className="page-subtitle">Static IP and per-APN routing overrides — {rows.length} total</div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
            <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add Routing</button>
          </div>
        </div>
      )}
      {compact && (
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginBottom: 8 }}>
          <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
          <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add Routing</button>
        </div>
      )}

      {rows.length === 0 ? (
        <div className="empty-state">
          <div style={{ marginBottom: 8 }}><GitBranch size={32} style={{ color: 'var(--text-muted)' }} /></div>
          <div>No subscriber routings configured.</div>
          <button className="btn btn-primary" style={{ marginTop: 12 }} onClick={() => setModal('add')}><Plus size={14} /> Add Routing</button>
        </div>
      ) : (
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th className={`sortable${sortKey === '_imsi' ? ' sort-active' : ''}`} onClick={() => handleSort('_imsi')}>IMSI<SortIcon col="_imsi" /></th>
                <th className={`sortable${sortKey === '_apn' ? ' sort-active' : ''}`} onClick={() => handleSort('_apn')}>APN<SortIcon col="_apn" /></th>
                <th className={`sortable${sortKey === 'ip_version' ? ' sort-active' : ''}`} onClick={() => handleSort('ip_version')}>IP Version<SortIcon col="ip_version" /></th>
                <th className={`sortable${sortKey === 'ip_address' ? ' sort-active' : ''}`} onClick={() => handleSort('ip_address')}>Static IP<SortIcon col="ip_address" /></th>
                <th className={`sortable${sortKey === 'last_modified' ? ' sort-active' : ''}`} onClick={() => handleSort('last_modified')}>Last Modified<SortIcon col="last_modified" /></th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {sorted.map(row => (
                <tr key={row.subscriber_routing_id}>
                  <td className="mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>{subMap[row.subscriber_id] || `sub#${row.subscriber_id}`}</td>
                  <td className="mono" style={{ fontSize: '0.82rem' }}>{apnMap[row.apn_id] || `apn#${row.apn_id}`}</td>
                  <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{IP_VERSION_LABELS[row.ip_version] ?? row.ip_version}</td>
                  <td className="mono" style={{ fontSize: '0.78rem' }}>{row.ip_address || '—'}</td>
                  <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{row.last_modified ? new Date(row.last_modified).toLocaleString() : '—'}</td>
                  <td>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button className="btn-icon" title="Edit" onClick={() => setModal({ routing: row })}><Pencil size={13} /></button>
                      <button className="btn-icon danger" title="Delete" onClick={() => setDelConfirm(row)} disabled={deleting === row.subscriber_routing_id}>
                        {deleting === row.subscriber_routing_id ? <Spinner size="sm" /> : <Trash2 size={13} />}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {modal === 'add' && <RoutingModal onClose={() => setModal(null)} onSaved={refresh} subscribers={subscribers} apns={apns} />}
      {modal?.routing && <RoutingModal routing={modal.routing} onClose={() => setModal(null)} onSaved={refresh} subscribers={subscribers} apns={apns} />}

      {delConfirm && (
        <Modal title="Delete Routing" onClose={() => setDelConfirm(null)}>
          <div className="modal-body">
            <p>Delete routing for <span className="mono">{subMap[delConfirm.subscriber_id] || `sub#${delConfirm.subscriber_id}`}</span> → <span className="mono">{apnMap[delConfirm.apn_id] || `apn#${delConfirm.apn_id}`}</span>? This cannot be undone.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setDelConfirm(null)}>Cancel</button>
            <button className="btn btn-danger" onClick={() => handleDelete(delConfirm)} disabled={deleting === delConfirm.subscriber_routing_id}>
              {deleting === delConfirm.subscriber_routing_id ? <Spinner size="sm" /> : 'Delete'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
