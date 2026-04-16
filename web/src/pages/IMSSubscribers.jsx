import React, { useState, useEffect } from 'react'
import { Plus, Pencil, Trash2, Search, RefreshCw, Phone, ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import { useSort } from '../hooks/useSort.js'
import { useInfiniteScroll } from '../hooks/useInfiniteScroll.js'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import {
  getIMSSubscribers, createIMSSubscriber, updateIMSSubscriber, deleteIMSSubscriber,
  getSubscribers, getIFCProfiles,
} from '../api/client.js'
import IFCProfiles from './IFCProfiles.jsx'

const TAB_STYLE = (active) => ({
  padding: '6px 16px',
  fontSize: '0.82rem',
  fontWeight: active ? 600 : 400,
  color: active ? 'var(--accent)' : 'var(--text-muted)',
  background: 'none',
  border: 'none',
  borderBottom: active ? '2px solid var(--accent)' : '2px solid transparent',
  cursor: 'pointer',
  whiteSpace: 'nowrap',
})

const SECTION_STYLE = {
  fontSize: '0.75rem',
  fontWeight: 600,
  textTransform: 'uppercase',
  letterSpacing: '0.08em',
  color: 'var(--text-muted)',
  marginBottom: 8,
  marginTop: 16,
  borderBottom: '1px solid var(--border-subtle)',
  paddingBottom: 4,
}

function IMSModal({ row, onClose, onSaved, subscriberList, ifcProfiles }) {
  const toast = useToast()
  const isEdit = !!row
  const [availableIfcProfiles, setAvailableIfcProfiles] = useState(ifcProfiles)
  const [loadingIfcProfiles, setLoadingIfcProfiles] = useState(true)
  const [form, setForm] = useState(isEdit ? {
    imsi: row.imsi || '',
    msisdn: row.msisdn || '',
    msisdn_list: row.msisdn_list || '',
    ifc_profile_id: row.ifc_profile_id != null ? String(row.ifc_profile_id) : '',
  } : {
    imsi: '',
    msisdn: '',
    msisdn_list: '',
    ifc_profile_id: '',
  })
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let active = true

    async function loadIfcProfiles() {
      setLoadingIfcProfiles(true)
      try {
        const data = await getIFCProfiles()
        if (active) setAvailableIfcProfiles(Array.isArray(data) ? data : [])
      } catch (err) {
        if (active) {
          setAvailableIfcProfiles(Array.isArray(ifcProfiles) ? ifcProfiles : [])
          toast.error('IFC profiles', err.message || 'Failed to load IFC profiles')
        }
      } finally {
        if (active) setLoadingIfcProfiles(false)
      }
    }

    loadIfcProfiles()
    return () => { active = false }
  }, [ifcProfiles, toast])

  function set(k, v) {
    setForm(prev => ({ ...prev, [k]: v }))
  }

  function handleImsiChange(e) {
    const imsi = e.target.value
    set('imsi', imsi)
    if (imsi) {
      const sub = subscriberList.find(s => s.imsi === imsi)
      if (sub && sub.msisdn) {
        setForm(prev => ({
          ...prev,
          msisdn: sub.msisdn,
          msisdn_list: prev.msisdn_list || sub.msisdn,
        }))
      }
    }
  }

  function handleMsisdnChange(e) {
    const v = e.target.value
    setForm(prev => ({
      ...prev,
      msisdn: v,
      msisdn_list: prev.msisdn_list || v,
    }))
  }

  async function handleSubmit(e) {
    e.preventDefault()
    if (!form.msisdn.trim()) {
      toast.error('Validation', 'MSISDN is required')
      return
    }
    if (!form.ifc_profile_id) {
      toast.error('Validation', 'IFC Profile is required')
      return
    }
    setSaving(true)
    try {
      const payload = {
        msisdn: form.msisdn,
        ...(form.msisdn_list && { msisdn_list: form.msisdn_list }),
        ...(form.imsi && { imsi: form.imsi }),
        ...(form.ifc_profile_id !== '' && { ifc_profile_id: parseInt(form.ifc_profile_id, 10) }),
      }
      if (isEdit) {
        await updateIMSSubscriber(row.ims_subscriber_id, payload)
        toast.success('Updated', `IMS subscriber ${form.msisdn} updated`)
      } else {
        await createIMSSubscriber(payload)
        toast.success('Created', `IMS subscriber ${form.msisdn} created`)
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
    <Modal title={isEdit ? 'Edit IMS Subscriber' : 'Add IMS Subscriber'} onClose={onClose} size="lg">
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}>
        <div className="modal-body">

          <div style={SECTION_STYLE}>Identity</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">IMSI</label>
              <select
                className="select"
                value={form.imsi}
                onChange={handleImsiChange}
              >
                <option value="">— Select Subscriber (optional) —</option>
                {subscriberList.map(s => (
                  <option key={s.subscriber_id} value={s.imsi}>{s.imsi} {s.msisdn ? `(${s.msisdn})` : ''}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label className="form-label">MSISDN <span style={{ color: 'var(--danger)' }}>*</span></label>
              <input
                className="input mono"
                value={form.msisdn}
                onChange={handleMsisdnChange}
                placeholder="441234567890"
                required
              />
            </div>
          </div>
          <div className="form-group">
            <label className="form-label">MSISDN List (comma-separated additional MSISDNs)</label>
            <input
              className="input mono"
              value={form.msisdn_list}
              onChange={e => set('msisdn_list', e.target.value)}
              placeholder="441234567890,441234567891"
            />
          </div>
          <div className="form-group">
            <label className="form-label">IFC Profile <span style={{ color: 'var(--danger)' }}>*</span></label>
            <select
              className="select"
              value={form.ifc_profile_id}
              onChange={e => set('ifc_profile_id', e.target.value)}
              required
              disabled={loadingIfcProfiles}
            >
              <option value="">
                {loadingIfcProfiles ? 'Loading IFC Profiles...' : '— Select IFC Profile —'}
              </option>
              {availableIfcProfiles.map(p => (
                <option key={p.ifc_profile_id} value={String(p.ifc_profile_id)}>{p.name}</option>
              ))}
            </select>
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

export default function IMSSubscribers() {
  const toast = useToast()
  const [activeTab, setActiveTab] = useState('ims')
  const [search, setSearch] = useState('')
  const [modal, setModal] = useState(null)
  const [delConfirm, setDelConfirm] = useState(null)
  const [deleting, setDeleting] = useState(null)
  const [subscriberList, setSubscriberList] = useState([])
  const [ifcProfiles, setIfcProfiles] = useState([])

  const { items, total, loading, loadingMore, error, sentinelRef, refresh } = useInfiniteScroll(getIMSSubscribers, search)

  useEffect(() => {
    getSubscribers().then(d => setSubscriberList(Array.isArray(d?.items) ? d.items : [])).catch(() => {})
    getIFCProfiles().then(d => setIfcProfiles(Array.isArray(d) ? d : [])).catch(() => {})
  }, [])

  const enriched = items.map(r => ({ ...r, _ifc_profile: getIfcName(r.ifc_profile_id) }))
  const { sorted: sortedRows, sortKey, sortDir, handleSort } = useSort(enriched, 'msisdn')

  function SortIcon({ col }) {
    if (sortKey !== col) return <span className="sort-icon"><ChevronsUpDown size={11} /></span>
    return <span className="sort-icon">{sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />}</span>
  }

  function getIfcName(id) {
    if (id == null) return '—'
    const p = ifcProfiles.find(p => p.ifc_profile_id === id)
    return p ? p.name : String(id)
  }

  async function handleDelete(row) {
    setDeleting(row.ims_subscriber_id)
    try {
      await deleteIMSSubscriber(row.ims_subscriber_id)
      toast.success('Deleted', `IMS subscriber ${row.msisdn} deleted`)
      setDelConfirm(null)
      refresh()
    } catch (err) {
      toast.error('Delete failed', err.message)
    } finally {
      setDeleting(null)
    }
  }

  if (loading) {
    return (
      <div className="loading-center">
        <Spinner size="lg" />
        <span>Loading IMS subscribers...</span>
      </div>
    )
  }

  if (error && items.length === 0) {
    return (
      <div className="error-state">
        <div>{error}</div>
        <button className="btn btn-ghost mt-12" onClick={refresh}><RefreshCw size={14} /> Retry</button>
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <div className="page-title">IMS Subscribers</div>
          <div className="page-subtitle">VoLTE/IMS subscriber profiles — {total != null ? total : items.length} total</div>
        </div>
        {activeTab === 'ims' && (
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
            <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add IMS Subscriber</button>
          </div>
        )}
      </div>

      <div style={{ display: 'flex', borderBottom: '1px solid var(--border)', marginBottom: 16 }}>
        <button style={TAB_STYLE(activeTab === 'ims')} onClick={() => setActiveTab('ims')}>IMS Subscribers</button>
        <button style={TAB_STYLE(activeTab === 'ifc')} onClick={() => setActiveTab('ifc')}>IFC Profiles</button>
      </div>

      {activeTab === 'ims' && (<>
        <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
          <div style={{ position: 'relative', flex: 1, maxWidth: 340 }}>
            <Search size={14} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
            <input className="input" style={{ paddingLeft: 32 }} placeholder="Search by MSISDN or IMSI..."
              value={search} onChange={e => setSearch(e.target.value)} />
          </div>
        </div>

        {!loading && items.length === 0 ? (
          <div className="empty-state">
            <div style={{ marginBottom: 8 }}><Phone size={32} style={{ color: 'var(--text-muted)' }} /></div>
            <div>{search ? 'No results match your search.' : 'No IMS subscribers configured.'}</div>
            {!search && <button className="btn btn-primary" style={{ marginTop: 12 }} onClick={() => setModal('add')}><Plus size={14} /> Add IMS Subscriber</button>}
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th className={`sortable${sortKey === 'msisdn' ? ' sort-active' : ''}`} onClick={() => handleSort('msisdn')}>MSISDN<SortIcon col="msisdn" /></th>
                  <th className={`sortable${sortKey === 'imsi' ? ' sort-active' : ''}`} onClick={() => handleSort('imsi')}>IMSI<SortIcon col="imsi" /></th>
                  <th className={`sortable${sortKey === '_ifc_profile' ? ' sort-active' : ''}`} onClick={() => handleSort('_ifc_profile')}>IFC Profile<SortIcon col="_ifc_profile" /></th>
                  <th className={`sortable${sortKey === 'last_modified' ? ' sort-active' : ''}`} onClick={() => handleSort('last_modified')}>Last Modified<SortIcon col="last_modified" /></th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {sortedRows.map(row => (
                  <tr key={row.ims_subscriber_id}>
                    <td className="mono" style={{ fontWeight: 600 }}>{row.msisdn}</td>
                    <td className="mono" style={{ color: 'var(--text-muted)', fontSize: '0.78rem' }}>{row.imsi || '—'}</td>
                    <td style={{ color: 'var(--text-muted)', fontSize: '0.82rem' }}>{getIfcName(row.ifc_profile_id)}</td>
                    <td style={{ color: 'var(--text-muted)', fontSize: '0.78rem' }}>
                      {row.last_modified ? new Date(row.last_modified).toLocaleString() : '—'}
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn-icon" title="Edit" onClick={() => setModal({ row })}><Pencil size={13} /></button>
                        <button className="btn-icon danger" title="Delete" onClick={() => setDelConfirm(row)}><Trash2 size={13} /></button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div ref={sentinelRef} style={{ height: 1 }} />
            {loadingMore && <div style={{ textAlign: 'center', padding: '12px 0', color: 'var(--text-muted)', fontSize: '0.8rem' }}><Spinner size="sm" /> Loading more…</div>}
          </div>
        )}

        {modal === 'add' && <IMSModal onClose={() => setModal(null)} onSaved={refresh} subscriberList={subscriberList} ifcProfiles={ifcProfiles} />}
        {modal && modal.row && <IMSModal row={modal.row} onClose={() => setModal(null)} onSaved={refresh} subscriberList={subscriberList} ifcProfiles={ifcProfiles} />}

        {delConfirm && (
          <Modal title="Delete IMS Subscriber" onClose={() => setDelConfirm(null)}>
            <div className="modal-body">
              <p>Delete IMS subscriber <strong>{delConfirm.msisdn}</strong>? This cannot be undone.</p>
            </div>
            <div className="modal-footer">
              <button className="btn btn-ghost" onClick={() => setDelConfirm(null)}>Cancel</button>
              <button className="btn btn-danger" onClick={() => handleDelete(delConfirm)} disabled={deleting === delConfirm.ims_subscriber_id}>
                {deleting === delConfirm.ims_subscriber_id ? <Spinner size="sm" /> : 'Delete'}
              </button>
            </div>
          </Modal>
        )}
      </>)}

      {activeTab === 'ifc' && <IFCProfiles compact />}
    </div>
  )
}
