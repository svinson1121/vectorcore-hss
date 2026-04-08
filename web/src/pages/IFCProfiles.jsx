import React, { useState, useCallback, useRef, useEffect } from 'react'
import { Plus, Pencil, Trash2, RefreshCw, FileCode, Maximize2, Minimize2, ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import { useSort } from '../hooks/useSort.js'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getIFCProfiles, createIFCProfile, updateIFCProfile, deleteIFCProfile, getIMSSubscribers } from '../api/client.js'

const DEFAULT_IFC_TEMPLATE = `<?xml version="1.0" encoding="UTF-8"?>
<!--Default iFC template used by VectorCore HSS. Variables: {imsi} {msisdn} {mnc} {mcc}-->
<IMSSubscription>
    <PrivateID>{imsi}@ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</PrivateID>
    <ServiceProfile>
        <PublicIdentity>
            <Identity>sip:{msisdn}@ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</Identity>
            <Extension>
                <IdentityType>0</IdentityType>
                <Extension>
                    <AliasIdentityGroupID>1</AliasIdentityGroupID>
                </Extension>
            </Extension>
        </PublicIdentity>
        <PublicIdentity>
            <Identity>tel:{msisdn}</Identity>
            <Extension>
                <IdentityType>0</IdentityType>
                <Extension>
                    <AliasIdentityGroupID>1</AliasIdentityGroupID>
                </Extension>
            </Extension>
        </PublicIdentity>
        <PublicIdentity>
            <Identity>sip:{imsi}@ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</Identity>
            <Extension>
                <IdentityType>0</IdentityType>
            </Extension>
        </PublicIdentity>

        <!-- Copy SIP REGISTER towards Application Server -->
        <!--
        <InitialFilterCriteria>
            <Priority>10</Priority>
            <TriggerPoint>
                <ConditionTypeCNF>0</ConditionTypeCNF>
                <SPT>
                    <ConditionNegated>0</ConditionNegated>
                    <Group>0</Group>
                    <Method>REGISTER</Method>
                    <Extension></Extension>
                </SPT>
            </TriggerPoint>
            <ApplicationServer>
                <ServerName>sip:applicationserver.mnc{mnc}.mcc{mcc}.3gppnetwork.org:5060</ServerName>
                <DefaultHandling>0</DefaultHandling>
                <Extension>
                    <IncludeRegisterRequest/>
                    <IncludeRegisterResponse/>
                </Extension>
            </ApplicationServer>
        </InitialFilterCriteria>
        -->

        <!-- Copy SIP REGISTER towards SMSc -->
        <InitialFilterCriteria>
            <Priority>11</Priority>
            <TriggerPoint>
                <ConditionTypeCNF>0</ConditionTypeCNF>
                <SPT>
                    <ConditionNegated>0</ConditionNegated>
                    <Group>0</Group>
                    <Method>REGISTER</Method>
                    <Extension></Extension>
                </SPT>
            </TriggerPoint>
            <ApplicationServer>
                <ServerName>sip:smsc.ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org:5060</ServerName>
                <DefaultHandling>0</DefaultHandling>
                <Extension>
                    <IncludeRegisterRequest/>
                    <IncludeRegisterResponse/>
                </Extension>
            </ApplicationServer>
        </InitialFilterCriteria>

        <!-- SIP MESSAGE Traffic -->
        <InitialFilterCriteria>
            <Priority>20</Priority>
            <TriggerPoint>
                <ConditionTypeCNF>1</ConditionTypeCNF>
                <SPT>
                    <ConditionNegated>0</ConditionNegated>
                    <Group>0</Group>
                    <Method>MESSAGE</Method>
                    <Extension></Extension>
                </SPT>
                <SPT>
                    <ConditionNegated>1</ConditionNegated>
                    <Group>1</Group>
                    <SIPHeader>
                        <Header>Server</Header>
                    </SIPHeader>
                </SPT>
                <SPT>
                    <ConditionNegated>0</ConditionNegated>
                    <Group>2</Group>
                    <SessionCase>0</SessionCase>
                    <Extension></Extension>
                </SPT>
            </TriggerPoint>
            <ApplicationServer>
                <ServerName>sip:smsc.ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org:5060</ServerName>
                <DefaultHandling>0</DefaultHandling>
            </ApplicationServer>
        </InitialFilterCriteria>

        <!-- SIP USSD Traffic to USSD-GW -->
        <InitialFilterCriteria>
            <Priority>25</Priority>
            <TriggerPoint>
                <ConditionTypeCNF>1</ConditionTypeCNF>
                <SPT>
                    <ConditionNegated>0</ConditionNegated>
                    <Group>1</Group>
                    <SIPHeader>
                        <Header>Recv-Info</Header>
                        <Content>"g.3gpp.ussd"</Content>
                    </SIPHeader>
                </SPT>
            </TriggerPoint>
            <ApplicationServer>
                <ServerName>sip:ussd.ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org:5060</ServerName>
                <DefaultHandling>0</DefaultHandling>
            </ApplicationServer>
        </InitialFilterCriteria>

        <!-- SIP INVITE Traffic from Registered Sub -->
        <!--
        <InitialFilterCriteria>
            <Priority>30</Priority>
            <TriggerPoint>
                <ConditionTypeCNF>1</ConditionTypeCNF>
                <SPT>
                    <ConditionNegated>0</ConditionNegated>
                    <Group>0</Group>
                    <Method>INVITE</Method>
                    <Extension></Extension>
                </SPT>
                <SPT>
                    <Group>0</Group>
                    <SessionCase>0</SessionCase>
                </SPT>
            </TriggerPoint>
            <ApplicationServer>
                <ServerName>sip:softswitch.ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org</ServerName>
                <DefaultHandling>0</DefaultHandling>
            </ApplicationServer>
        </InitialFilterCriteria>
        -->

        <!-- SIP INVITE Traffic for calls to Unregistered Sub (TERMINATING_UNREGISTERED) -->
        <!--
        <InitialFilterCriteria>
            <Priority>40</Priority>
            <TriggerPoint>
                <ConditionTypeCNF>0</ConditionTypeCNF>
                <SPT>
                    <ConditionNegated>0</ConditionNegated>
                    <Group>0</Group>
                    <Method>INVITE</Method>
                    <Extension></Extension>
                </SPT>
                <SPT>
                    <Group>0</Group>
                    <SessionCase>2</SessionCase>
                </SPT>
            </TriggerPoint>
            <ApplicationServer>
                <ServerName>sip:softswitch.ims.mnc{mnc}.mcc{mcc}.3gppnetwork.org:5060</ServerName>
                <DefaultHandling>0</DefaultHandling>
            </ApplicationServer>
        </InitialFilterCriteria>
        -->

    </ServiceProfile>
</IMSSubscription>`

