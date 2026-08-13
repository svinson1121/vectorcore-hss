import React, { useState, useCallback, useEffect } from 'react'
import { Plus, Pencil, Trash2, XCircle, RefreshCw, X } from 'lucide-react'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import { usePoller } from '../hooks/usePoller.js'
import { getAPNs, createAPN, updateAPN, deleteAPN, getChargingRules, getSubscribers, getSubscriberRoutings } from '../api/client.js'
import ChargingRules from './ChargingRules.jsx'

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

const IP_VERSION_LABELS = { 0: 'IPv4', 1: 'IPv6', 2: 'IPv4v6', 3: 'IPv4 or v6' }

function fmtBps(v) {
  if (!v) return '—'
  if (v >= 1e9) return `${(v / 1e9).toFixed(1)} Gbps`
  if (v >= 1e6) return `${(v / 1e6).toFixed(1)} Mbps`
  if (v >= 1e3) return `${(v / 1e3).toFixed(1)} Kbps`
  return `${v} bps`
}

const CHIP_STYLE = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: 4,
  padding: '2px 8px 2px 10px',
  background: 'var(--accent)',
  color: '#fff',
  borderRadius: 12,
  fontSize: '0.78rem',
  fontWeight: 500,
}

const CHIP_BTN_STYLE = {
  background: 'none',
  border: 'none',
  cursor: 'pointer',
  color: '#fff',
  padding: 0,
  display: 'flex',
  alignItems: 'center',
  opacity: 0.8,
}

function ChargingRuleChips({ selectedIds, ruleList, onRemove }) {
  if (selectedIds.length === 0) return (
    <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', padding: '4px 0' }}>No charging rules selected</div>
  )
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 6 }}>
      {selectedIds.map(id => {
        const rule = ruleList.find(r => String(r.charging_rule_id) === String(id))
        return (
          <span key={id} style={CHIP_STYLE}>
            {rule ? rule.rule_name : `ID:${id}`}
            <button type="button" style={CHIP_BTN_STYLE} onClick={() => onRemove(id)} title="Remove">
              <X size={11} />
            </button>
          </span>
        )
      })}
    </div>
  )
}

