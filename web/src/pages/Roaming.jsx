import React, { useState, useCallback } from 'react'
import { Plus, Pencil, Trash2, Search, RefreshCw, Globe2, ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import { useSort } from '../hooks/useSort.js'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getRoamingRules, createRoamingRule, updateRoamingRule, deleteRoamingRule } from '../api/client.js'

const EMPTY_FORM = {
  name: '',
  mcc: '',
  mnc: '',
  allow: true,
  enabled: true,
}

function RoamingModal({ row, onClose, onSaved }) {
  const toast = useToast()
  const isEdit = !!row
  const [form, setForm] = useState(row ? {
    name: row.name || '',
    mcc: row.mcc || '',
    mnc: row.mnc || '',
    allow: row.allow !== false,
    enabled: row.enabled !== false,
  } : { ...EMPTY_FORM })
  const [saving, setSaving] = useState(false)

  function set(k, v) {
    setForm(prev => ({ ...prev, [k]: v }))
  }

  async function handleSubmit(e) {
    e.preventDefault()
    setSaving(true)
    try {
      const payload = {
        ...(form.name && { name: form.name }),
        ...(form.mcc && { mcc: form.mcc }),
        ...(form.mnc && { mnc: form.mnc }),
        allow: form.allow,
        enabled: form.enabled,
      }
      if (isEdit) {
        await updateRoamingRule(row.roaming_rule_id, payload)
        toast.success('Updated', 'Roaming rule updated')
      } else {
        await createRoamingRule(payload)
        toast.success('Created', 'Roaming rule created')
      }
      onSaved()
      onClose()
    } catch (err) {
      toast.error('Save failed', err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal title={isEdit ? 'Edit Roaming Rule' : 'Add Roaming Rule'} onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">Name</label>
            <input
              className="input"
              value={form.name}
              onChange={e => set('name', e.target.value)}
              placeholder="Roaming partner name"
            />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">MCC</label>
              <input
                className="input mono"
                value={form.mcc}
                onChange={e => set('mcc', e.target.value)}
                placeholder="001"
                maxLength={3}
              />
            </div>
            <div className="form-group">
              <label className="form-label">MNC</label>
              <input
                className="input mono"
                value={form.mnc}
                onChange={e => set('mnc', e.target.value)}
                placeholder="01"
                maxLength={3}
              />
            </div>
          </div>
          <div className="form-row">
            <label className="checkbox-wrap">
              <input type="checkbox" checked={form.allow} onChange={e => set('allow', e.target.checked)} />
              <span className="form-label" style={{ margin: 0 }}>Allow Roaming</span>
            </label>
            <label className="checkbox-wrap">
              <input type="checkbox" checked={form.enabled} onChange={e => set('enabled', e.target.checked)} />
              <span className="form-label" style={{ margin: 0 }}>Enabled</span>
            </label>
          </div>
        </div>
        <div className="modal-footer">
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={saving}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? <Spinner size="sm" /> : null}
            {isEdit ? 'Save' : 'Create'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

export default function Roaming() {
  const toast = useToast()
  const fetchFn = useCallback(getRoamingRules, [])
  const { data, error, loading, refresh } = usePoller(fetchFn, 15000)

  const [search, setSearch] = useState('')
  const [modal, setModal] = useState(null)
  const [delConfirm, setDelConfirm] = useState(null)
  const [deleting, setDeleting] = useState(null)

  const rows = Array.isArray(data) ? data : []

  const filtered = rows.filter(r => {
    if (!search) return true
    const q = search.toLowerCase()
    return (
      (r.name || '').toLowerCase().includes(q) ||
      (r.mcc || '').includes(q) ||
      (r.mnc || '').includes(q)
    )
  })
  const { sorted, sortKey, sortDir, handleSort } = useSort(filtered, 'name')

  function SortIcon({ col }) {
    if (sortKey !== col) return <span className="sort-icon"><ChevronsUpDown size={11} /></span>
    return <span className="sort-icon">{sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />}</span>
  }

  async function handleDelete(row) {
    setDeleting(row.roaming_rule_id)
    try {
      await deleteRoamingRule(row.roaming_rule_id)
      toast.success('Deleted', 'Roaming rule deleted')
      setDelConfirm(null)
      refresh()
    } catch (err) {
      toast.error('Delete failed', err.message)
    } finally {
      setDeleting(null)
    }
  }

  if (loading) return (
    <div className="loading-center"><Spinner size="lg" /><span>Loading roaming rules...</span></div>
  )
  if (error && rows.length === 0) return (
    <div className="error-state">
      <div>{error}</div>
      <button className="btn btn-ghost mt-12" onClick={refresh}><RefreshCw size={14} /> Retry</button>
    </div>
  )

  return (
    <div>
      <div className="page-header">
        <div>
          <div className="page-title">Roaming Rules</div>
          <div className="page-subtitle">Per-PLMN roaming allow/block rules — {rows.length} total</div>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
          <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add Rule</button>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <div style={{ position: 'relative', flex: 1, maxWidth: 340 }}>
          <Search size={14} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
          <input
            className="input"
            style={{ paddingLeft: 32 }}
            placeholder="Search by name, MCC, or MNC..."
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
        </div>
      </div>

      {filtered.length === 0 ? (
        <div className="empty-state">
          <div style={{ marginBottom: 8 }}><Globe2 size={32} style={{ color: 'var(--text-muted)' }} /></div>
          <div>{search ? 'No results match your search.' : 'No roaming rules configured.'}</div>
          {!search && (
            <button className="btn btn-primary" style={{ marginTop: 12 }} onClick={() => setModal('add')}>
              <Plus size={14} /> Add Rule
            </button>
          )}
        </div>
      ) : (
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th className={`sortable${sortKey === 'name' ? ' sort-active' : ''}`} onClick={() => handleSort('name')}>Name<SortIcon col="name" /></th>
                <th className={`sortable${sortKey === 'mcc' ? ' sort-active' : ''}`} onClick={() => handleSort('mcc')}>MCC<SortIcon col="mcc" /></th>
                <th className={`sortable${sortKey === 'mnc' ? ' sort-active' : ''}`} onClick={() => handleSort('mnc')}>MNC<SortIcon col="mnc" /></th>
                <th className={`sortable${sortKey === 'plmn' ? ' sort-active' : ''}`} onClick={() => handleSort('plmn')}>PLMN<SortIcon col="plmn" /></th>
                <th className={`sortable${sortKey === 'allow' ? ' sort-active' : ''}`} onClick={() => handleSort('allow')}>Action<SortIcon col="allow" /></th>
                <th className={`sortable${sortKey === 'enabled' ? ' sort-active' : ''}`} onClick={() => handleSort('enabled')}>Enabled<SortIcon col="enabled" /></th>
                <th className={`sortable${sortKey === 'last_modified' ? ' sort-active' : ''}`} onClick={() => handleSort('last_modified')}>Last Modified<SortIcon col="last_modified" /></th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {sorted.map(row => {
                const allowed = row.allow !== false
                const enabled = row.enabled !== false
                return (
                  <tr key={row.roaming_rule_id}>
                    <td style={{ fontWeight: 600 }}>{row.name || '—'}</td>
                    <td className="mono">{row.mcc || '—'}</td>
                    <td className="mono">{row.mnc || '—'}</td>
                    <td className="mono" style={{ color: 'var(--text-muted)', fontSize: '0.78rem' }}>
                      {row.mcc && row.mnc ? `${row.mcc}${row.mnc}` : '—'}
                    </td>
                    <td>
                      <span style={{ color: allowed ? 'var(--success)' : 'var(--danger)', fontWeight: 600, fontSize: '0.78rem' }}>
                        {allowed ? 'Allow' : 'Block'}
                      </span>
                    </td>
                    <td>
                      <span style={{ color: enabled ? 'var(--success)' : 'var(--text-muted)', fontSize: '0.78rem' }}>
                        {enabled ? 'Yes' : 'No'}
                      </span>
                    </td>
                    <td style={{ color: 'var(--text-muted)', fontSize: '0.78rem' }}>
                      {row.last_modified ? new Date(row.last_modified).toLocaleString() : '—'}
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn-icon" title="Edit" onClick={() => setModal({ row })}>
                          <Pencil size={13} />
                        </button>
                        <button className="btn-icon danger" title="Delete" onClick={() => setDelConfirm(row)}>
                          <Trash2 size={13} />
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {modal === 'add' && (
        <RoamingModal onClose={() => setModal(null)} onSaved={refresh} />
      )}
      {modal && modal.row && (
        <RoamingModal row={modal.row} onClose={() => setModal(null)} onSaved={refresh} />
      )}

      {delConfirm && (
        <Modal title="Delete Roaming Rule" onClose={() => setDelConfirm(null)}>
          <div className="modal-body">
            <p>Delete roaming rule <strong>{delConfirm.name || `MCC=${delConfirm.mcc} MNC=${delConfirm.mnc}`}</strong>? This cannot be undone.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setDelConfirm(null)}>Cancel</button>
            <button
              className="btn btn-danger"
              onClick={() => handleDelete(delConfirm)}
              disabled={deleting === delConfirm.roaming_rule_id}
            >
              {deleting === delConfirm.roaming_rule_id ? <Spinner size="sm" /> : 'Delete'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
