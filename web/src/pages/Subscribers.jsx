import React, { useState, useEffect } from 'react'
import { Plus, Pencil, Trash2, Search, XCircle, RefreshCw, X, ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import { useSort } from '../hooks/useSort.js'
import { useInfiniteScroll } from '../hooks/useInfiniteScroll.js'
import Spinner from '../components/Spinner.jsx'
import Modal from '../components/Modal.jsx'
import Badge from '../components/Badge.jsx'
import { useToast } from '../components/Toast.jsx'
import {
  getSubscribers, createSubscriber, updateSubscriber, deleteSubscriber,
  getAUCs, getAPNs, getEIRHistory, getIMSSubscribers, getSubscriberAttributes, getSubscriberRoutings,
} from '../api/client.js'
import SubscriberAttributes from './SubscriberAttributes.jsx'
import SubscriberRoutings from './SubscriberRoutings.jsx'

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

const NAM_LABELS = { 0: 'PACKET_AND_CIRCUIT (PS+CS)', 2: 'ONLY_PACKET (PS Only)' }

const RAT_BITS = [
  { value: 1,   label: 'UTRAN (3G)' },
  { value: 2,   label: 'GERAN (2G)' },
  { value: 4,   label: 'GAN' },
  { value: 8,   label: 'I-HSPA-Evo' },
  { value: 16,  label: 'E-UTRAN (4G)' },
  { value: 32,  label: 'HO Non-3GPP' },
  { value: 64,  label: 'NB-IoT' },
  { value: 256, label: 'NR as Secondary RAT (NSA)' },
  { value: 512, label: 'NR in Unlicensed Spectrum' },
  { value: 1024, label: 'NR in Licensed Spectrum (SA)' },
]

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

function APNChips({ selectedIds, apnList, onRemove }) {
  if (selectedIds.length === 0) return (
    <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', padding: '4px 0' }}>No APNs selected</div>
  )
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 6 }}>
      {selectedIds.map((id, idx) => {
        const apn = apnList.find(a => String(a.apn_id) === String(id))
        return (
          <span key={id} style={CHIP_STYLE}>
            {idx === 0 && <span style={{ fontSize: '0.65rem', opacity: 0.85, marginRight: 2 }}>[default]</span>}
            {apn ? apn.apn : `ID:${id}`}
            <button type="button" style={CHIP_BTN_STYLE} onClick={() => onRemove(id)} title="Remove">
              <X size={11} />
            </button>
          </span>
        )
      })}
    </div>
  )
}

const SST_LABELS = {
  1: '1 — eMBB (enhanced Mobile Broadband)',
  2: '2 — URLLC (Ultra-Reliable Low Latency)',
  3: '3 — MIoT (Massive IoT)',
  4: '4 — V2X (Vehicle-to-Everything)',
}

function parseNSSAI(raw) {
  if (!raw) return []
  try { return JSON.parse(raw) } catch { return [] }
}

function serializeNSSAI(slices) {
  if (!slices || slices.length === 0) return ''
  return JSON.stringify(slices.map(s => {
    const o = { sst: Number(s.sst) }
    if (s.sd && s.sd.trim()) o.sd = s.sd.trim()
    return o
  }))
}