function APNModal({ apn, onClose, onSaved, chargingRules }) {
  const toast = useToast()
  const isEdit = !!apn

  const initialRuleIds = isEdit && apn.charging_rule_list
    ? apn.charging_rule_list.split(',').map(s => s.trim()).filter(Boolean)
    : []

  const [form, setForm] = useState(apn ? {
    apn: apn.apn || '',
    ip_version: apn.ip_version != null ? apn.ip_version : 0,
    pgw_address: apn.pgw_address || '',
    sgw_address: apn.sgw_address || '',
    charging_characteristics: apn.charging_characteristics || '0800',
    apn_ambr_dl: apn.apn_ambr_dl || 0,
    apn_ambr_ul: apn.apn_ambr_ul || 0,
    qci: apn.qci || 9,
    arp_priority: apn.arp_priority || 1,
    arp_preemption_capability: apn.arp_preemption_capability || false,
    arp_preemption_vulnerability: apn.arp_preemption_vulnerability === true,
    nbiot: apn.nbiot || false,
    nidd_scef_id: apn.nidd_scef_id || '',
    nidd_scef_realm: apn.nidd_scef_realm || '',
    nidd_mechanism: apn.nidd_mechanism != null ? apn.nidd_mechanism : 0,
    nidd_rds: apn.nidd_rds != null ? apn.nidd_rds : 0,
    nidd_preferred_data_mode: apn.nidd_preferred_data_mode != null ? apn.nidd_preferred_data_mode : 0,
  } : {
    apn: '',
    ip_version: 0,
    pgw_address: '',
    sgw_address: '',
    charging_characteristics: '0800',
    apn_ambr_dl: 0,
    apn_ambr_ul: 0,
    qci: 9,
    arp_priority: 1,
    arp_preemption_capability: false,
    arp_preemption_vulnerability: false,
    nbiot: false,
    nidd_scef_id: '',
    nidd_scef_realm: '',
    nidd_mechanism: 0,
    nidd_rds: 0,
    nidd_preferred_data_mode: 0,
  })

  const [selectedRuleIds, setSelectedRuleIds] = useState(initialRuleIds)
  const [rulePickerValue, setRulePickerValue] = useState('')
  const [availableChargingRules, setAvailableChargingRules] = useState(chargingRules)
  const [loadingChargingRules, setLoadingChargingRules] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let active = true

    async function loadChargingRules() {
      setLoadingChargingRules(true)
      try {
        const data = await getChargingRules()
        if (active) setAvailableChargingRules(Array.isArray(data) ? data : [])
      } catch (err) {
        if (active) {
          setAvailableChargingRules(Array.isArray(chargingRules) ? chargingRules : [])
          toast.error('Charging rules', err.message || 'Failed to load charging rules')
        }
      } finally {
        if (active) setLoadingChargingRules(false)
      }
    }

    loadChargingRules()
    return () => { active = false }
  }, [chargingRules, toast])

  function set(k, v) {
    setForm(prev => ({ ...prev, [k]: v }))
  }

  function addRule() {
    if (!rulePickerValue) return
    if (selectedRuleIds.includes(rulePickerValue)) return
    setSelectedRuleIds(prev => [...prev, rulePickerValue])
    setRulePickerValue('')
  }

  function removeRule(id) {
    setSelectedRuleIds(prev => prev.filter(x => x !== id))
  }

  async function handleSubmit(e) {
    e.preventDefault()
    setSaving(true)
    try {
      const payload = {
        ...form,
        ip_version: Number(form.ip_version),
        apn_ambr_dl: Number(form.apn_ambr_dl),
        apn_ambr_ul: Number(form.apn_ambr_ul),
        qci: Number(form.qci),
        arp_priority: Number(form.arp_priority),
        nidd_mechanism: Number(form.nidd_mechanism),
        nidd_rds: Number(form.nidd_rds),
        nidd_preferred_data_mode: Number(form.nidd_preferred_data_mode),
        charging_rule_list: selectedRuleIds.join(',') || undefined,
      }
      if (isEdit) {
        await updateAPN(apn.apn_id, payload)
        toast.success('APN updated', form.apn)
      } else {
        await createAPN(payload)
        toast.success('APN created', form.apn)
      }
      onSaved()
      onClose()
    } catch (err) {
      toast.error(isEdit ? 'Update failed' : 'Create failed', err.message)
    } finally {
      setSaving(false)
    }
  }

  const availableRules = availableChargingRules.filter(r => !selectedRuleIds.includes(String(r.charging_rule_id)))

  return (
    <Modal title={isEdit ? 'Edit APN' : 'Add APN'} onClose={onClose} size="lg">
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}>
        <div className="modal-body">

          <div style={SECTION_STYLE}>Core</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">APN Name <span style={{ color: 'var(--danger)' }}>*</span></label>
              <input
                className="input mono"
                value={form.apn}
                onChange={e => set('apn', e.target.value)}
                placeholder="internet"
                required
              />
            </div>
            <div className="form-group">
              <label className="form-label">IP Version</label>
              <select className="select" value={form.ip_version} onChange={e => set('ip_version', e.target.value)}>
                <option value={0}>0 — IPv4</option>
                <option value={1}>1 — IPv6</option>
                <option value={2}>2 — IPv4v6</option>
                <option value={3}>3 — IPv4 or v6</option>
              </select>
            </div>
          </div>
          <div className="form-group">
            <label className="form-label">Charging Characteristics</label>
            <input className="input mono" value={form.charging_characteristics} onChange={e => set('charging_characteristics', e.target.value)} placeholder="0800" />
          </div>

          <div style={SECTION_STYLE}>Addresses</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">PGW Address</label>
              <input className="input mono" value={form.pgw_address} onChange={e => set('pgw_address', e.target.value)} placeholder="(optional)" />
            </div>
            <div className="form-group">
              <label className="form-label">SGW Address</label>
              <input className="input mono" value={form.sgw_address} onChange={e => set('sgw_address', e.target.value)} placeholder="(optional)" />
            </div>
          </div>

          <div style={SECTION_STYLE}>QoS</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">APN AMBR DL (bps)</label>
              <input className="input" type="number" min="0" value={form.apn_ambr_dl} onChange={e => set('apn_ambr_dl', e.target.value)} />
            </div>
            <div className="form-group">
              <label className="form-label">APN AMBR UL (bps)</label>
              <input className="input" type="number" min="0" value={form.apn_ambr_ul} onChange={e => set('apn_ambr_ul', e.target.value)} />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">QCI</label>
              <input className="input" type="number" min="1" max="9" value={form.qci} onChange={e => set('qci', e.target.value)} />
            </div>
            <div className="form-group">
              <label className="form-label">ARP Priority</label>
              <input className="input" type="number" min="1" max="15" value={form.arp_priority} onChange={e => set('arp_priority', e.target.value)} />
            </div>
          </div>
          <div className="form-row">
            <label className="checkbox-wrap">
              <input type="checkbox" checked={form.arp_preemption_capability} onChange={e => set('arp_preemption_capability', e.target.checked)} />
              <span className="form-label" style={{ margin: 0 }}>ARP Preemption Capability</span>
            </label>
            <label className="checkbox-wrap">
              <input type="checkbox" checked={form.arp_preemption_vulnerability} onChange={e => set('arp_preemption_vulnerability', e.target.checked)} />
              <span className="form-label" style={{ margin: 0 }}>ARP Preemption Vulnerability</span>
            </label>
          </div>

          <div style={SECTION_STYLE}>Charging Rules</div>
          <div className="form-group">
            <label className="form-label">Charging Rule List</label>
            <div style={{ display: 'flex', gap: 8 }}>
              <select
                className="select"
                value={rulePickerValue}
                onChange={e => setRulePickerValue(e.target.value)}
                style={{ flex: 1 }}
                disabled={loadingChargingRules}
              >
                <option value="">{loadingChargingRules ? 'Loading charging rules...' : '— Add a charging rule —'}</option>
                {availableRules.map(r => (
                  <option key={r.charging_rule_id} value={String(r.charging_rule_id)}>{r.rule_name}</option>
                ))}
              </select>
              <button
                type="button"
                className="btn btn-ghost"
                onClick={addRule}
                disabled={loadingChargingRules || !rulePickerValue}
              >
                Add
              </button>
            </div>
            <ChargingRuleChips selectedIds={selectedRuleIds} ruleList={availableChargingRules} onRemove={removeRule} />
          </div>

          <div style={SECTION_STYLE}>NB-IoT</div>
          <div className="form-row">
            <label className="checkbox-wrap">
              <input type="checkbox" checked={form.nbiot} onChange={e => set('nbiot', e.target.checked)} />
              <span className="form-label" style={{ margin: 0 }}>NB-IoT Enabled</span>
            </label>
          </div>
          {form.nbiot && (
            <>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">NIDD SCEF ID</label>
                  <input className="input mono" value={form.nidd_scef_id} onChange={e => set('nidd_scef_id', e.target.value)} placeholder="(optional)" />
                </div>
                <div className="form-group">
                  <label className="form-label">NIDD SCEF Realm</label>
                  <input className="input mono" value={form.nidd_scef_realm} onChange={e => set('nidd_scef_realm', e.target.value)} placeholder="(optional)" />
                </div>
              </div>
              <div className="form-row-3">
                <div className="form-group">
                  <label className="form-label">NIDD Mechanism</label>
                  <select className="select" value={form.nidd_mechanism} onChange={e => set('nidd_mechanism', e.target.value)}>
                    <option value={0}>0 — SGi</option>
                    <option value={1}>1 — SCEF</option>
                  </select>
                </div>
                <div className="form-group">
                  <label className="form-label">NIDD RDS</label>
                  <select className="select" value={form.nidd_rds} onChange={e => set('nidd_rds', e.target.value)}>
                    <option value={0}>0 — Disabled</option>
                    <option value={1}>1 — Enabled</option>
                  </select>
                </div>
                <div className="form-group">
                  <label className="form-label">NIDD Preferred Data Mode</label>
                  <select className="select" value={form.nidd_preferred_data_mode} onChange={e => set('nidd_preferred_data_mode', e.target.value)}>
                    <option value={0}>0</option>
                    <option value={1}>1</option>
                  </select>
                </div>
              </div>
            </>
          )}

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

