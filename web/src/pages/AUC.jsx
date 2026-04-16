import React, { useState, useEffect } from 'react'
import { Plus, Pencil, Trash2, Search, RefreshCw, Eye, EyeOff, ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import { useSort } from '../hooks/useSort.js'
import { useInfiniteScroll } from '../hooks/useInfiniteScroll.js'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import { getAUCs, createAUC, updateAUC, deleteAUC, getAlgorithmProfiles, getSubscribers } from '../api/client.js'
import AlgorithmProfiles from './AlgorithmProfiles.jsx'

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

const EMPTY_FORM = {
  imsi: '', iccid: '', ki: '', opc: '', amf: '8000',
  esim: false, sim_vendor: '', batch_name: '', lpa: '',
  pin1: '', pin2: '', puk1: '', puk2: '',
  kid: '', psk: '', des: '', adm1: '',
  misc1: '', misc2: '', misc3: '', misc4: '',
  algorithm_profile_id: '',
}

function AUCModal({ auc, onClose, onSaved, algorithmProfiles }) {
  const toast = useToast()
  const isEdit = !!auc
  const [showKeys, setShowKeys] = useState(!isEdit)
  const [availableAlgorithmProfiles, setAvailableAlgorithmProfiles] = useState(algorithmProfiles)
  const [loadingAlgorithmProfiles, setLoadingAlgorithmProfiles] = useState(true)
  const [form, setForm] = useState(auc ? {
    imsi: auc.imsi || '',
    iccid: auc.iccid || '',
    ki: '', opc: '',
    amf: auc.amf || '8000',
    esim: auc.esim || false,
    sim_vendor: auc.sim_vendor || '',
    batch_name: auc.batch_name || '',
    lpa: auc.lpa || '',
    pin1: auc.pin1 || '', pin2: auc.pin2 || '',
    puk1: auc.puk1 || '', puk2: auc.puk2 || '',
    kid: '', psk: '', des: '', adm1: '',
    misc1: auc.misc1 || '', misc2: auc.misc2 || '',
    misc3: auc.misc3 || '', misc4: auc.misc4 || '',
    algorithm_profile_id: auc.algorithm_profile_id != null ? String(auc.algorithm_profile_id) : '',
  } : { ...EMPTY_FORM })
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let active = true

    async function loadAlgorithmProfiles() {
      setLoadingAlgorithmProfiles(true)
      try {
        const data = await getAlgorithmProfiles()
        if (active) setAvailableAlgorithmProfiles(Array.isArray(data) ? data : [])
      } catch (err) {
        if (active) {
          setAvailableAlgorithmProfiles(Array.isArray(algorithmProfiles) ? algorithmProfiles : [])
          toast.error('Algorithm profiles', err.message || 'Failed to load algorithm profiles')
        }
      } finally {
        if (active) setLoadingAlgorithmProfiles(false)
      }
    }

    loadAlgorithmProfiles()
    return () => { active = false }
  }, [algorithmProfiles, toast])

  function set(k, v) { setForm(prev => ({ ...prev, [k]: v })) }

  async function handleSubmit(e) {
    e.preventDefault()
    setSaving(true)
    try {
      if (isEdit) {
        // Partial update — only send fields that have values; omit empty strings
        // so the backend preserves existing Ki/OPc.
        const payload = {}
        if (form.ki)    payload.ki  = form.ki
        if (form.opc)   payload.opc = form.opc
        if (form.amf)   payload.amf = form.amf
        if (form.iccid) payload.iccid = form.iccid
        if (form.kid)   payload.kid  = form.kid
        if (form.psk)   payload.psk  = form.psk
        if (form.des)   payload.des  = form.des
        if (form.adm1)  payload.adm1 = form.adm1
        payload.sim_vendor  = form.sim_vendor  || null
        payload.batch_name  = form.batch_name  || null
        payload.esim        = form.esim
        payload.lpa         = form.lpa  || null
        payload.pin1        = form.pin1 || null
        payload.pin2        = form.pin2 || null
        payload.puk1        = form.puk1 || null
        payload.puk2        = form.puk2 || null
        payload.misc1       = form.misc1 || null
        payload.misc2       = form.misc2 || null
        payload.misc3       = form.misc3 || null
        payload.misc4       = form.misc4 || null
        payload.algorithm_profile_id = form.algorithm_profile_id !== ''
          ? parseInt(form.algorithm_profile_id, 10) : null
        await updateAUC(auc.auc_id, payload)
        toast.success('AUC updated', form.imsi)
      } else {
        const payload = { ...form }
        payload.algorithm_profile_id = payload.algorithm_profile_id !== ''
          ? parseInt(payload.algorithm_profile_id, 10) : null
        await createAUC(payload)
        toast.success('AUC created', form.imsi)
      }
      onSaved(); onClose()
    } catch (err) {
      toast.error(isEdit ? 'Update failed' : 'Create failed', err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal title={isEdit ? 'Edit AUC Entry' : 'Add AUC Entry'} onClose={onClose} size="lg">
      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
        <div className="modal-body">

          <div style={SECTION_STYLE}>Identity</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">IMSI <span style={{ color: 'var(--danger)' }}>*</span></label>
              <input
                className="input mono"
                value={form.imsi}
                onChange={e => set('imsi', e.target.value)}
                placeholder="001010000000001"
                required
                disabled={isEdit}
                maxLength={15}
              />
            </div>
            <div className="form-group">
              <label className="form-label">ICCID</label>
              <input
                className="input mono"
                value={form.iccid}
                onChange={e => set('iccid', e.target.value)}
                placeholder="89xxxxxxxxxxxxxxxxxxxx"
                maxLength={22}
              />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">SIM Vendor</label>
              <input className="input" value={form.sim_vendor} onChange={e => set('sim_vendor', e.target.value)} placeholder="(optional)" />
            </div>
            <div className="form-group">
              <label className="form-label">Batch Name</label>
              <input className="input" value={form.batch_name} onChange={e => set('batch_name', e.target.value)} placeholder="(optional)" />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Algorithm Profile</label>
              <select className="select" value={form.algorithm_profile_id} onChange={e => set('algorithm_profile_id', e.target.value)} disabled={loadingAlgorithmProfiles}>
                <option value="">Default (Standard Milenage)</option>
                {(availableAlgorithmProfiles || []).map(p => (
                  <option key={p.algorithm_profile_id} value={String(p.algorithm_profile_id)}>
                    {p.profile_name}
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group" style={{ alignSelf: 'flex-end' }}>
              <label className="checkbox-wrap">
                <input type="checkbox" checked={form.esim} onChange={e => set('esim', e.target.checked)} />
                <span className="form-label" style={{ margin: 0 }}>eSIM</span>
              </label>
            </div>
          </div>
          {form.esim && (
            <div className="form-group">
              <label className="form-label">LPA (eSIM activation code)</label>
              <input className="input mono" value={form.lpa} onChange={e => set('lpa', e.target.value)} placeholder="LPA:1:..." />
            </div>
          )}

          <div style={{ ...SECTION_STYLE, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span>Authentication Keys</span>
            {isEdit && (
              <button
                type="button"
                className="btn btn-ghost"
                style={{ fontSize: '0.72rem', padding: '2px 8px', height: 'auto' }}
                onClick={() => setShowKeys(v => !v)}
              >
                {showKeys ? <EyeOff size={12} /> : <Eye size={12} />}
                {showKeys ? ' Hide Keys' : ' Show Keys'}
              </button>
            )}
          </div>
          {showKeys ? (
            <>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">Ki (hex 32){!isEdit && <span style={{ color: 'var(--danger)' }}> *</span>}</label>
                  <input className="input mono" value={form.ki} onChange={e => set('ki', e.target.value)}
                    placeholder="0000000000000000000000000000000" maxLength={32} required={!isEdit} />
                </div>
                <div className="form-group">
                  <label className="form-label">OPc (hex 32){!isEdit && <span style={{ color: 'var(--danger)' }}> *</span>}</label>
                  <input className="input mono" value={form.opc} onChange={e => set('opc', e.target.value)}
                    placeholder="0000000000000000000000000000000" maxLength={32} required={!isEdit} />
                </div>
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">AMF (hex 4)</label>
                  <input className="input mono" value={form.amf} onChange={e => set('amf', e.target.value)} placeholder="8000" maxLength={4} />
                </div>
                <div className="form-group">
                  <label className="form-label">KID</label>
                  <input className="input mono" value={form.kid} onChange={e => set('kid', e.target.value)} placeholder="(optional)" />
                </div>
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">PSK</label>
                  <input className="input mono" value={form.psk} onChange={e => set('psk', e.target.value)} placeholder="(optional)" />
                </div>
                <div className="form-group">
                  <label className="form-label">DES</label>
                  <input className="input mono" value={form.des} onChange={e => set('des', e.target.value)} placeholder="(optional)" />
                </div>
              </div>
              <div className="form-group">
                <label className="form-label">ADM1</label>
                <input className="input mono" value={form.adm1} onChange={e => set('adm1', e.target.value)} placeholder="(optional)" />
              </div>
            </>
          ) : isEdit ? (
            <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', padding: '8px 0' }}>
              Key fields hidden. Click "Show Keys" to edit.
            </div>
          ) : null}

          <div style={SECTION_STYLE}>SIM Codes</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">PIN1</label>
              <input className="input mono" value={form.pin1} onChange={e => set('pin1', e.target.value)} placeholder="(optional)" />
            </div>
            <div className="form-group">
              <label className="form-label">PIN2</label>
              <input className="input mono" value={form.pin2} onChange={e => set('pin2', e.target.value)} placeholder="(optional)" />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">PUK1</label>
              <input className="input mono" value={form.puk1} onChange={e => set('puk1', e.target.value)} placeholder="(optional)" />
            </div>
            <div className="form-group">
              <label className="form-label">PUK2</label>
              <input className="input mono" value={form.puk2} onChange={e => set('puk2', e.target.value)} placeholder="(optional)" />
            </div>
          </div>

          <div style={SECTION_STYLE}>Misc</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Misc 1</label>
              <input className="input" value={form.misc1} onChange={e => set('misc1', e.target.value)} placeholder="(optional)" />
            </div>
            <div className="form-group">
              <label className="form-label">Misc 2</label>
              <input className="input" value={form.misc2} onChange={e => set('misc2', e.target.value)} placeholder="(optional)" />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Misc 3</label>
              <input className="input" value={form.misc3} onChange={e => set('misc3', e.target.value)} placeholder="(optional)" />
            </div>
            <div className="form-group">
              <label className="form-label">Misc 4</label>
              <input className="input" value={form.misc4} onChange={e => set('misc4', e.target.value)} placeholder="(optional)" />
            </div>
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

export default function AUC() {
  const toast = useToast()
  const [algorithmProfiles, setAlgorithmProfiles] = useState([])
  const [activeTab, setActiveTab] = useState('auc')
  const [search, setSearch] = useState('')
  const [modal, setModal] = useState(null)
  const [deleting, setDeleting] = useState(null)
  const [subscribers, setSubscribers] = useState([])

  const { items, total, loading, loadingMore, error, sentinelRef, refresh } = useInfiniteScroll(getAUCs, search)

  useEffect(() => {
    getAlgorithmProfiles().then(d => setAlgorithmProfiles(Array.isArray(d) ? d : [])).catch(() => {})
    getSubscribers().then(d => setSubscribers(Array.isArray(d?.items) ? d.items : [])).catch(() => {})
  }, [])

  const profileMap = {}
  algorithmProfiles.forEach(p => { profileMap[p.algorithm_profile_id] = p.profile_name })

  const enriched = items.map(a => ({
    ...a,
    _algorithm: a.algorithm_profile_id != null ? (profileMap[a.algorithm_profile_id] || `#${a.algorithm_profile_id}`) : 'Default',
  }))
  const { sorted: sortedAucs, sortKey, sortDir, handleSort } = useSort(enriched, 'imsi')

  function SortIcon({ col }) {
    if (sortKey !== col) return <span className="sort-icon"><ChevronsUpDown size={11} /></span>
    return <span className="sort-icon">{sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />}</span>
  }

  function aucUsageReason(auc) {
    const sub = subscribers.find(row => Number(row.auc_id) === auc.auc_id)
    if (sub) return `AUC is still used by subscriber ${sub.imsi}`
    return ''
  }

  async function handleDelete(auc) {
    if (!window.confirm(`Delete AUC entry for IMSI ${auc.imsi}?`)) return
    setDeleting(auc.auc_id)
    try {
      await deleteAUC(auc.auc_id)
      toast.success('AUC deleted', auc.imsi)
      refresh()
    } catch (err) {
      toast.error('Delete failed', err.message)
    } finally {
      setDeleting(null)
    }
  }

  if (loading) return <div className="loading-center"><Spinner size="lg" /><span>Loading AUC entries...</span></div>
  if (error && items.length === 0) return (
    <div className="error-state">
      <div>{error}</div>
      <button className="btn btn-ghost mt-12" onClick={refresh}><RefreshCw size={14} /> Retry</button>
    </div>
  )

  return (
    <div>
      <div className="page-header">
        <div>
          <div className="page-title">SIM Cards / AUC</div>
          <div className="page-subtitle">{total != null ? `${total} AUC entr${total !== 1 ? 'ies' : 'y'} configured` : 'Loading…'}</div>
        </div>
        {activeTab === 'auc' && (
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
            <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add AUC</button>
          </div>
        )}
      </div>

      <div style={{ display: 'flex', borderBottom: '1px solid var(--border)', marginBottom: 16 }}>
        <button style={TAB_STYLE(activeTab === 'auc')} onClick={() => setActiveTab('auc')}>SIM Cards / AUC</button>
        <button style={TAB_STYLE(activeTab === 'profiles')} onClick={() => setActiveTab('profiles')}>Algorithm Profiles</button>
      </div>

      {activeTab === 'auc' && (<>
        <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
          <div style={{ position: 'relative', flex: 1, maxWidth: 360 }}>
            <Search size={14} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
            <input className="input" style={{ paddingLeft: 32 }} placeholder="Search by IMSI, ICCID, vendor, batch..."
              value={search} onChange={e => setSearch(e.target.value)} />
          </div>
        </div>

        {!loading && items.length === 0 ? (
          <div className="empty-state">{search ? 'No AUC entries match your search.' : 'No AUC entries configured yet.'}</div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th className={`sortable${sortKey === 'imsi' ? ' sort-active' : ''}`} onClick={() => handleSort('imsi')}>IMSI<SortIcon col="imsi" /></th>
                  <th className={`sortable${sortKey === 'iccid' ? ' sort-active' : ''}`} onClick={() => handleSort('iccid')}>ICCID<SortIcon col="iccid" /></th>
                  <th>AMF</th>
                  <th className={`sortable${sortKey === 'esim' ? ' sort-active' : ''}`} onClick={() => handleSort('esim')}>eSIM<SortIcon col="esim" /></th>
                  <th className={`sortable${sortKey === 'sim_vendor' ? ' sort-active' : ''}`} onClick={() => handleSort('sim_vendor')}>Vendor<SortIcon col="sim_vendor" /></th>
                  <th className={`sortable${sortKey === 'batch_name' ? ' sort-active' : ''}`} onClick={() => handleSort('batch_name')}>Batch<SortIcon col="batch_name" /></th>
                  <th className={`sortable${sortKey === '_algorithm' ? ' sort-active' : ''}`} onClick={() => handleSort('_algorithm')}>Algorithm<SortIcon col="_algorithm" /></th>
                  <th>SQN</th>
                  <th className={`sortable${sortKey === 'last_modified' ? ' sort-active' : ''}`} onClick={() => handleSort('last_modified')}>Last Modified<SortIcon col="last_modified" /></th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {sortedAucs.map(a => (
                  <tr key={a.auc_id}>
                    <td className="mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>{a.imsi}</td>
                    <td className="mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{a.iccid || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.78rem' }}>{a.amf || '—'}</td>
                    <td style={{ fontSize: '0.78rem' }}>
                      {a.esim ? <span style={{ color: 'var(--success)', fontWeight: 600 }}>Yes</span>
                              : <span style={{ color: 'var(--text-muted)' }}>No</span>}
                    </td>
                    <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{a.sim_vendor || '—'}</td>
                    <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{a.batch_name || '—'}</td>
                    <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                      {a.algorithm_profile_id != null ? (profileMap[a.algorithm_profile_id] || `#${a.algorithm_profile_id}`) : 'Default'}
                    </td>
                    <td className="mono" style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>{a.sqn ?? '—'}</td>
                    <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                      {a.last_modified ? new Date(a.last_modified).toLocaleString() : '—'}
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn-icon" title="Edit" onClick={() => setModal({ auc: a })}><Pencil size={13} /></button>
                        <button
                          className="btn-icon danger"
                          title={aucUsageReason(a) || 'Delete'}
                          onClick={() => handleDelete(a)}
                          disabled={deleting === a.auc_id || !!aucUsageReason(a)}
                        >
                          {deleting === a.auc_id ? <Spinner size="sm" /> : <Trash2 size={13} />}
                        </button>
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

        {modal === 'add' && <AUCModal onClose={() => setModal(null)} onSaved={refresh} algorithmProfiles={algorithmProfiles} />}
        {modal && modal.auc && <AUCModal auc={modal.auc} onClose={() => setModal(null)} onSaved={refresh} algorithmProfiles={algorithmProfiles} />}
      </>)}

      {activeTab === 'profiles' && <AlgorithmProfiles compact />}
    </div>
  )
}