function NSSAIEditor({ value, onChange }) {
  const slices = parseNSSAI(value)
  const [newSst, setNewSst] = useState('1')
  const [newSd, setNewSd]   = useState('')

  function addSlice() {
    const updated = [...slices, { sst: Number(newSst), ...(newSd.trim() ? { sd: newSd.trim() } : {}) }]
    onChange(serializeNSSAI(updated))
    setNewSd('')
  }

  function removeSlice(i) {
    const updated = slices.filter((_, idx) => idx !== i)
    onChange(serializeNSSAI(updated))
  }

  return (
    <div className="form-group">
      <label className="form-label">NSSAI — Network Slice Selection</label>
      {slices.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 4, marginBottom: 8 }}>
          {slices.map((s, i) => (
            <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 8px', background: 'var(--bg-elevated)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-subtle)' }}>
              <span style={{ fontSize: '0.78rem' }}>
                <span style={{ fontWeight: 600 }}>SST {s.sst}</span>
                {s.sst in SST_LABELS && <span style={{ color: 'var(--text-muted)', marginLeft: 4 }}>({SST_LABELS[s.sst].split(' — ')[1]})</span>}
                {s.sd && <span className="mono" style={{ marginLeft: 8, color: 'var(--text-muted)' }}>SD: {s.sd}</span>}
              </span>
              <button type="button" className="btn-icon danger" style={{ marginLeft: 'auto', padding: '1px 4px' }} onClick={() => removeSlice(i)}>
                <X size={11} />
              </button>
            </div>
          ))}
        </div>
      )}
      <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
        <div className="form-group" style={{ margin: 0, flex: '0 0 260px' }}>
          <label className="form-label" style={{ fontSize: '0.72rem' }}>Slice/Service Type (SST)</label>
          <select className="select" value={newSst} onChange={e => setNewSst(e.target.value)}>
            {Object.entries(SST_LABELS).map(([v, label]) => (
              <option key={v} value={v}>{label}</option>
            ))}
          </select>
        </div>
        <div className="form-group" style={{ margin: 0, flex: 1 }}>
          <label className="form-label" style={{ fontSize: '0.72rem' }}>Slice Differentiator (SD, optional hex)</label>
          <input className="input mono" value={newSd} onChange={e => setNewSd(e.target.value)} placeholder="e.g. 000001" maxLength={6} />
        </div>
        <button type="button" className="btn btn-ghost" style={{ flexShrink: 0 }} onClick={addSlice}>
          <Plus size={13} /> Add Slice
        </button>
      </div>
      {slices.length === 0 && (
        <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginTop: 4 }}>
          No slices configured — UE will use network default (SST=1 eMBB).
        </div>
      )}
    </div>
  )
}

