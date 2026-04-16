import React, { useState, useCallback, useEffect } from 'react'
import { Plus, Pencil, Trash2, RefreshCw, Zap } from 'lucide-react'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import { useToast } from '../components/Toast.jsx'
import { usePoller } from '../hooks/usePoller.js'
import {
  getChargingRules, createChargingRule, updateChargingRule, deleteChargingRule,
  getTFTs, createTFT, updateTFT, deleteTFT,
  getAPNs,
} from '../api/client.js'

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

function fmtBps(v) {
  if (!v) return '—'
  if (v >= 1e9) return `${(v / 1e9).toFixed(1)} Gbps`
  if (v >= 1e6) return `${(v / 1e6).toFixed(1)} Mbps`
  if (v >= 1e3) return `${(v / 1e3).toFixed(1)} Kbps`
  return `${v} bps`
}

const TFT_DIR_LABELS = { 1: 'Downlink only', 2: 'Uplink only', 3: 'Bidirectional' }

// ---- Charging Rule Modal ----

const EMPTY_RULE = {
  rule_name: '',
  qci: 9,
  arp_priority: 1,
  arp_preemption_capability: false,
  arp_preemption_vulnerability: false,
  mbr_dl: 0,
  mbr_ul: 0,
  gbr_dl: 0,
  gbr_ul: 0,
  tft_group_id: '',
  precedence: '',
  rating_group: '',
}

