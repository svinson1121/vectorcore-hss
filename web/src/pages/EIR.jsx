import React, { useState, useCallback, useEffect, useRef } from 'react'
import { Plus, Pencil, Trash2, Search, RefreshCw, ShieldCheck, Upload, Download, Database, X, ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import { useSort } from '../hooks/useSort.js'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getEIRs, createEIR, updateEIR, deleteEIR, getTACCount, lookupIMEI, createTAC, updateTAC, deleteTAC } from '../api/client.js'

async function handleTACImport(file) {
  const csv = await file.text()
  const res = await fetch('/api/v1/eir/tac/import', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ csv }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

const RESPONSE_LABELS = { 0: 'Whitelist', 1: 'Blacklist', 2: 'Greylist' }
const RESPONSE_COLORS = { 0: 'var(--success)', 1: 'var(--danger)', 2: 'var(--warning)' }

const EMPTY_FORM = { imei: '', imsi: '', regex_mode: '1', match_response_code: '0' }

function EIRModal({ row, onClose, onSaved }) {
  const toast = useToast()
  const isEdit = !!row
  const [form, setForm] = useState(row ? {
    imei: row.imei || '', imsi: row.imsi || '',
    regex_mode: String(row.regex_mode ?? 1),
    match_response_code: String(row.match_response_code ?? 0),
  } : { ...EMPTY_FORM })
  const [saving, setSaving] = useState(false)

  function set(k, v) { setForm(prev => ({ ...prev, [k]: v })) }

  async function handleSubmit(e) {
    e.preventDefault()
    if (!form.imei.trim() && !form.imsi.trim()) {
      toast.error('Validation', 'At least one of IMEI or IMSI is required'); return
    }
    setSaving(true)
    try {
      const payload = {
        ...(form.imei && { imei: form.imei }),
        ...(form.imsi && { imsi: form.imsi }),
        regex_mode: parseInt(form.regex_mode, 10),
        match_response_code: parseInt(form.match_response_code, 10),
      }
      if (isEdit) { await updateEIR(row.eir_id, payload); toast.success('Updated', 'EIR rule updated') }
      else { await createEIR(payload); toast.success('Created', 'EIR rule created') }
      onSaved(); onClose()
    } catch (err) { toast.error('Save failed', err.message) }
    finally { setSaving(false) }
  }

  return (
    <Modal title={isEdit ? 'Edit EIR Rule' : 'Add EIR Rule'} onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">IMEI / Pattern</label>
            <input className="input mono" value={form.imei} onChange={e => set('imei', e.target.value)} placeholder="35xxxxxxxxxxxxxx or regex" />
          </div>
          <div className="form-group">
            <label className="form-label">IMSI / Pattern</label>
            <input className="input mono" value={form.imsi} onChange={e => set('imsi', e.target.value)} placeholder="001010000000001 or regex" />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Match Mode</label>
              <select className="select" value={form.regex_mode} onChange={e => set('regex_mode', e.target.value)}>
                <option value="0">Exact match</option>
                <option value="1">Regex</option>
              </select>
            </div>
            <div className="form-group">
              <label className="form-label">Action on match</label>
              <select className="select" value={form.match_response_code} onChange={e => set('match_response_code', e.target.value)}>
                <option value="0">Whitelist (Allow)</option>
                <option value="1">Blacklist (Block)</option>
                <option value="2">Greylist</option>
              </select>
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

function TACLookup({ toast }) {
  const [imei, setImei] = useState('')
  const [result, setResult] = useState(null)
  const [searching, setSearching] = useState(false)
  const [editModal, setEditModal] = useState(null)
  const [delConfirm, setDelConfirm] = useState(false)
  const [saving, setSaving] = useState(false)

  async function doSearch(e) {
    e.preventDefault()
    if (!imei.trim()) return
    setSearching(true); setResult(null)
    try {
      const r = await lookupIMEI(imei.trim())
      setResult(r)
    } catch (err) {
      toast.error('Lookup failed', err.message)
    } finally { setSearching(false) }
  }

  async function handleDelete() {
    if (!result) return
    setSaving(true)
    try {
      await deleteTAC(result.tac)
      toast.success('Deleted', `TAC ${result.tac} removed`)
      setResult(null); setDelConfirm(false)
    } catch (err) { toast.error('Delete failed', err.message) }
    finally { setSaving(false) }
  }

  return (
    <div style={{ marginTop: 20 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
        <Search size={16} style={{ color: 'var(--accent)' }} />
        <h3 style={{ margin: 0, fontSize: '0.9rem', fontWeight: 600 }}>TAC Lookup / Edit</h3>
      </div>
      <form onSubmit={doSearch} style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
        <input
          className="input mono"
          style={{ maxWidth: 280 }}
          value={imei}
          onChange={e => setImei(e.target.value)}
          placeholder="Enter IMEI (15 digits)"
          maxLength={15}
        />
        <button className="btn btn-ghost" type="submit" disabled={searching || !imei.trim()}>
          {searching ? <Spinner size="sm" /> : <Search size={13} />} Lookup
        </button>
      </form>

      {result && (
        <div style={{ padding: '12px 16px', background: 'var(--bg-elevated)', border: '1px solid var(--border-subtle)', borderRadius: 'var(--radius-sm)', display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
          <div>
            <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginBottom: 2 }}>TAC</div>
            <div className="mono" style={{ fontWeight: 600 }}>{result.tac}</div>
          </div>
          <div>
            <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginBottom: 2 }}>Make</div>
            <div style={{ fontWeight: 500 }}>{result.make || '—'}</div>
          </div>
          <div>
            <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginBottom: 2 }}>Model</div>
            <div style={{ fontWeight: 500 }}>{result.model || '—'}</div>
          </div>
          <div style={{ marginLeft: 'auto', display: 'flex', gap: 6 }}>
            <button className="btn btn-ghost" style={{ fontSize: '0.8rem' }} onClick={() => setEditModal({ ...result })}>
              <Pencil size={13} /> Edit
            </button>
            <button className="btn btn-danger" style={{ fontSize: '0.8rem' }} onClick={() => setDelConfirm(true)}>
              <Trash2 size={13} /> Delete
            </button>
          </div>
        </div>
      )}

      {/* Edit TAC modal */}
      {editModal && (
        <TACEditModal
          entry={editModal}
          onClose={() => setEditModal(null)}
          onSaved={() => { setEditModal(null); setResult(null); setImei('') }}
          toast={toast}
        />
      )}

      {/* Delete confirm */}
      {delConfirm && (
        <Modal title="Delete TAC Entry" onClose={() => setDelConfirm(false)}>
          <div className="modal-body">
            <p>Delete TAC <strong className="mono">{result?.tac}</strong> ({result?.make} {result?.model})? This cannot be undone.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setDelConfirm(false)}>Cancel</button>
            <button className="btn btn-danger" onClick={handleDelete} disabled={saving}>
              {saving ? <Spinner size="sm" /> : 'Delete'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}

function TACEditModal({ entry, onClose, onSaved, toast }) {
  const [form, setForm] = useState({ make: entry.make || '', model: entry.model || '' })
  const [saving, setSaving] = useState(false)

  async function handleSubmit(e) {
    e.preventDefault()
    setSaving(true)
    try {
      await updateTAC(entry.tac, { tac: entry.tac, make: form.make, model: form.model })
      toast.success('Updated', `TAC ${entry.tac} updated`)
      onSaved()
    } catch (err) { toast.error('Update failed', err.message) }
    finally { setSaving(false) }
  }

  return (
    <Modal title={`Edit TAC ${entry.tac}`} onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">TAC</label>
            <input className="input mono" value={entry.tac} disabled />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Make</label>
              <input className="input" value={form.make} onChange={e => setForm(p => ({ ...p, make: e.target.value }))} placeholder="Samsung" />
            </div>
            <div className="form-group">
              <label className="form-label">Model</label>
              <input className="input" value={form.model} onChange={e => setForm(p => ({ ...p, model: e.target.value }))} placeholder="Galaxy S24" />
            </div>
          </div>
        </div>
        <div className="modal-footer">
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={saving}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? <Spinner size="sm" /> : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

function TACAddModal({ onClose, onSaved, toast }) {
  const [form, setForm] = useState({ tac: '', make: '', model: '' })
  const [saving, setSaving] = useState(false)

  async function handleSubmit(e) {
    e.preventDefault()
    if (!form.tac.trim()) { toast.error('Validation', 'TAC is required'); return }
    setSaving(true)
    try {
      await createTAC({ tac: form.tac.trim(), make: form.make, model: form.model })
      toast.success('Created', `TAC ${form.tac.trim()} added`)
      onSaved()
      onClose()
    } catch (err) { toast.error('Create failed', err.message) }
    finally { setSaving(false) }
  }

  return (
    <Modal title="Add TAC Entry" onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">TAC <span style={{ color: 'var(--danger)' }}>*</span></label>
            <input className="input mono" value={form.tac} onChange={e => setForm(p => ({ ...p, tac: e.target.value }))} placeholder="35674108" maxLength={8} required />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Make</label>
              <input className="input" value={form.make} onChange={e => setForm(p => ({ ...p, make: e.target.value }))} placeholder="Samsung" />
            </div>
            <div className="form-group">
              <label className="form-label">Model</label>
              <input className="input" value={form.model} onChange={e => setForm(p => ({ ...p, model: e.target.value }))} placeholder="Galaxy S24" />
            </div>
          </div>
        </div>
        <div className="modal-footer">
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={saving}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? <Spinner size="sm" /> : 'Create'}
          </button>
        </div>
      </form>
    </Modal>
  )
}

export default function EIRPage() {
  const toast = useToast()
  const fetchFn = useCallback(getEIRs, [])
  const { data, error, loading, refresh } = usePoller(fetchFn, 15000)

  const [search, setSearch] = useState('')
  const [modal, setModal] = useState(null)
  const [delConfirm, setDelConfirm] = useState(null)
  const [deleting, setDeleting] = useState(null)
  const [tacCount, setTacCount] = useState(null)
  const [tacImporting, setTacImporting] = useState(false)
  const [tacAddModal, setTacAddModal] = useState(false)
  const fileInputRef = useRef(null)

  useEffect(() => {
    getTACCount().then(n => setTacCount(n)).catch(() => {})
  }, [])

  async function onTACFileChange(e) {
    const file = e.target.files && e.target.files[0]
    if (!file) return
    setTacImporting(true)
    try {
      await handleTACImport(file)
      toast.success('TAC Import', 'TAC database imported successfully')
      getTACCount().then(n => setTacCount(n)).catch(() => {})
    } catch (err) { toast.error('TAC Import failed', err.message) }
    finally { setTacImporting(false); if (fileInputRef.current) fileInputRef.current.value = '' }
  }

  const rows = Array.isArray(data) ? data : []
  const filtered = rows.filter(r => {
    if (!search) return true
    const q = search.toLowerCase()
    return (r.imei || '').toLowerCase().includes(q) || (r.imsi || '').toLowerCase().includes(q)
  })
  const { sorted, sortKey, sortDir, handleSort } = useSort(filtered, 'imei')

  function SortIcon({ col }) {
    if (sortKey !== col) return <span className="sort-icon"><ChevronsUpDown size={11} /></span>
    return <span className="sort-icon">{sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />}</span>
  }

  async function handleDelete(row) {
    setDeleting(row.eir_id)
    try {
      await deleteEIR(row.eir_id)
      toast.success('Deleted', 'EIR rule deleted')
      setDelConfirm(null); refresh()
    } catch (err) { toast.error('Delete failed', err.message) }
    finally { setDeleting(null) }
  }

  if (loading) return <div className="loading-center"><Spinner size="lg" /><span>Loading EIR...</span></div>
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
          <div className="page-title">EIR — Equipment Identity Register</div>
          <div className="page-subtitle">IMEI/IMSI whitelist, blacklist, and greylist rules — {rows.length} total</div>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
          <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add Rule</button>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <div style={{ position: 'relative', flex: 1, maxWidth: 340 }}>
          <Search size={14} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
          <input className="input" style={{ paddingLeft: 32 }} placeholder="Search by IMEI or IMSI..."
            value={search} onChange={e => setSearch(e.target.value)} />
        </div>
      </div>

      {filtered.length === 0 ? (
        <div className="empty-state">
          <div style={{ marginBottom: 8 }}><ShieldCheck size={32} style={{ color: 'var(--text-muted)' }} /></div>
          <div>{search ? 'No results match your search.' : 'No EIR rules configured.'}</div>
          {!search && <button className="btn btn-primary" style={{ marginTop: 12 }} onClick={() => setModal('add')}><Plus size={14} /> Add Rule</button>}
        </div>
      ) : (
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th className={`sortable${sortKey === 'imei' ? ' sort-active' : ''}`} onClick={() => handleSort('imei')}>IMEI / Pattern<SortIcon col="imei" /></th>
                <th className={`sortable${sortKey === 'imsi' ? ' sort-active' : ''}`} onClick={() => handleSort('imsi')}>IMSI / Pattern<SortIcon col="imsi" /></th>
                <th className={`sortable${sortKey === 'regex_mode' ? ' sort-active' : ''}`} onClick={() => handleSort('regex_mode')}>Mode<SortIcon col="regex_mode" /></th>
                <th className={`sortable${sortKey === 'match_response_code' ? ' sort-active' : ''}`} onClick={() => handleSort('match_response_code')}>Action<SortIcon col="match_response_code" /></th>
                <th className={`sortable${sortKey === 'last_modified' ? ' sort-active' : ''}`} onClick={() => handleSort('last_modified')}>Last Modified<SortIcon col="last_modified" /></th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {sorted.map(row => {
                const code = row.match_response_code ?? 0
                return (
                  <tr key={row.eir_id}>
                    <td className="mono">{row.imei || '—'}</td>
                    <td className="mono">{row.imsi || '—'}</td>
                    <td style={{ color: 'var(--text-muted)', fontSize: '0.78rem' }}>{row.regex_mode === 1 ? 'Regex' : 'Exact'}</td>
                    <td><span style={{ color: RESPONSE_COLORS[code], fontWeight: 600, fontSize: '0.78rem' }}>{RESPONSE_LABELS[code] ?? code}</span></td>
                    <td style={{ color: 'var(--text-muted)', fontSize: '0.78rem' }}>{row.last_modified ? new Date(row.last_modified).toLocaleString() : '—'}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn-icon" title="Edit" onClick={() => setModal({ row })}><Pencil size={13} /></button>
                        <button className="btn-icon danger" title="Delete" onClick={() => setDelConfirm(row)}><Trash2 size={13} /></button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {modal === 'add' && <EIRModal onClose={() => setModal(null)} onSaved={refresh} />}
      {modal && modal.row && <EIRModal row={modal.row} onClose={() => setModal(null)} onSaved={refresh} />}

      {delConfirm && (
        <Modal title="Delete EIR Rule" onClose={() => setDelConfirm(null)}>
          <div className="modal-body">
            <p>Delete EIR rule for IMEI <strong>{delConfirm.imei || '(any)'}</strong> / IMSI <strong>{delConfirm.imsi || '(any)'}</strong>? This cannot be undone.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setDelConfirm(null)}>Cancel</button>
            <button className="btn btn-danger" onClick={() => handleDelete(delConfirm)} disabled={deleting === delConfirm.eir_id}>
              {deleting === delConfirm.eir_id ? <Spinner size="sm" /> : 'Delete'}
            </button>
          </div>
        </Modal>
      )}

      {/* TAC Device Database */}
      <div style={{ marginTop: 32, padding: '20px 24px', background: 'var(--card-bg)', border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
          <Database size={16} style={{ color: 'var(--accent)' }} />
          <h3 style={{ margin: 0, fontSize: '0.9rem', fontWeight: 600 }}>TAC Device Database</h3>
        </div>
        <div style={{ fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: 16 }}>
          Device make/model lookup for IMEI enrichment.
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <input ref={fileInputRef} type="file" accept=".csv" style={{ display: 'none' }} onChange={onTACFileChange} />
          <button className="btn btn-primary" onClick={() => setTacAddModal(true)}>
            <Plus size={14} /> Add Entry
          </button>
          <button className="btn btn-ghost" onClick={() => fileInputRef.current && fileInputRef.current.click()} disabled={tacImporting}>
            {tacImporting ? <Spinner size="sm" /> : <Upload size={14} />} Import CSV
          </button>
          <a href="/api/v1/eir/tac/export" download="tac_export.csv" className="btn btn-ghost" style={{ textDecoration: 'none' }}>
            <Download size={14} /> Export CSV
          </a>
        </div>

        <TACLookup toast={toast} onAdded={() => getTACCount().then(n => setTacCount(n)).catch(() => {})} />
      </div>

      {tacAddModal && (
        <TACAddModal
          onClose={() => setTacAddModal(false)}
          onSaved={() => { setTacAddModal(false); getTACCount().then(n => setTacCount(n)).catch(() => {}) }}
          toast={toast}
        />
      )}
    </div>
  )
}
