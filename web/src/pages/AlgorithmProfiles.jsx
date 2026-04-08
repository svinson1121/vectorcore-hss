import React, { useState, useCallback, useEffect } from 'react'
import { Plus, Pencil, Trash2, RefreshCw, Cpu } from 'lucide-react'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getAlgorithmProfiles, createAlgorithmProfile, updateAlgorithmProfile, deleteAlgorithmProfile, getAUCs } from '../api/client.js'

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

const DEFAULT_R = { r1: 64, r2: 0, r3: 32, r4: 64, r5: 96 }

function AlgorithmProfileModal({ profile, onClose, onSaved }) {
  const toast = useToast()
  const isEdit = !!profile
  const [form, setForm] = useState(isEdit ? {
    profile_name: profile.profile_name || '',
    c1: profile.c1 || '',
    c2: profile.c2 || '',
    c3: profile.c3 || '',
    c4: profile.c4 || '',
    c5: profile.c5 || '',
    r1: profile.r1 != null ? profile.r1 : DEFAULT_R.r1,
    r2: profile.r2 != null ? profile.r2 : DEFAULT_R.r2,
    r3: profile.r3 != null ? profile.r3 : DEFAULT_R.r3,
    r4: profile.r4 != null ? profile.r4 : DEFAULT_R.r4,
    r5: profile.r5 != null ? profile.r5 : DEFAULT_R.r5,
  } : {
    profile_name: '',
    c1: '',
    c2: '',
    c3: '',
    c4: '',
    c5: '',
    r1: DEFAULT_R.r1,
    r2: DEFAULT_R.r2,
    r3: DEFAULT_R.r3,
    r4: DEFAULT_R.r4,
    r5: DEFAULT_R.r5,
  })
  const [saving, setSaving] = useState(false)

  function set(k, v) {
    setForm(prev => ({ ...prev, [k]: v }))
  }

  async function handleSubmit(e) {
    e.preventDefault()
    if (!form.profile_name.trim()) {
      toast.error('Validation', 'Profile name is required')
      return
    }
    setSaving(true)
    try {
      const payload = {
        profile_name: form.profile_name,
        c1: form.c1 || undefined,
        c2: form.c2 || undefined,
        c3: form.c3 || undefined,
        c4: form.c4 || undefined,
        c5: form.c5 || undefined,
        r1: Number(form.r1),
        r2: Number(form.r2),
        r3: Number(form.r3),
        r4: Number(form.r4),
        r5: Number(form.r5),
      }
      if (isEdit) {
        await updateAlgorithmProfile(profile.algorithm_profile_id, payload)
        toast.success('Updated', `Algorithm profile "${form.profile_name}" updated`)
      } else {
        await createAlgorithmProfile(payload)
        toast.success('Created', `Algorithm profile "${form.profile_name}" created`)
      }
      onSaved()
      onClose()
    } catch (err) {
      toast.error(isEdit ? 'Update failed' : 'Create failed', err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal title={isEdit ? 'Edit Algorithm Profile' : 'Add Algorithm Profile'} onClose={onClose} size="lg">
      <form onSubmit={handleSubmit}>
        <div className="modal-body">

          <div style={SECTION_STYLE}>Identity</div>
          <div className="form-group">
            <label className="form-label">Profile Name <span style={{ color: 'var(--danger)' }}>*</span></label>
            <input
              className="input"
              value={form.profile_name}
              onChange={e => set('profile_name', e.target.value)}
              placeholder="Custom Milenage Profile"
              required
            />
          </div>

          <div style={SECTION_STYLE}>Milenage Constants (c1–c5)</div>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: 8 }}>
            Hex strings, 32 characters each (128-bit). Leave blank to use algorithm defaults.
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">c1</label>
              <input className="input mono" style={{ fontSize: '0.75rem' }} value={form.c1} onChange={e => set('c1', e.target.value)} placeholder="00000000000000000000000000000000" maxLength={32} />
            </div>
            <div className="form-group">
              <label className="form-label">c2</label>
              <input className="input mono" style={{ fontSize: '0.75rem' }} value={form.c2} onChange={e => set('c2', e.target.value)} placeholder="00000000000000000000000000000001" maxLength={32} />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">c3</label>
              <input className="input mono" style={{ fontSize: '0.75rem' }} value={form.c3} onChange={e => set('c3', e.target.value)} placeholder="00000000000000000000000000000002" maxLength={32} />
            </div>
            <div className="form-group">
              <label className="form-label">c4</label>
              <input className="input mono" style={{ fontSize: '0.75rem' }} value={form.c4} onChange={e => set('c4', e.target.value)} placeholder="00000000000000000000000000000004" maxLength={32} />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">c5</label>
              <input className="input mono" style={{ fontSize: '0.75rem' }} value={form.c5} onChange={e => set('c5', e.target.value)} placeholder="00000000000000000000000000000008" maxLength={32} />
            </div>
            <div className="form-group" />
          </div>

          <div style={SECTION_STYLE}>Rotation Values (r1–r5)</div>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: 8 }}>
            Integer bit rotation values. Defaults: r1=64, r2=0, r3=32, r4=64, r5=96.
          </div>
          <div className="form-row-3">
            <div className="form-group">
              <label className="form-label">r1 <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}>(default 64)</span></label>
              <input className="input" type="number" min="0" max="127" value={form.r1} onChange={e => set('r1', e.target.value)} />
            </div>
            <div className="form-group">
              <label className="form-label">r2 <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}>(default 0)</span></label>
              <input className="input" type="number" min="0" max="127" value={form.r2} onChange={e => set('r2', e.target.value)} />
            </div>
            <div className="form-group">
              <label className="form-label">r3 <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}>(default 32)</span></label>
              <input className="input" type="number" min="0" max="127" value={form.r3} onChange={e => set('r3', e.target.value)} />
            </div>
          </div>
          <div className="form-row-3">
            <div className="form-group">
              <label className="form-label">r4 <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}>(default 64)</span></label>
              <input className="input" type="number" min="0" max="127" value={form.r4} onChange={e => set('r4', e.target.value)} />
            </div>
            <div className="form-group">
              <label className="form-label">r5 <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}>(default 96)</span></label>
              <input className="input" type="number" min="0" max="127" value={form.r5} onChange={e => set('r5', e.target.value)} />
            </div>
            <div className="form-group" />
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