function ChargingRuleModal({ rule, onClose, onSaved, tfts = [] }) {
  const toast = useToast()
  const isEdit = !!rule
  const [availableTfts, setAvailableTfts] = useState(tfts)
  const [loadingTfts, setLoadingTfts] = useState(true)
  const [form, setForm] = useState(rule ? {
    rule_name: rule.rule_name || '',
    qci: rule.qci || 9,
    arp_priority: rule.arp_priority || 1,
    arp_preemption_capability: rule.arp_preemption_capability || false,
    arp_preemption_vulnerability: rule.arp_preemption_vulnerability === true,
    mbr_dl: rule.mbr_dl || 0,
    mbr_ul: rule.mbr_ul || 0,
    gbr_dl: rule.gbr_dl || 0,
    gbr_ul: rule.gbr_ul || 0,
    tft_group_id: rule.tft_group_id != null ? String(rule.tft_group_id) : '',
    precedence: rule.precedence != null ? String(rule.precedence) : '',
    rating_group: rule.rating_group != null ? String(rule.rating_group) : '',
  } : { ...EMPTY_RULE })
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let active = true

    async function loadTfts() {
      setLoadingTfts(true)
      try {
        const data = await getTFTs()
        if (active) setAvailableTfts(Array.isArray(data) ? data : [])
      } catch (err) {
        if (active) {
          setAvailableTfts(Array.isArray(tfts) ? tfts : [])
          toast.error('TFT entries', err.message || 'Failed to load TFT entries')
        }
      } finally {
        if (active) setLoadingTfts(false)
      }
    }

    loadTfts()
    return () => { active = false }
  }, [tfts, toast])

  // Deduplicated, sorted list of Group IDs from existing TFT entries
  const tftGroupIds = [...new Set(availableTfts.map(t => t.tft_group_id))].sort((a, b) => a - b)

  function set(k, v) {
    setForm(prev => ({ ...prev, [k]: v }))
  }

  async function handleSubmit(e) {
    e.preventDefault()
    setSaving(true)
    try {
      const payload = {
        rule_name: form.rule_name,
        qci: Number(form.qci),
        arp_priority: Number(form.arp_priority),
        arp_preemption_capability: form.arp_preemption_capability,
        arp_preemption_vulnerability: form.arp_preemption_vulnerability,
        mbr_dl: Number(form.mbr_dl),
        mbr_ul: Number(form.mbr_ul),
        gbr_dl: Number(form.gbr_dl),
        gbr_ul: Number(form.gbr_ul),
        ...(form.tft_group_id !== '' && { tft_group_id: Number(form.tft_group_id) }),
        ...(form.precedence !== '' && { precedence: Number(form.precedence) }),
        ...(form.rating_group !== '' && { rating_group: Number(form.rating_group) }),
      }
      if (isEdit) {
        await updateChargingRule(rule.charging_rule_id, payload)
        toast.success('Charging rule updated', form.rule_name)
      } else {
        await createChargingRule(payload)
        toast.success('Charging rule created', form.rule_name)
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
    <Modal title={isEdit ? 'Edit Charging Rule' : 'Add Charging Rule'} onClose={onClose} size="lg">
      <form onSubmit={handleSubmit}>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">Rule Name <span style={{ color: 'var(--danger)' }}>*</span></label>
            <input
              className="input mono"
              value={form.rule_name}
              onChange={e => set('rule_name', e.target.value)}
              placeholder="rule_internet"
              required
            />
          </div>

          <div style={SECTION_STYLE}>QoS</div>
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

          <div style={SECTION_STYLE}>Bitrates</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">MBR DL (bps)</label>
              <input className="input" type="number" min="0" value={form.mbr_dl} onChange={e => set('mbr_dl', e.target.value)} />
            </div>
            <div className="form-group">
              <label className="form-label">MBR UL (bps)</label>
              <input className="input" type="number" min="0" value={form.mbr_ul} onChange={e => set('mbr_ul', e.target.value)} />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">GBR DL (bps)</label>
              <input className="input" type="number" min="0" value={form.gbr_dl} onChange={e => set('gbr_dl', e.target.value)} />
            </div>
            <div className="form-group">
              <label className="form-label">GBR UL (bps)</label>
              <input className="input" type="number" min="0" value={form.gbr_ul} onChange={e => set('gbr_ul', e.target.value)} />
            </div>
          </div>

          <div style={SECTION_STYLE}>Identifiers</div>
          <div className="form-row-3">
            <div className="form-group">
              <label className="form-label">TFT Group ID</label>
              {tftGroupIds.length > 0 ? (
                <select className="select" value={form.tft_group_id} onChange={e => set('tft_group_id', e.target.value)} disabled={loadingTfts}>
                  <option value="">— None —</option>
                  {tftGroupIds.map(id => (
                    <option key={id} value={String(id)}>Group {id}</option>
                  ))}
                </select>
              ) : (
                <input className="input" type="number" min="0" value={form.tft_group_id} onChange={e => set('tft_group_id', e.target.value)} placeholder="(no TFT groups defined)" />
              )}
            </div>
            <div className="form-group">
              <label className="form-label">Precedence</label>
              <input className="input" type="number" min="0" value={form.precedence} onChange={e => set('precedence', e.target.value)} placeholder="(optional)" />
            </div>
            <div className="form-group">
              <label className="form-label">Rating Group</label>
              <input className="input" type="number" min="0" value={form.rating_group} onChange={e => set('rating_group', e.target.value)} placeholder="(optional)" />
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

// ---- TFT Modal ----

const EMPTY_TFT = {
  tft_group_id: '',
  tft_string: '',
  direction: 1,
}

function TFTModal({ tft, onClose, onSaved }) {
  const toast = useToast()
  const isEdit = !!tft
  const [form, setForm] = useState(tft ? {
    tft_group_id: tft.tft_group_id != null ? String(tft.tft_group_id) : '',
    tft_string: tft.tft_string || '',
    direction: tft.direction != null ? tft.direction : 1,
  } : { ...EMPTY_TFT })
  const [saving, setSaving] = useState(false)

  function set(k, v) {
    setForm(prev => ({ ...prev, [k]: v }))
  }

  async function handleSubmit(e) {
    e.preventDefault()
    setSaving(true)
    try {
      const payload = {
        tft_group_id: Number(form.tft_group_id),
        tft_string: form.tft_string,
        direction: Number(form.direction),
      }
      if (isEdit) {
        await updateTFT(tft.tft_id, payload)
        toast.success('TFT updated', form.tft_string)
      } else {
        await createTFT(payload)
        toast.success('TFT created', form.tft_string)
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
    <Modal title={isEdit ? 'Edit TFT Entry' : 'Add TFT Entry'} onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="modal-body">
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">TFT Group ID <span style={{ color: 'var(--danger)' }}>*</span></label>
              <input
                className="input"
                type="number"
                min="0"
                value={form.tft_group_id}
                onChange={e => set('tft_group_id', e.target.value)}
                placeholder="1"
                required
              />
            </div>
            <div className="form-group">
              <label className="form-label">Direction</label>
              <select className="select" value={form.direction} onChange={e => set('direction', e.target.value)}>
                <option value={1}>1 — Downlink only</option>
                <option value={2}>2 — Uplink only</option>
                <option value={3}>3 — Bidirectional</option>
              </select>
            </div>
          </div>
          <div className="form-group">
            <label className="form-label">TFT String <span style={{ color: 'var(--danger)' }}>*</span></label>
            <input
              className="input mono"
              value={form.tft_string}
              onChange={e => set('tft_string', e.target.value)}
              placeholder="permit out ip from any to 0.0.0.0/0"
              required
            />
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

// ---- Main Page ----

export default function ChargingRules({ compact = false }) {
  const toast = useToast()
  const fetchRulesFn = useCallback(getChargingRules, [])
  const fetchTFTsFn = useCallback(getTFTs, [])
  const { data: rulesData, loading: rulesLoading, error: rulesError, refresh: refreshRules } = usePoller(fetchRulesFn, 30000)
  const { data: tftsData, loading: tftsLoading, refresh: refreshTFTs } = usePoller(fetchTFTsFn, 30000)

  const [ruleModal, setRuleModal] = useState(null)
  const [tftModal, setTftModal] = useState(null)
  const [delRuleConfirm, setDelRuleConfirm] = useState(null)
  const [delTFTConfirm, setDelTFTConfirm] = useState(null)
  const [deletingRule, setDeletingRule] = useState(null)
  const [deletingTFT, setDeletingTFT] = useState(null)
  const [apns, setAPNs] = useState([])

  const rules = Array.isArray(rulesData) ? [...rulesData].sort((a, b) => (a.rule_name || '').localeCompare(b.rule_name || '')) : []
  const tfts = Array.isArray(tftsData) ? [...tftsData].sort((a, b) => (a.tft_group_id ?? 0) - (b.tft_group_id ?? 0) || (a.rule_name || '').localeCompare(b.rule_name || '')) : []

  useEffect(() => {
    getAPNs().then(d => setAPNs(Array.isArray(d) ? d : [])).catch(() => {})
  }, [])

  function ruleUsageReason(rule) {
    const apn = apns.find(row => String(row.charging_rule_list || '').split(',').map(v => v.trim()).filter(Boolean).includes(String(rule.charging_rule_id)))
    if (apn) return `Charging rule is still used by APN ${apn.apn}`
    return ''
  }

  function tftUsageReason(tft) {
    const rule = rules.find(row => Number(row.tft_group_id) === Number(tft.tft_group_id))
    if (rule) return `TFT group is still used by charging rule ${rule.rule_name}`
    return ''
  }

  function refreshAll() {
    refreshRules()
    refreshTFTs()
  }

  async function handleDeleteRule(rule) {
    setDeletingRule(rule.charging_rule_id)
    try {
      await deleteChargingRule(rule.charging_rule_id)
      toast.success('Deleted', `Charging rule "${rule.rule_name}" deleted`)
      setDelRuleConfirm(null)
      refreshRules()
    } catch (err) {
      toast.error('Delete failed', err.message)
    } finally {
      setDeletingRule(null)
    }
  }

  async function handleDeleteTFT(tft) {
    setDeletingTFT(tft.tft_id)
    try {
      await deleteTFT(tft.tft_id)
      toast.success('Deleted', 'TFT entry deleted')
      setDelTFTConfirm(null)
      refreshTFTs()
    } catch (err) {
      toast.error('Delete failed', err.message)
    } finally {
      setDeletingTFT(null)
    }
  }

  if (rulesLoading && rules.length === 0) {
    return (
      <div className="loading-center">
        <Spinner size="lg" />
        <span>Loading charging rules...</span>
      </div>
    )
  }

  return (
    <div>
      {!compact && (
        <div className="page-header">
          <div>
            <div className="page-title">Charging &amp; TFT</div>
            <div className="page-subtitle">{rules.length} charging rule{rules.length !== 1 ? 's' : ''}, {tfts.length} TFT entr{tfts.length !== 1 ? 'ies' : 'y'}</div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-ghost" onClick={refreshAll}><RefreshCw size={14} /> Refresh</button>
          </div>
        </div>
      )}
      {compact && (
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginBottom: 8 }}>
          <button className="btn btn-ghost" onClick={refreshAll}><RefreshCw size={14} /> Refresh</button>
        </div>
      )}

      {/* Charging Rules Section */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8, marginTop: 8 }}>
        <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>Charging Rules</div>
        <button className="btn btn-primary" onClick={() => setRuleModal('add')}>
          <Plus size={14} /> Add Rule
        </button>
      </div>

      {rulesError && rules.length === 0 ? (
        <div className="empty-state">{rulesError}</div>
      ) : rules.length === 0 ? (
        <div className="empty-state">No charging rules configured yet.</div>
      ) : (
        <div className="table-container" style={{ marginBottom: 32 }}>
          <table>
            <thead>
              <tr>
                <th>Rule Name</th>
                <th>QCI</th>
                <th>MBR DL</th>
                <th>MBR UL</th>
                <th>GBR DL</th>
                <th>GBR UL</th>
                <th>TFT Group</th>
                <th>Precedence</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {rules.map(rule => (
                <tr key={rule.charging_rule_id}>
                  <td className="mono" style={{ fontWeight: 600 }}>{rule.rule_name}</td>
                  <td className="mono">{rule.qci || '—'}</td>
                  <td className="mono" style={{ fontSize: '0.78rem' }}>{fmtBps(rule.mbr_dl)}</td>
                  <td className="mono" style={{ fontSize: '0.78rem' }}>{fmtBps(rule.mbr_ul)}</td>
                  <td className="mono" style={{ fontSize: '0.78rem' }}>{fmtBps(rule.gbr_dl)}</td>
                  <td className="mono" style={{ fontSize: '0.78rem' }}>{fmtBps(rule.gbr_ul)}</td>
                  <td className="mono" style={{ color: 'var(--text-muted)' }}>{rule.tft_group_id ?? '—'}</td>
                  <td className="mono" style={{ color: 'var(--text-muted)' }}>{rule.precedence ?? '—'}</td>
                  <td>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button className="btn-icon" title="Edit" onClick={() => setRuleModal({ rule })}>
                        <Pencil size={13} />
                      </button>
                      <button
                        className="btn-icon danger"
                        title={ruleUsageReason(rule) || 'Delete'}
                        onClick={() => setDelRuleConfirm(rule)}
                        disabled={deletingRule === rule.charging_rule_id || !!ruleUsageReason(rule)}
                      >
                        {deletingRule === rule.charging_rule_id ? <Spinner size="sm" /> : <Trash2 size={13} />}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* TFT Section */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8, marginTop: 8 }}>
        <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>TFT Entries</div>
        <button className="btn btn-primary" onClick={() => setTftModal('add')}>
          <Plus size={14} /> Add TFT
        </button>
      </div>

      {tftsLoading && tfts.length === 0 ? (
        <div className="loading-center" style={{ padding: 24 }}><Spinner size="sm" /><span>Loading TFTs...</span></div>
      ) : tfts.length === 0 ? (
        <div className="empty-state">No TFT entries configured yet.</div>
      ) : (
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Group ID</th>
                <th>Rule String</th>
                <th>Direction</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {tfts.map(tft => (
                <tr key={tft.tft_id}>
                  <td className="mono" style={{ color: 'var(--text-muted)' }}>{tft.tft_group_id}</td>
                  <td className="mono" style={{ fontSize: '0.82rem' }}>{tft.tft_string || '—'}</td>
                  <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                    {TFT_DIR_LABELS[tft.direction] ?? tft.direction ?? '—'}
                  </td>
                  <td>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button className="btn-icon" title="Edit" onClick={() => setTftModal({ tft })}>
                        <Pencil size={13} />
                      </button>
                      <button
                        className="btn-icon danger"
                        title={tftUsageReason(tft) || 'Delete'}
                        onClick={() => setDelTFTConfirm(tft)}
                        disabled={deletingTFT === tft.tft_id || !!tftUsageReason(tft)}
                      >
                        {deletingTFT === tft.tft_id ? <Spinner size="sm" /> : <Trash2 size={13} />}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Charging Rule Modals */}
      {ruleModal === 'add' && (
        <ChargingRuleModal onClose={() => setRuleModal(null)} onSaved={refreshRules} tfts={tfts} />
      )}
      {ruleModal && ruleModal.rule && (
        <ChargingRuleModal rule={ruleModal.rule} onClose={() => setRuleModal(null)} onSaved={refreshRules} tfts={tfts} />
      )}

      {/* TFT Modals */}
      {tftModal === 'add' && (
        <TFTModal onClose={() => setTftModal(null)} onSaved={refreshTFTs} />
      )}
      {tftModal && tftModal.tft && (
        <TFTModal tft={tftModal.tft} onClose={() => setTftModal(null)} onSaved={refreshTFTs} />
      )}

      {/* Delete Confirmations */}
      {delRuleConfirm && (
        <Modal title="Delete Charging Rule" onClose={() => setDelRuleConfirm(null)}>
          <div className="modal-body">
            <p>Delete charging rule <strong>{delRuleConfirm.rule_name}</strong>? This cannot be undone.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setDelRuleConfirm(null)}>Cancel</button>
            <button
              className="btn btn-danger"
              onClick={() => handleDeleteRule(delRuleConfirm)}
              disabled={deletingRule === delRuleConfirm.charging_rule_id}
            >
              {deletingRule === delRuleConfirm.charging_rule_id ? <Spinner size="sm" /> : 'Delete'}
            </button>
          </div>
        </Modal>
      )}

      {delTFTConfirm && (
        <Modal title="Delete TFT Entry" onClose={() => setDelTFTConfirm(null)}>
          <div className="modal-body">
            <p>Delete TFT entry <strong>{delTFTConfirm.tft_string}</strong>? This cannot be undone.</p>
          </div>
          <div className="modal-footer">
            <button className="btn btn-ghost" onClick={() => setDelTFTConfirm(null)}>Cancel</button>
            <button
              className="btn btn-danger"
              onClick={() => handleDeleteTFT(delTFTConfirm)}
              disabled={deletingTFT === delTFTConfirm.tft_id}
            >
              {deletingTFT === delTFTConfirm.tft_id ? <Spinner size="sm" /> : 'Delete'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