/** Simple XML editor textarea with Tab-key support and line numbers */
function XMLEditor({ value, onChange, rows = 16 }) {
  const textareaRef = useRef(null)
  const lineCount = (value || '').split('\n').length
  const lineNumbers = Array.from({ length: Math.max(lineCount, rows) }, (_, i) => i + 1)

  function handleKeyDown(e) {
    if (e.key === 'Tab') {
      e.preventDefault()
      const ta = textareaRef.current
      const start = ta.selectionStart
      const end = ta.selectionEnd
      const newVal = value.slice(0, start) + '  ' + value.slice(end)
      onChange(newVal)
      // Restore cursor after React re-render
      requestAnimationFrame(() => {
        ta.selectionStart = ta.selectionEnd = start + 2
      })
    }
  }

  return (
    <div style={{
      display: 'flex',
      border: '1px solid var(--border)',
      borderRadius: 'var(--radius-sm)',
      background: 'var(--bg-input)',
      overflow: 'hidden',
      fontFamily: 'var(--font-mono)',
      fontSize: '0.8rem',
      lineHeight: '1.6',
    }}>
      {/* Line numbers */}
      <div style={{
        padding: '7px 8px 7px 10px',
        background: 'var(--bg-elevated)',
        borderRight: '1px solid var(--border-subtle)',
        color: 'var(--text-muted)',
        textAlign: 'right',
        userSelect: 'none',
        minWidth: 36,
        flexShrink: 0,
        overflowY: 'hidden',
      }}>
        {lineNumbers.map(n => <div key={n}>{n}</div>)}
      </div>
      {/* Editor */}
      <textarea
        ref={textareaRef}
        value={value}
        onChange={e => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={DEFAULT_IFC_TEMPLATE}
        rows={rows}
        style={{
          flex: 1,
          resize: 'vertical',
          border: 'none',
          outline: 'none',
          padding: '7px 10px',
          background: 'transparent',
          color: 'var(--text)',
          fontFamily: 'inherit',
          fontSize: 'inherit',
          lineHeight: 'inherit',
          minHeight: rows * 1.6 * 13 + 14,
        }}
        spellCheck={false}
        autoCorrect="off"
        autoCapitalize="off"
      />
    </div>
  )
}

function IFCProfileModal({ profile, onClose, onSaved }) {
  const toast = useToast()
  const isEdit = !!profile
  const [form, setForm] = useState(isEdit ? {
    name: profile.name || '',
    xml_data: profile.xml_data || '',
  } : { name: '', xml_data: DEFAULT_IFC_TEMPLATE })
  const [saving, setSaving] = useState(false)

  function set(k, v) { setForm(prev => ({ ...prev, [k]: v })) }

  async function handleSubmit(e) {
    e.preventDefault()
    if (!form.name.trim()) { toast.error('Validation', 'Profile name is required'); return }
    if (!form.xml_data.trim()) { toast.error('Validation', 'XML data is required'); return }
    setSaving(true)
    try {
      const payload = { name: form.name, xml_data: form.xml_data }
      if (isEdit) {
        await updateIFCProfile(profile.ifc_profile_id, payload)
        toast.success('Updated', `IFC profile "${form.name}" updated`)
      } else {
        await createIFCProfile(payload)
        toast.success('Created', `IFC profile "${form.name}" created`)
      }
      onSaved(); onClose()
    } catch (err) {
      toast.error(isEdit ? 'Update failed' : 'Create failed', err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal title={isEdit ? 'Edit IFC Profile' : 'Add IFC Profile'} onClose={onClose} size="lg">
      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">Profile Name <span style={{ color: 'var(--danger)' }}>*</span></label>
            <input
              className="input"
              value={form.name}
              onChange={e => set('name', e.target.value)}
              placeholder="Default IFC Profile"
              required
            />
          </div>
          {isEdit && (
            <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
              Profile ID: <span style={{ fontFamily: 'var(--font-mono)' }}>{profile.ifc_profile_id}</span>
            </div>
          )}

          <div className="form-group">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 5 }}>
              <label className="form-label" style={{ margin: 0 }}>
                IFC XML Data <span style={{ color: 'var(--danger)' }}>*</span>
              </label>
              <span style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>
                Tab = 2 spaces
              </span>
            </div>
            <XMLEditor value={form.xml_data} onChange={v => set('xml_data', v)} rows={18} />
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

export default function IFCProfiles({ compact = false }) {
  const toast = useToast()
  const fetchFn = useCallback(getIFCProfiles, [])
  const { data, error, loading, refresh } = usePoller(fetchFn, 30000)

  const [modal, setModal] = useState(null)
  const [delConfirm, setDelConfirm] = useState(null)
  const [deleting, setDeleting] = useState(null)
  const [imsSubscribers, setIMSSubscribers] = useState([])

  const profiles = Array.isArray(data) ? data : []
  const { sorted, sortKey, sortDir, handleSort } = useSort(profiles, 'name')

  useEffect(() => {
    getIMSSubscribers().then(d => setIMSSubscribers(Array.isArray(d?.items) ? d.items : Array.isArray(d) ? d : [])).catch(() => {})
  }, [])

  function SortIcon({ col }) {
    if (sortKey !== col) return <span className="sort-icon"><ChevronsUpDown size={11} /></span>
    return <span className="sort-icon">{sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />}</span>
  }

  function profileUsageReason(profile) {
    const ims = imsSubscribers.find(row => Number(row.ifc_profile_id) === profile.ifc_profile_id)
    if (ims) return `IFC profile is still used by IMS subscriber ${ims.msisdn || ims.impi || ims.imsi || `#${ims.ims_subscriber_id}`}`
    return ''
  }

  async function handleDelete(profile) {
    setDeleting(profile.ifc_profile_id)
    try {
      await deleteIFCProfile(profile.ifc_profile_id)
      toast.success('Deleted', `IFC profile "${profile.name}" deleted`)
      setDelConfirm(null)
      refresh()
    } catch (err) {
      toast.error('Delete failed', err.message)
    } finally {
      setDeleting(null)
    }
  }

  if (loading) return <div className="loading-center"><Spinner size="lg" /><span>Loading IFC profiles...</span></div>
  if (error && profiles.length === 0) return (
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
            <div className="page-title">IFC Profiles</div>
            <div className="page-subtitle">Initial Filter Criteria profiles for IMS/VoLTE — {profiles.length} total</div>
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
          <div style={{ marginBottom: 8 }}><FileCode size={32} style={{ color: 'var(--text-muted)' }} /></div>
          <div>No IFC profiles configured.</div>
          <button className="btn btn-primary" style={{ marginTop: 12 }} onClick={() => setModal('add')}>
            <Plus size={14} /> Add Profile
          </button>
        </div>
      ) : (
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th className={`sortable${sortKey === 'name' ? ' sort-active' : ''}`} onClick={() => handleSort('name')}>Profile Name<SortIcon col="name" /></th>
                <th className={`sortable${sortKey === 'last_modified' ? ' sort-active' : ''}`} onClick={() => handleSort('last_modified')}>Last Modified<SortIcon col="last_modified" /></th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {sorted.map(p => (
                <tr key={p.ifc_profile_id}>
                  <td style={{ fontWeight: 600 }}>{p.name}</td>
                  <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                    {p.last_modified ? new Date(p.last_modified).toLocaleString() : '—'}
                  </td>
                  <td>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button className="btn-icon" title="Edit" onClick={() => setModal({ profile: p })}><Pencil size={13} /></button>
                      <button
                        className="btn-icon danger"
                        title={profileUsageReason(p) || 'Delete'}
                        onClick={() => setDelConfirm(p)}
                        disabled={deleting === p.ifc_profile_id || !!profileUsageReason(p)}
                      >
                        {deleting === p.ifc_profile_id ? <Spinner size="sm" /> : <Trash2 size={13} />}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {modal === 'add' && <IFCProfileModal onClose={() => setModal(null)} onSaved={refresh} />}
      {modal && modal.profile && <IFCProfileModal profile={modal.profile} onClose={() => setModal(null)} onSaved={refresh} />}

      {delConfirm && (
        <Modal title="Delete IFC Profile" onClose={() => setDelConfirm(null)}>
          <div className="modal-body">
            <p>Delete IFC profile <strong>"{delConfirm.name}"</strong>? Subscribers referencing this profile will lose their IFC configuration. This cannot be undone.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setDelConfirm(null)}>Cancel</button>
            <button className="btn btn-danger" onClick={() => handleDelete(delConfirm)} disabled={deleting === delConfirm.ifc_profile_id}>
              {deleting === delConfirm.ifc_profile_id ? <Spinner size="sm" /> : 'Delete'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