export default function APNs() {
  const toast = useToast()
  const fetchFn = useCallback(getAPNs, [])
  const { data, error, loading, refresh } = usePoller(fetchFn, 30000)

  const [activeTab, setActiveTab] = useState('apns')
  const [modal, setModal] = useState(null)
  const [deleting, setDeleting] = useState(null)
  const [chargingRules, setChargingRules] = useState([])
  const [subscribers, setSubscribers] = useState([])
  const [subscriberRoutings, setSubscriberRoutings] = useState([])

  useEffect(() => {
    getChargingRules().then(d => setChargingRules(Array.isArray(d) ? d : [])).catch(() => {})
    getSubscribers().then(d => setSubscribers(Array.isArray(d?.items) ? d.items : [])).catch(() => {})
    getSubscriberRoutings().then(d => setSubscriberRoutings(Array.isArray(d) ? d : [])).catch(() => {})
  }, [])

  const apns = Array.isArray(data) ? data : []
  const sortedApns = [...apns].sort((a, b) => (a.apn || '').localeCompare(b.apn || ''))

  function apnUsageReason(apn) {
    const defaultSub = subscribers.find(row => Number(row.default_apn) === apn.apn_id)
    if (defaultSub) return `APN is still used as default APN by subscriber ${defaultSub.imsi}`
    const listSub = subscribers.find(row => String(row.apn_list || '').split(',').map(v => v.trim()).filter(Boolean).includes(String(apn.apn_id)))
    if (listSub) return `APN is still used by subscriber ${listSub.imsi}`
    const routing = subscriberRoutings.find(row => Number(row.apn_id) === apn.apn_id)
    if (routing) return `APN is still used by subscriber routing #${routing.subscriber_routing_id}`
    return ''
  }

  async function handleDelete(apn) {
    if (!window.confirm(`Delete APN "${apn.apn}"?`)) return
    setDeleting(apn.apn_id)
    try {
      await deleteAPN(apn.apn_id)
      toast.success('APN deleted', apn.apn)
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
        <span>Loading APNs...</span>
      </div>
    )
  }

  if (error && apns.length === 0) {
    return (
      <div className="error-state">
        <XCircle size={32} className="error-icon" />
        <div>{error}</div>
        <button className="btn btn-ghost mt-12" onClick={refresh}>
          <RefreshCw size={14} /> Retry
        </button>
      </div>
    )
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <div className="page-title">APNs</div>
          <div className="page-subtitle">{apns.length} APN{apns.length !== 1 ? 's' : ''} configured</div>
        </div>
        {activeTab === 'apns' && (
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
            <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add APN</button>
          </div>
        )}
      </div>

      <div style={{ display: 'flex', borderBottom: '1px solid var(--border)', marginBottom: 16 }}>
        <button style={TAB_STYLE(activeTab === 'apns')} onClick={() => setActiveTab('apns')}>APNs</button>
        <button style={TAB_STYLE(activeTab === 'charging')} onClick={() => setActiveTab('charging')}>Charging &amp; TFT</button>
      </div>

      {activeTab === 'apns' && (<>
        {apns.length === 0 ? (
          <div className="empty-state">No APNs configured yet.</div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>APN Name</th>
                  <th>IP Version</th><th>AMBR DL</th><th>AMBR UL</th>
                  <th>QCI</th><th>ARP</th><th>NB-IoT</th><th></th>
                </tr>
              </thead>
              <tbody>
                {sortedApns.map(apn => (
                  <tr key={apn.apn_id}>
                    <td className="mono" style={{ fontWeight: 600 }}>{apn.apn}</td>
                    <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                      {IP_VERSION_LABELS[apn.ip_version] ?? apn.ip_version ?? '—'}
                    </td>
                    <td className="mono" style={{ fontSize: '0.8rem' }}>{fmtBps(apn.apn_ambr_dl)}</td>
                    <td className="mono" style={{ fontSize: '0.8rem' }}>{fmtBps(apn.apn_ambr_ul)}</td>
                    <td className="mono">{apn.qci || '—'}</td>
                    <td className="mono">{apn.arp_priority || '—'}</td>
                    <td style={{ fontSize: '0.78rem' }}>
                      {apn.nbiot ? <span style={{ color: 'var(--success)', fontWeight: 600 }}>Yes</span>
                                 : <span style={{ color: 'var(--text-muted)' }}>No</span>}
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn-icon" title="Edit" onClick={() => setModal({ apn })}><Pencil size={13} /></button>
                        <button
                          className="btn-icon danger"
                          title={apnUsageReason(apn) || 'Delete'}
                          onClick={() => handleDelete(apn)}
                          disabled={deleting === apn.apn_id || !!apnUsageReason(apn)}
                        >
                          {deleting === apn.apn_id ? <Spinner size="sm" /> : <Trash2 size={13} />}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {modal === 'add' && <APNModal onClose={() => setModal(null)} onSaved={refresh} chargingRules={chargingRules} />}
        {modal && modal.apn && <APNModal apn={modal.apn} onClose={() => setModal(null)} onSaved={refresh} chargingRules={chargingRules} />}
      </>)}

      {activeTab === 'charging' && <ChargingRules compact />}
    </div>
  )
}