function SubscriberModal({ sub, onClose, onSaved, aucList, apnList }) {
  const toast = useToast()
  const isEdit = !!sub

  // Parse initial APN chips from stored apn_list CSV
  const initialApnIds = isEdit && sub.apn_list
    ? sub.apn_list.split(',').map(s => s.trim()).filter(Boolean)
    : []

  const [form, setForm] = useState(isEdit ? {
    imsi: sub.imsi || '',
    auc_id: sub.auc_id != null ? String(sub.auc_id) : '',
    msisdn: sub.msisdn || '',
    enabled: sub.enabled !== false,
    roaming_enabled: sub.roaming_enabled !== false,
    ue_ambr_dl: sub.ue_ambr_dl || 0,
    ue_ambr_ul: sub.ue_ambr_ul || 0,
    nam: sub.nam != null ? sub.nam : 0,
    subscribed_rau_tau_timer: sub.subscribed_rau_tau_timer != null ? sub.subscribed_rau_tau_timer : 600,
    access_restriction_data: sub.access_restriction_data != null ? Number(sub.access_restriction_data) : 0,
    nssai: sub.nssai || '',
  } : {
    imsi: '',
    auc_id: '',
    msisdn: '',
    enabled: true,
    roaming_enabled: true,
    ue_ambr_dl: 0,
    ue_ambr_ul: 0,
    nam: 0,
    subscribed_rau_tau_timer: 600,
    access_restriction_data: 0,
    nssai: '',
  })

  const [selectedApnIds, setSelectedApnIds] = useState(initialApnIds)
  const [apnPickerValue, setApnPickerValue] = useState('')
  const [availableAucs, setAvailableAucs] = useState(aucList)
  const [availableApns, setAvailableApns] = useState(apnList)
  const [loadingAucs, setLoadingAucs] = useState(true)
  const [loadingApns, setLoadingApns] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let active = true

    async function loadAucs() {
      setLoadingAucs(true)
      try {
        const data = await getAUCs()
        if (active) setAvailableAucs(Array.isArray(data?.items) ? data.items : [])
      } catch (err) {
        if (active) {
          setAvailableAucs(Array.isArray(aucList) ? aucList : [])
          toast.error('SIM cards / AUC', err.message || 'Failed to load AUC entries')
        }
      } finally {
        if (active) setLoadingAucs(false)
      }
    }

    async function loadApns() {
      setLoadingApns(true)
      try {
        const data = await getAPNs()
        if (active) setAvailableApns(Array.isArray(data) ? data : [])
      } catch (err) {
        if (active) {
          setAvailableApns(Array.isArray(apnList) ? apnList : [])
          toast.error('APNs', err.message || 'Failed to load APNs')
        }
      } finally {
        if (active) setLoadingApns(false)
      }
    }

    loadAucs()
    loadApns()
    return () => { active = false }
  }, [aucList, apnList, toast])

  function set(k, v) {
    setForm(prev => ({ ...prev, [k]: v }))
  }

  function handleAucChange(e) {
    const aucId = e.target.value
    set('auc_id', aucId)
    if (aucId) {
      const auc = availableAucs.find(a => String(a.auc_id) === aucId)
      if (auc && auc.imsi) {
        set('imsi', auc.imsi)
      }
    }
  }

  function addApn() {
    if (!apnPickerValue) return
    if (selectedApnIds.includes(apnPickerValue)) return
    setSelectedApnIds(prev => [...prev, apnPickerValue])
    setApnPickerValue('')
  }

  function removeApn(id) {
    setSelectedApnIds(prev => prev.filter(x => x !== id))
  }

  function toggleRat(bitValue) {
    const current = Number(form.access_restriction_data) || 0
    const newVal = (current & bitValue) ? (current & ~bitValue) : (current | bitValue)
    set('access_restriction_data', newVal)
  }

  async function handleSubmit(e) {
    e.preventDefault()
    if (!form.auc_id) {
      toast.error('Validation', 'IMSI / AUC is required')
      return
    }
    setSaving(true)
    try {
      const apn_list = selectedApnIds.join(',')
      const default_apn = selectedApnIds.length > 0 ? Number(selectedApnIds[0]) : 0
      const payload = {
        imsi: form.imsi,
        auc_id: Number(form.auc_id),
        msisdn: form.msisdn || undefined,
        enabled: form.enabled,
        roaming_enabled: form.roaming_enabled,
        apn_list,
        default_apn,
        ue_ambr_dl: Number(form.ue_ambr_dl),
        ue_ambr_ul: Number(form.ue_ambr_ul),
        nam: Number(form.nam),
        subscribed_rau_tau_timer: Number(form.subscribed_rau_tau_timer),
        access_restriction_data: Number(form.access_restriction_data) || undefined,
        nssai: form.nssai || undefined,
      }
      if (isEdit) {
        await updateSubscriber(sub.subscriber_id, payload)
        toast.success('Subscriber updated', form.imsi)
      } else {
        await createSubscriber(payload)
        toast.success('Subscriber created', form.imsi)
      }
      onSaved()
      onClose()
    } catch (err) {
      toast.error(isEdit ? 'Update failed' : 'Create failed', err.message)
    } finally {
      setSaving(false)
    }
  }

  const ard = Number(form.access_restriction_data) || 0
  const selectableApns = availableApns.filter(a => !selectedApnIds.includes(String(a.apn_id)))

  return (
    <Modal title={isEdit ? 'Edit Subscriber' : 'Add Subscriber'} onClose={onClose} size="lg">
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}>
        <div className="modal-body">

          <div style={SECTION_STYLE}>Identity</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">IMSI / ICCID <span style={{ color: 'var(--danger)' }}>*</span></label>
              <select
                className="select"
                value={form.auc_id}
                onChange={handleAucChange}
                required
                disabled={loadingAucs}
              >
                <option value="">{loadingAucs ? 'Loading SIM Cards...' : '— Select SIM Card —'}</option>
                {availableAucs.map(a => (
                  <option key={a.auc_id} value={String(a.auc_id)}>
                    {a.imsi} {a.iccid ? `[${a.iccid}]` : ''}
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label className="form-label">MSISDN</label>
              <input
                className="input mono"
                value={form.msisdn}
                onChange={e => set('msisdn', e.target.value)}
                placeholder="14155550100"
              />
            </div>
          </div>
          <div className="form-row">
            <div className="form-group" style={{ alignSelf: 'center', paddingTop: 20 }}>
              <label className="checkbox-wrap">
                <input type="checkbox" checked={form.enabled} onChange={e => set('enabled', e.target.checked)} />
                <span className="form-label" style={{ margin: 0 }}>Enabled</span>
              </label>
            </div>
            <div className="form-group" style={{ alignSelf: 'center', paddingTop: 20 }}>
              <label className="checkbox-wrap">
                <input type="checkbox" checked={form.roaming_enabled} onChange={e => set('roaming_enabled', e.target.checked)} />
                <span className="form-label" style={{ margin: 0 }}>Roaming Enabled</span>
              </label>
            </div>
          </div>

          <div style={SECTION_STYLE}>APN Configuration</div>
          <div className="form-group">
            <label className="form-label">APNs</label>
            <div style={{ display: 'flex', gap: 8 }}>
              <select
                className="select"
                value={apnPickerValue}
                onChange={e => setApnPickerValue(e.target.value)}
                style={{ flex: 1 }}
                disabled={loadingApns}
              >
                <option value="">{loadingApns ? 'Loading APNs...' : '— Add an APN —'}</option>
                {selectableApns.map(a => (
                  <option key={a.apn_id} value={String(a.apn_id)}>{a.apn}</option>
                ))}
              </select>
              <button
                type="button"
                className="btn btn-ghost"
                onClick={addApn}
                disabled={loadingApns || !apnPickerValue}
              >
                Add
              </button>
            </div>
            <APNChips selectedIds={selectedApnIds} apnList={availableApns} onRemove={removeApn} />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">UE AMBR DL (bps)</label>
              <input className="input" type="number" min="0" value={form.ue_ambr_dl} onChange={e => set('ue_ambr_dl', e.target.value)} />
            </div>
            <div className="form-group">
              <label className="form-label">UE AMBR UL (bps)</label>
              <input className="input" type="number" min="0" value={form.ue_ambr_ul} onChange={e => set('ue_ambr_ul', e.target.value)} />
            </div>
          </div>

          <div style={SECTION_STYLE}>Network Settings</div>
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">NAM (Network Access Mode)</label>
              <select className="select" value={form.nam} onChange={e => set('nam', Number(e.target.value))}>
                <option value={0}>0 — PACKET_AND_CIRCUIT (PS+CS)</option>
                <option value={2}>2 — ONLY_PACKET (PS Only)</option>
              </select>
            </div>
            <div className="form-group">
              <label className="form-label">Subscribed RAU/TAU Timer (s)</label>
              <input className="input" type="number" min="0" value={form.subscribed_rau_tau_timer} onChange={e => set('subscribed_rau_tau_timer', e.target.value)} />
            </div>
          </div>

          <div style={SECTION_STYLE}>Access Restrictions</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '6px 16px' }}>
            {RAT_BITS.map(({ value, label }) => (
              <label key={value} className="checkbox-wrap" style={{ marginBottom: 4 }}>
                <input
                  type="checkbox"
                  checked={!!(ard & value)}
                  onChange={() => toggleRat(value)}
                />
                <span style={{ fontSize: '0.82rem' }}>{label}</span>
              </label>
            ))}
          </div>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: 4 }}>
            Raw value: {ard}
          </div>

          <div style={SECTION_STYLE}>5G / NR</div>
          <NSSAIEditor value={form.nssai} onChange={v => set('nssai', v)} />

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