export default function AlgorithmProfiles({ compact = false }) {
  const toast = useToast()
  const fetchFn = useCallback(getAlgorithmProfiles, [])
  const { data, error, loading, refresh } = usePoller(fetchFn, 30000)

  const [modal, setModal] = useState(null)
  const [delConfirm, setDelConfirm] = useState(null)
  const [deleting, setDeleting] = useState(null)
  const [aucs, setAUCs] = useState([])

  const profiles = Array.isArray(data) ? data : []

  useEffect(() => {
    getAUCs().then(d => setAUCs(Array.isArray(d?.items) ? d.items : [])).catch(() => {})
  }, [])

  function profileUsageReason(profile) {
    const auc = aucs.find(row => Number(row.algorithm_profile_id) === profile.algorithm_profile_id)
    if (auc) return `Algorithm profile is still used by AUC ${auc.imsi || `#${auc.auc_id}`}`
    return ''
  }

  async function handleDelete(profile) {
    setDeleting(profile.algorithm_profile_id)
    try {
      await deleteAlgorithmProfile(profile.algorithm_profile_id)
      toast.success('Deleted', `Algorithm profile "${profile.profile_name}" deleted`)
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
        <span>Loading algorithm profiles...</span>
      </div>
    )
  }

  if (error && profiles.length === 0) {
    return (
      <div className="error-state">
        <div>{error}</div>
        <button className="btn btn-ghost mt-12" onClick={refresh}><RefreshCw size={14} /> Retry</button>
      </div>
    )
  }

  return (
    <div>
      {!compact && (
        <div className="page-header">
          <div>
            <div className="page-title">Algorithm Profiles</div>
            <div className="page-subtitle">Custom Milenage c/r constant profiles — {profiles.length} total</div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
            <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add Profile</button>
          </div>
        </div>
      )}
      {compact && (
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginBottom: 8 }}>
          <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
          <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add Profile</button>
        </div>
      )}

      {profiles.length === 0 ? (
        <div className="empty-state">
          <div style={{ marginBottom: 8 }}><Cpu size={32} style={{ color: 'var(--text-muted)' }} /></div>
          <div>No algorithm profiles configured.</div>
          <button className="btn btn-primary" style={{ marginTop: 12 }} onClick={() => setModal('add')}>
            <Plus size={14} /> Add Profile
          </button>
        </div>
      ) : (
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Profile Name</th>
                <th>R Values</th>
                <th>Last Modified</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {profiles.map(profile => (
                <tr key={profile.algorithm_profile_id}>
                  <td style={{ fontWeight: 600 }}>{profile.profile_name}</td>
                  <td className="mono" style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                    r1={profile.r1 ?? 64} r2={profile.r2 ?? 0} r3={profile.r3 ?? 32} r4={profile.r4 ?? 64} r5={profile.r5 ?? 96}
                  </td>
                  <td style={{ color: 'var(--text-muted)', fontSize: '0.78rem' }}>
                    {profile.last_modified ? new Date(profile.last_modified).toLocaleString() : '—'}
                  </td>
                  <td>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button className="btn-icon" title="Edit" onClick={() => setModal({ profile })}>
                        <Pencil size={13} />
                      </button>
                      <button
                        className="btn-icon danger"
                        title={profileUsageReason(profile) || 'Delete'}
                        onClick={() => setDelConfirm(profile)}
                        disabled={!!profileUsageReason(profile)}
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {modal === 'add' && (
        <AlgorithmProfileModal onClose={() => setModal(null)} onSaved={refresh} />
      )}
      {modal && modal.profile && (
        <AlgorithmProfileModal profile={modal.profile} onClose={() => setModal(null)} onSaved={refresh} />
      )}

      {delConfirm && (
        <Modal title="Delete Algorithm Profile" onClose={() => setDelConfirm(null)}>
          <div className="modal-body">
            <p>Delete algorithm profile <strong>{delConfirm.profile_name}</strong>? This cannot be undone.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setDelConfirm(null)}>Cancel</button>
            <button
              className="btn btn-danger"
              onClick={() => handleDelete(delConfirm)}
              disabled={deleting === delConfirm.algorithm_profile_id}
            >
              {deleting === delConfirm.algorithm_profile_id ? <Spinner size="sm" /> : 'Delete'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
