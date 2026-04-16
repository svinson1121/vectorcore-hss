import React, { useState, useCallback, useEffect } from 'react'
import { Plus, Pencil, Trash2, Search, RefreshCw, Tag, ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import { useSort } from '../hooks/useSort.js'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getSubscriberAttributes, createSubscriberAttribute, updateSubscriberAttribute, deleteSubscriberAttribute, getSubscribers } from '../api/client.js'

function AttributeModal({ attr, onClose, onSaved, subscribers }) {
  const toast = useToast()
  const isEdit = !!attr
  const [availableSubscribers, setAvailableSubscribers] = useState(subscribers)
  const [loadingSubscribers, setLoadingSubscribers] = useState(true)
  const [form, setForm] = useState(isEdit ? {
    subscriber_id: String(attr.subscriber_id ?? ''),
    key: attr.key || '',
    value: attr.value || '',
  } : { subscriber_id: '', key: '', value: '' })
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let active = true

    async function loadSubscribers() {
      setLoadingSubscribers(true)
      try {
        const data = await getSubscribers()
        if (active) setAvailableSubscribers(Array.isArray(data?.items) ? data.items : [])
      } catch (err) {
        if (active) {
          setAvailableSubscribers(Array.isArray(subscribers) ? subscribers : [])
          toast.error('Subscribers', err.message || 'Failed to load subscribers')
        }
      } finally {
        if (active) setLoadingSubscribers(false)
      }
    }

    loadSubscribers()
    return () => { active = false }
  }, [subscribers, toast])

  function set(k, v) { setForm(p => ({ ...p, [k]: v })) }

  async function handleSubmit(e) {
    e.preventDefault()
    if (!form.subscriber_id) { toast.error('Validation', 'Subscriber is required'); return }
    if (!form.key.trim()) { toast.error('Validation', 'Key is required'); return }
    setSaving(true)
    try {
      const payload = { subscriber_id: Number(form.subscriber_id), key: form.key, value: form.value }
      if (isEdit) {
        await updateSubscriberAttribute(attr.subscriber_attributes_id, payload)
        toast.success('Updated', `Attribute "${form.key}" updated`)
      } else {
        await createSubscriberAttribute(payload)
        toast.success('Created', `Attribute "${form.key}" created`)
      }
      onSaved(); onClose()
    } catch (err) { toast.error('Save failed', err.message) }
    finally { setSaving(false) }
  }

  return (
    <Modal title={isEdit ? 'Edit Attribute' : 'Add Subscriber Attribute'} onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">Subscriber <span style={{ color: 'var(--danger)' }}>*</span></label>
            <select className="select" value={form.subscriber_id} onChange={e => set('subscriber_id', e.target.value)} disabled={isEdit || loadingSubscribers} required>
              <option value="">{loadingSubscribers ? 'Loading subscribers...' : '— Select subscriber —'}</option>
              {availableSubscribers.map(s => (
                <option key={s.subscriber_id} value={String(s.subscriber_id)}>
                  {s.imsi}{s.msisdn ? ` (${s.msisdn})` : ''}
                </option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label className="form-label">Key <span style={{ color: 'var(--danger)' }}>*</span></label>
            <input className="input mono" value={form.key} onChange={e => set('key', e.target.value)} placeholder="custom_attribute" required maxLength={60} />
          </div>
          <div className="form-group">
            <label className="form-label">Value</label>
            <textarea
              className="input"
              value={form.value}
              onChange={e => set('value', e.target.value)}
              placeholder="Attribute value (up to 12000 chars)"
              rows={4}
              style={{ resize: 'vertical', fontFamily: 'var(--font-mono)', fontSize: '0.82rem' }}
            />
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

export default function SubscriberAttributes({ compact = false }) {
  const toast = useToast()
  const fetchFn = useCallback(getSubscriberAttributes, [])
  const { data, error, loading, refresh } = usePoller(fetchFn, 30000)
  const [subscribers, setSubscribers] = useState([])
  const [search, setSearch] = useState('')
  const [modal, setModal] = useState(null)
  const [delConfirm, setDelConfirm] = useState(null)
  const [deleting, setDeleting] = useState(null)

  useEffect(() => {
    getSubscribers().then(d => setSubscribers(Array.isArray(d?.items) ? d.items : [])).catch(() => {})
  }, [])

  const subMap = {}
  subscribers.forEach(s => { subMap[s.subscriber_id] = s.imsi })

  const rows = Array.isArray(data) ? data : []
  const filtered = rows.filter(r => {
    if (!search) return true
    const q = search.toLowerCase()
    return (r.key || '').toLowerCase().includes(q) ||
      (r.value || '').toLowerCase().includes(q) ||
      (subMap[r.subscriber_id] || '').includes(q)
  })
  const enriched = filtered.map(r => ({ ...r, _imsi: subMap[r.subscriber_id] || '' }))
  const { sorted, sortKey, sortDir, handleSort } = useSort(enriched, '_imsi')

  function SortIcon({ col }) {
    if (sortKey !== col) return <span className="sort-icon"><ChevronsUpDown size={11} /></span>
    return <span className="sort-icon">{sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />}</span>
  }

  async function handleDelete(row) {
    setDeleting(row.subscriber_attributes_id)
    try {
      await deleteSubscriberAttribute(row.subscriber_attributes_id)
      toast.success('Deleted', `Attribute "${row.key}" deleted`)
      setDelConfirm(null); refresh()
    } catch (err) { toast.error('Delete failed', err.message) }
    finally { setDeleting(null) }
  }

  if (loading) return <div className="loading-center"><Spinner size="lg" /><span>Loading attributes...</span></div>
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
            <div className="page-title">Subscriber Attributes</div>
            <div className="page-subtitle">Custom key-value attributes per subscriber — {rows.length} total</div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
            <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add Attribute</button>
          </div>
        </div>
      )}
      {compact && (
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginBottom: 8 }}>
          <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
          <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add Attribute</button>
        </div>
      )}

      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <div style={{ position: 'relative', flex: 1, maxWidth: 340 }}>
          <Search size={14} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
          <input className="input" style={{ paddingLeft: 32 }} placeholder="Search by IMSI, key, or value..."
            value={search} onChange={e => setSearch(e.target.value)} />
        </div>
      </div>

      {filtered.length === 0 ? (
        <div className="empty-state">
          <div style={{ marginBottom: 8 }}><Tag size={32} style={{ color: 'var(--text-muted)' }} /></div>
          <div>{search ? 'No attributes match your search.' : 'No subscriber attributes configured.'}</div>
          {!search && <button className="btn btn-primary" style={{ marginTop: 12 }} onClick={() => setModal('add')}><Plus size={14} /> Add Attribute</button>}
        </div>
      ) : (
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th className={`sortable${sortKey === '_imsi' ? ' sort-active' : ''}`} onClick={() => handleSort('_imsi')}>IMSI<SortIcon col="_imsi" /></th>
                <th className={`sortable${sortKey === 'key' ? ' sort-active' : ''}`} onClick={() => handleSort('key')}>Key<SortIcon col="key" /></th>
                <th className={`sortable${sortKey === 'value' ? ' sort-active' : ''}`} onClick={() => handleSort('value')}>Value<SortIcon col="value" /></th>
                <th className={`sortable${sortKey === 'last_modified' ? ' sort-active' : ''}`} onClick={() => handleSort('last_modified')}>Last Modified<SortIcon col="last_modified" /></th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {sorted.map(row => (
                <tr key={row.subscriber_attributes_id}>
                  <td className="mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>{subMap[row.subscriber_id] || `sub#${row.subscriber_id}`}</td>
                  <td className="mono" style={{ fontSize: '0.82rem', color: 'var(--accent)' }}>{row.key}</td>
                  <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)', maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{row.value || '—'}</td>
                  <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{row.last_modified ? new Date(row.last_modified).toLocaleString() : '—'}</td>
                  <td>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button className="btn-icon" title="Edit" onClick={() => setModal({ attr: row })}><Pencil size={13} /></button>
                      <button className="btn-icon danger" title="Delete" onClick={() => setDelConfirm(row)} disabled={deleting === row.subscriber_attributes_id}>
                        {deleting === row.subscriber_attributes_id ? <Spinner size="sm" /> : <Trash2 size={13} />}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {modal === 'add' && <AttributeModal onClose={() => setModal(null)} onSaved={refresh} subscribers={subscribers} />}
      {modal?.attr && <AttributeModal attr={modal.attr} onClose={() => setModal(null)} onSaved={refresh} subscribers={subscribers} />}

      {delConfirm && (
        <Modal title="Delete Attribute" onClose={() => setDelConfirm(null)}>
          <div className="modal-body">
            <p>Delete attribute <strong>"{delConfirm.key}"</strong> for subscriber <span className="mono">{subMap[delConfirm.subscriber_id] || `#${delConfirm.subscriber_id}`}</span>? This cannot be undone.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setDelConfirm(null)}>Cancel</button>
            <button className="btn btn-danger" onClick={() => handleDelete(delConfirm)} disabled={deleting === delConfirm.subscriber_attributes_id}>
              {deleting === delConfirm.subscriber_attributes_id ? <Spinner size="sm" /> : 'Delete'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