export default function Subscribers() {
  const toast = useToast()
  const [activeTab, setActiveTab] = useState('subscribers')
  const [search, setSearch] = useState('')
  const [modal, setModal] = useState(null)
  const [deleting, setDeleting] = useState(null)
  const [aucList, setAucList] = useState([])
  const [apnList, setApnList] = useState([])
  const [eirHistory, setEirHistory] = useState([])
  const [imsSubscribers, setIMSSubscribers] = useState([])
  const [subscriberAttributes, setSubscriberAttributes] = useState([])
  const [subscriberRoutings, setSubscriberRoutings] = useState([])

  const { items, total, loading, loadingMore, error, sentinelRef, refresh } = useInfiniteScroll(getSubscribers, search)

  useEffect(() => {
    getAUCs().then(d => setAucList(Array.isArray(d?.items) ? d.items : [])).catch(() => {})
    getAPNs().then(d => setApnList(Array.isArray(d) ? d : [])).catch(() => {})
    getEIRHistory().then(d => setEirHistory(Array.isArray(d) ? d : [])).catch(() => {})
    getIMSSubscribers().then(d => setIMSSubscribers(Array.isArray(d?.items) ? d.items : Array.isArray(d) ? d : [])).catch(() => {})
    getSubscriberAttributes().then(d => setSubscriberAttributes(Array.isArray(d) ? d : [])).catch(() => {})
    getSubscriberRoutings().then(d => setSubscriberRoutings(Array.isArray(d) ? d : [])).catch(() => {})
  }, [])

  const { sorted: sortedSubs, sortKey, sortDir, handleSort } = useSort(items, 'imsi')

  function SortIcon({ col }) {
    if (sortKey !== col) return <span className="sort-icon"><ChevronsUpDown size={11} /></span>
    return <span className="sort-icon">{sortDir === 'asc' ? <ChevronUp size={11} /> : <ChevronDown size={11} />}</span>
  }

  function getDeviceInfo(imsi) {
    const h = eirHistory.find(e => e.imsi === imsi)
    if (!h) return 'Unknown'
    const parts = [h.imei, [h.make, h.model].filter(Boolean).join(' ')].filter(Boolean)
    return parts.join(' ')
  }

  function subscriberUsageReason(sub) {
    const imsSub = imsSubscribers.find(row => String(row.imsi) === String(sub.imsi))
    if (imsSub) return `In use by IMS subscriber ${imsSub.impi || imsSub.imsi}`

    const attrs = subscriberAttributes.find(row => String(row.subscriber_id) === String(sub.subscriber_id))
    if (attrs) return `In use by subscriber attributes for ${sub.imsi}`

    const routing = subscriberRoutings.find(row => String(row.subscriber_id) === String(sub.subscriber_id))
    if (routing) return `In use by subscriber routing for ${sub.imsi}`

    return ''
  }

  async function handleDelete(sub) {
    if (!window.confirm(`Delete subscriber ${sub.imsi}?`)) return
    setDeleting(sub.subscriber_id)
    try {
      await deleteSubscriber(sub.subscriber_id)
      toast.success('Subscriber deleted', sub.imsi)
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
        <span>Loading subscribers...</span>
      </div>
    )
  }

  if (error && items.length === 0) {
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
          <div className="page-title">Subscribers</div>
          <div className="page-subtitle">{total != null ? total : items.length} subscriber{(total ?? items.length) !== 1 ? 's' : ''} configured</div>
        </div>
        {activeTab === 'subscribers' && (
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn btn-ghost" onClick={refresh}><RefreshCw size={14} /> Refresh</button>
            <button className="btn btn-primary" onClick={() => setModal('add')}><Plus size={14} /> Add Subscriber</button>
          </div>
        )}
      </div>

      {/* Tab bar */}
      <div style={{ display: 'flex', borderBottom: '1px solid var(--border)', marginBottom: 16, gap: 0 }}>
        <button style={TAB_STYLE(activeTab === 'subscribers')} onClick={() => setActiveTab('subscribers')}>Subscribers</button>
        <button style={TAB_STYLE(activeTab === 'attributes')} onClick={() => setActiveTab('attributes')}>Attributes</button>
        <button style={TAB_STYLE(activeTab === 'routings')} onClick={() => setActiveTab('routings')}>Routings</button>
      </div>

      {activeTab === 'subscribers' && (<>
        <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
          <div style={{ position: 'relative', flex: 1, maxWidth: 340 }}>
            <Search size={14} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
            <input
              className="input"
              style={{ paddingLeft: 32 }}
              placeholder="Search by IMSI or MSISDN..."
              value={search}
              onChange={e => setSearch(e.target.value)}
            />
          </div>
        </div>

        {!loading && items.length === 0 ? (
          <div className="empty-state">
            {search ? 'No subscribers match your search.' : 'No subscribers configured yet.'}
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th className={`sortable${sortKey === 'imsi' ? ' sort-active' : ''}`} onClick={() => handleSort('imsi')}>IMSI<SortIcon col="imsi" /></th>
                  <th className={`sortable${sortKey === 'msisdn' ? ' sort-active' : ''}`} onClick={() => handleSort('msisdn')}>MSISDN<SortIcon col="msisdn" /></th>
                  <th className={`sortable${sortKey === 'enabled' ? ' sort-active' : ''}`} onClick={() => handleSort('enabled')}>Status<SortIcon col="enabled" /></th>
                  <th className={`sortable${sortKey === 'nam' ? ' sort-active' : ''}`} onClick={() => handleSort('nam')}>NAM<SortIcon col="nam" /></th>
                  <th>UE AMBR DL</th>
                  <th>IMEI / Device</th>
                  <th className={`sortable${sortKey === 'last_modified' ? ' sort-active' : ''}`} onClick={() => handleSort('last_modified')}>Last Modified<SortIcon col="last_modified" /></th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {sortedSubs.map(sub => (
                  <tr key={sub.subscriber_id}>
                    <td className="mono" style={{ fontSize: '0.82rem', fontWeight: 600 }}>{sub.imsi}</td>
                    <td className="mono" style={{ fontSize: '0.82rem' }}>{sub.msisdn || '—'}</td>
                    <td>
                      <Badge state={sub.enabled !== false ? 'enabled' : 'disabled'} />
                    </td>
                    <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                      {NAM_LABELS[sub.nam ?? 0] ?? '—'}
                    </td>
                    <td className="mono" style={{ fontSize: '0.78rem' }}>
                      {fmtBps(sub.ue_ambr_dl)}
                    </td>
                    <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)', maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {getDeviceInfo(sub.imsi)}
                    </td>
                    <td style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                      {sub.last_modified ? new Date(sub.last_modified).toLocaleString() : '—'}
                    </td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn-icon" title="Edit" onClick={() => setModal({ sub })}><Pencil size={13} /></button>
                        <button
                          className="btn-icon danger"
                          title={subscriberUsageReason(sub) || 'Delete'}
                          onClick={() => handleDelete(sub)}
                          disabled={deleting === sub.subscriber_id || !!subscriberUsageReason(sub)}
                        >
                          {deleting === sub.subscriber_id ? <Spinner size="sm" /> : <Trash2 size={13} />}
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

        {modal === 'add' && (
          <SubscriberModal onClose={() => setModal(null)} onSaved={refresh} aucList={aucList} apnList={apnList} />
        )}
        {modal && typeof modal === 'object' && modal.sub && (
          <SubscriberModal sub={modal.sub} onClose={() => setModal(null)} onSaved={refresh} aucList={aucList} apnList={apnList} />
        )}
      </>)}

      {activeTab === 'attributes' && <SubscriberAttributes compact />}
      {activeTab === 'routings' && <SubscriberRoutings compact />}
    </div>
  )
}
