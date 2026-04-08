import React, { useState, useEffect, useRef } from 'react'
import { X, Check, ChevronRight, Eye, EyeOff, Plus } from 'lucide-react'
import Spinner from '../components/Spinner.jsx'
import { useToast } from '../components/Toast.jsx'
import { createAUC, createSubscriber, createIMSSubscriber, getAPNs, getAlgorithmProfiles, getIFCProfiles } from '../api/client.js'

const STEPS = ['SIM / AUC', 'Subscriber', 'IMS Subscriber', 'Review']

const CHIP_STYLE = {
  display: 'inline-flex', alignItems: 'center', gap: 4,
  padding: '2px 8px 2px 10px', background: 'var(--accent)',
  color: '#fff', borderRadius: 12, fontSize: '0.78rem', fontWeight: 500,
}
const CHIP_BTN = {
  background: 'none', border: 'none', cursor: 'pointer', color: '#fff',
  padding: 0, display: 'flex', alignItems: 'center', opacity: 0.8,
}
const SECTION_STYLE = {
  fontSize: '0.72rem', fontWeight: 600, textTransform: 'uppercase',
  letterSpacing: '0.08em', color: 'var(--text-muted)',
  borderBottom: '1px solid var(--border-subtle)', paddingBottom: 4,
  marginBottom: 10, marginTop: 16,
}

export default function SubscriberWizard({ onClose }) {
  const toast = useToast()
  const overlayRef = useRef(null)
  const [step, setStep] = useState(0)
  const [skipIMS, setSkipIMS] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [progress, setProgress] = useState(null) // 'auc' | 'subscriber' | 'ims' | 'done'

  // Preloaded lists
  const [apnList, setApnList] = useState([])
  const [algorithmProfiles, setAlgorithmProfiles] = useState([])
  const [ifcProfiles, setIfcProfiles] = useState([])

  useEffect(() => {
    getAPNs().then(d => setApnList(Array.isArray(d) ? d : [])).catch(() => {})
    getAlgorithmProfiles().then(d => setAlgorithmProfiles(Array.isArray(d) ? d : [])).catch(() => {})
    getIFCProfiles().then(d => setIfcProfiles(Array.isArray(d) ? d : [])).catch(() => {})
  }, [])

  // ── Step 1: AUC ──────────────────────────────────────────────────────────────
  const [showKeys, setShowKeys] = useState(true)
  const [auc, setAUC] = useState({
    imsi: '', iccid: '', ki: '', opc: '', amf: '8000',
    esim: false, sim_vendor: '', batch_name: '', lpa: '',
    pin1: '', pin2: '', puk1: '', puk2: '',
    algorithm_profile_id: '',
  })

  // ── Step 2: Subscriber ───────────────────────────────────────────────────────
  const [sub, setSub] = useState({
    msisdn: '', enabled: true, roaming_enabled: true,
    ue_ambr_dl: 0, ue_ambr_ul: 0, nam: 0,
    subscribed_rau_tau_timer: 300,
  })
  const [selectedApnIds, setSelectedApnIds] = useState([])
  const [apnPickerValue, setApnPickerValue] = useState('')

  // ── Step 3: IMS Subscriber ───────────────────────────────────────────────────
  const [ims, setIMS] = useState({ msisdn_list: '', ifc_profile_id: '' })

  function setA(k, v) { setAUC(p => ({ ...p, [k]: v })) }
  function setS(k, v) { setSub(p => ({ ...p, [k]: v })) }
  function setI(k, v) { setIMS(p => ({ ...p, [k]: v })) }

  function addApn() {
    if (!apnPickerValue || selectedApnIds.includes(apnPickerValue)) return
    setSelectedApnIds(p => [...p, apnPickerValue])
    setApnPickerValue('')
  }
  function removeApn(id) { setSelectedApnIds(p => p.filter(x => x !== id)) }

  // Propagate subscriber MSISDN → IMS step: mirror into msisdn_list if not already set
  useEffect(() => {
    if (sub.msisdn) setIMS(p => ({
      ...p,
      msisdn: sub.msisdn,
      msisdn_list: p.msisdn_list || sub.msisdn,
    }))
  }, [sub.msisdn])

  function handleOverlayClick(e) {
    if (e.target === overlayRef.current) onClose()
  }

  function canAdvance() {
    if (step === 0) return auc.imsi.length >= 10 && auc.ki.length === 32 && auc.opc.length === 32
    if (step === 1) return !!sub.msisdn.trim()
    return true
  }

  function advance() {
    if (step === 2 && skipIMS) { setStep(3); return }
    setStep(s => Math.min(s + 1, 3))
  }

  function back() { setStep(s => Math.max(s - 1, 0)) }

  async function handleFinish() {
    setSubmitting(true)
    let createdAUCId = null
    try {
      setProgress('auc')
      const aucPayload = {
        imsi: auc.imsi, ki: auc.ki, opc: auc.opc, amf: auc.amf || '8000',
        ...(auc.iccid && { iccid: auc.iccid }),
        ...(auc.sim_vendor && { sim_vendor: auc.sim_vendor }),
        ...(auc.batch_name && { batch_name: auc.batch_name }),
        esim: auc.esim,
        ...(auc.esim && auc.lpa && { lpa: auc.lpa }),
        ...(auc.pin1 && { pin1: auc.pin1 }), ...(auc.pin2 && { pin2: auc.pin2 }),
        ...(auc.puk1 && { puk1: auc.puk1 }), ...(auc.puk2 && { puk2: auc.puk2 }),
        algorithm_profile_id: auc.algorithm_profile_id !== '' ? parseInt(auc.algorithm_profile_id, 10) : null,
      }
      const aucResult = await createAUC(aucPayload)
      createdAUCId = aucResult?.auc_id

      setProgress('subscriber')
      const apn_list = selectedApnIds.join(',')
      const default_apn = selectedApnIds.length > 0 ? Number(selectedApnIds[0]) : 0
      const subPayload = {
        imsi: auc.imsi,
        auc_id: createdAUCId,
        msisdn: sub.msisdn || undefined,
        enabled: sub.enabled,
        roaming_enabled: sub.roaming_enabled,
        ue_ambr_dl: Number(sub.ue_ambr_dl),
        ue_ambr_ul: Number(sub.ue_ambr_ul),
        nam: Number(sub.nam),
        subscribed_rau_tau_timer: Number(sub.subscribed_rau_tau_timer),
        apn_list,
        default_apn,
      }
      await createSubscriber(subPayload)

      if (!skipIMS) {
        setProgress('ims')
        const imsPayload = {
          msisdn: sub.msisdn,
          imsi: auc.imsi,
          ...(ims.msisdn_list && { msisdn_list: ims.msisdn_list }),
          ...(ims.ifc_profile_id !== '' && { ifc_profile_id: parseInt(ims.ifc_profile_id, 10) }),
        }
        await createIMSSubscriber(imsPayload)
      }

      setProgress('done')
      toast.success('Subscriber created', `${auc.imsi} provisioned successfully`)
      setTimeout(onClose, 1200)
    } catch (err) {
      toast.error(`Failed at ${progress || 'setup'}`, err.message)
      setProgress(null)
      setSubmitting(false)
    }
  }

  const availableApns = apnList.filter(a => !selectedApnIds.includes(String(a.apn_id)))

  return (
    <div className="modal-overlay" ref={overlayRef} onClick={handleOverlayClick} role="dialog" aria-modal="true">
      <div className="modal modal-xl" style={{ maxHeight: '94vh' }} role="document">

        {/* Header */}
        <div className="modal-header">
          <div>
            <h3 className="modal-title">New Subscriber Wizard</h3>
            <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginTop: 2 }}>
              Step {step + 1} of {STEPS.length} — {STEPS[step]}
            </div>
          </div>
          <button className="btn-icon" onClick={onClose} aria-label="Close"><X size={16} /></button>
        </div>

        {/* Step indicator */}
        <div style={{ display: 'flex', padding: '12px 20px', gap: 0, borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)', flexShrink: 0 }}>
          {STEPS.map((label, i) => {
            const done = i < step
            const active = i === step
            const skipped = i === 2 && skipIMS
            return (
              <React.Fragment key={i}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, flex: i < STEPS.length - 1 ? 1 : 'none' }}>
                  <div style={{
                    width: 22, height: 22, borderRadius: '50%', flexShrink: 0,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: '0.7rem', fontWeight: 700,
                    background: done ? 'var(--success)' : active ? 'var(--accent)' : 'var(--bg-input)',
                    color: (done || active) ? '#fff' : 'var(--text-muted)',
                    border: active ? '2px solid var(--accent)' : 'none',
                  }}>
                    {done ? <Check size={11} /> : i + 1}
                  </div>
                  <span style={{ fontSize: '0.75rem', fontWeight: active ? 600 : 400, color: active ? 'var(--text)' : 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                    {label}{skipped ? ' (skip)' : ''}
                  </span>
                  {i < STEPS.length - 1 && <div style={{ flex: 1, height: 1, background: 'var(--border-subtle)', margin: '0 8px' }} />}
                </div>
              </React.Fragment>
            )
          })}
        </div>

        {/* Body */}
        <div className="modal-body" style={{ overflowY: 'auto' }}>

          {/* ── STEP 1: AUC ── */}
          {step === 0 && (
            <>
              <div style={SECTION_STYLE}>Identity</div>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">IMSI <span style={{ color: 'var(--danger)' }}>*</span></label>
                  <input className="input mono" value={auc.imsi} onChange={e => setA('imsi', e.target.value)} placeholder="001010000000001" maxLength={15} required />
                </div>
                <div className="form-group">
                  <label className="form-label">ICCID</label>
                  <input className="input mono" value={auc.iccid} onChange={e => setA('iccid', e.target.value)} placeholder="89xxxxxxxxxxxxxxxxxxxx" maxLength={22} />
                </div>
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">SIM Vendor</label>
                  <input className="input" value={auc.sim_vendor} onChange={e => setA('sim_vendor', e.target.value)} placeholder="(optional)" />
                </div>
                <div className="form-group">
                  <label className="form-label">Batch Name</label>
                  <input className="input" value={auc.batch_name} onChange={e => setA('batch_name', e.target.value)} placeholder="(optional)" />
                </div>
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">Algorithm Profile</label>
                  <select className="select" value={auc.algorithm_profile_id} onChange={e => setA('algorithm_profile_id', e.target.value)}>
                    <option value="">Default (Standard Milenage)</option>
                    {algorithmProfiles.map(p => (
                      <option key={p.algorithm_profile_id} value={String(p.algorithm_profile_id)}>{p.profile_name}</option>
                    ))}
                  </select>
                </div>
                <div className="form-group" style={{ alignSelf: 'flex-end' }}>
                  <label className="checkbox-wrap">
                    <input type="checkbox" checked={auc.esim} onChange={e => setA('esim', e.target.checked)} />
                    <span className="form-label" style={{ margin: 0 }}>eSIM</span>
                  </label>
                </div>
              </div>

              <div style={{ ...SECTION_STYLE, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span>Authentication Keys</span>
                <button type="button" className="btn btn-ghost" style={{ fontSize: '0.72rem', padding: '2px 8px', height: 'auto' }} onClick={() => setShowKeys(v => !v)}>
                  {showKeys ? <EyeOff size={12} /> : <Eye size={12} />}{showKeys ? ' Hide' : ' Show'}
                </button>
              </div>
              {showKeys && (
                <>
                  <div className="form-row">
                    <div className="form-group">
                      <label className="form-label">Ki (hex 32) <span style={{ color: 'var(--danger)' }}>*</span></label>
                      <input className="input mono" value={auc.ki} onChange={e => setA('ki', e.target.value)} placeholder="0000000000000000000000000000000" maxLength={32} />
                      {auc.ki && auc.ki.length !== 32 && <div style={{ fontSize: '0.7rem', color: 'var(--danger)', marginTop: 2 }}>Must be exactly 32 hex characters</div>}
                    </div>
                    <div className="form-group">
                      <label className="form-label">OPc (hex 32) <span style={{ color: 'var(--danger)' }}>*</span></label>
                      <input className="input mono" value={auc.opc} onChange={e => setA('opc', e.target.value)} placeholder="0000000000000000000000000000000" maxLength={32} />
                      {auc.opc && auc.opc.length !== 32 && <div style={{ fontSize: '0.7rem', color: 'var(--danger)', marginTop: 2 }}>Must be exactly 32 hex characters</div>}
                    </div>
                  </div>
                  <div className="form-row">
                    <div className="form-group">
                      <label className="form-label">AMF (hex 4)</label>
                      <input className="input mono" value={auc.amf} onChange={e => setA('amf', e.target.value)} placeholder="8000" maxLength={4} />
                    </div>
                  </div>
                </>
              )}

              <div style={SECTION_STYLE}>SIM Codes (optional)</div>
              <div className="form-row">
                <div className="form-group"><label className="form-label">PIN1</label><input className="input mono" value={auc.pin1} onChange={e => setA('pin1', e.target.value)} placeholder="(optional)" /></div>
                <div className="form-group"><label className="form-label">PIN2</label><input className="input mono" value={auc.pin2} onChange={e => setA('pin2', e.target.value)} placeholder="(optional)" /></div>
              </div>
              <div className="form-row">
                <div className="form-group"><label className="form-label">PUK1</label><input className="input mono" value={auc.puk1} onChange={e => setA('puk1', e.target.value)} placeholder="(optional)" /></div>
                <div className="form-group"><label className="form-label">PUK2</label><input className="input mono" value={auc.puk2} onChange={e => setA('puk2', e.target.value)} placeholder="(optional)" /></div>
              </div>
            </>
          )}

          {/* ── STEP 2: Subscriber ── */}
          {step === 1 && (
            <>
              <div style={SECTION_STYLE}>Identity</div>
              <div style={{ padding: '6px 10px', background: 'var(--bg-elevated)', borderRadius: 'var(--radius-sm)', marginBottom: 12, fontSize: '0.82rem' }}>
                IMSI: <span className="mono" style={{ fontWeight: 600 }}>{auc.imsi}</span>
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">MSISDN <span style={{ color: 'var(--danger)' }}>*</span></label>
                  <input className="input mono" value={sub.msisdn} onChange={e => setS('msisdn', e.target.value)} placeholder="441234567890" required />
                </div>
                <div className="form-group" style={{ alignSelf: 'flex-end', display: 'flex', gap: 16, paddingBottom: 4 }}>
                  <label className="checkbox-wrap"><input type="checkbox" checked={sub.enabled} onChange={e => setS('enabled', e.target.checked)} /><span className="form-label" style={{ margin: 0 }}>Enabled</span></label>
                  <label className="checkbox-wrap"><input type="checkbox" checked={sub.roaming_enabled} onChange={e => setS('roaming_enabled', e.target.checked)} /><span className="form-label" style={{ margin: 0 }}>Roaming</span></label>
                </div>
              </div>

              <div style={SECTION_STYLE}>APN Configuration</div>
              <div className="form-group">
                <label className="form-label">APNs</label>
                <div style={{ display: 'flex', gap: 8 }}>
                  <select className="select" value={apnPickerValue} onChange={e => setApnPickerValue(e.target.value)} style={{ flex: 1 }}>
                    <option value="">— Add an APN —</option>
                    {availableApns.map(a => <option key={a.apn_id} value={String(a.apn_id)}>{a.apn}</option>)}
                  </select>
                  <button type="button" className="btn btn-ghost" onClick={addApn} disabled={!apnPickerValue}>Add</button>
                </div>
                {selectedApnIds.length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 6 }}>
                    {selectedApnIds.map((id, idx) => {
                      const apn = apnList.find(a => String(a.apn_id) === id)
                      return (
                        <span key={id} style={CHIP_STYLE}>
                          {idx === 0 && <span style={{ fontSize: '0.65rem', opacity: 0.85, marginRight: 2 }}>[default]</span>}
                          {apn ? apn.apn : `ID:${id}`}
                          <button type="button" style={CHIP_BTN} onClick={() => removeApn(id)}><X size={11} /></button>
                        </span>
                      )
                    })}
                  </div>
                )}
              </div>

              <div style={SECTION_STYLE}>QoS & Network</div>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">UE AMBR DL (bps)</label>
                  <input className="input" type="number" min="0" value={sub.ue_ambr_dl} onChange={e => setS('ue_ambr_dl', e.target.value)} />
                </div>
                <div className="form-group">
                  <label className="form-label">UE AMBR UL (bps)</label>
                  <input className="input" type="number" min="0" value={sub.ue_ambr_ul} onChange={e => setS('ue_ambr_ul', e.target.value)} />
                </div>
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label className="form-label">NAM</label>
                  <select className="select" value={sub.nam} onChange={e => setS('nam', Number(e.target.value))}>
                    <option value={0}>0 — PS+CS</option>
                    <option value={2}>2 — PS Only</option>
                  </select>
                </div>
                <div className="form-group">
                  <label className="form-label">RAU/TAU Timer (s)</label>
                  <input className="input" type="number" min="0" value={sub.subscribed_rau_tau_timer} onChange={e => setS('subscribed_rau_tau_timer', e.target.value)} />
                </div>
              </div>
            </>
          )}

          {/* ── STEP 3: IMS Subscriber ── */}
          {step === 2 && (
            <>
              <div style={{ padding: '8px 12px', background: 'rgba(99,102,241,0.08)', border: '1px solid rgba(99,102,241,0.2)', borderRadius: 'var(--radius-sm)', marginBottom: 12, fontSize: '0.82rem' }}>
                This step is <strong>optional</strong>. Click "Skip" to create only AUC + Subscriber.
              </div>
              <div style={SECTION_STYLE}>IMS Identity</div>
              <div style={{ padding: '6px 10px', background: 'var(--bg-elevated)', borderRadius: 'var(--radius-sm)', marginBottom: 12, fontSize: '0.82rem', display: 'flex', gap: 24 }}>
                <span>IMSI: <span className="mono" style={{ fontWeight: 600 }}>{auc.imsi}</span></span>
                <span>MSISDN: <span className="mono" style={{ fontWeight: 600 }}>{sub.msisdn || '—'}</span></span>
              </div>
              <div className="form-group">
                <label className="form-label">Additional MSISDNs (MSISDN List, comma-separated)</label>
                <input className="input mono" value={ims.msisdn_list} onChange={e => setI('msisdn_list', e.target.value)} placeholder="441234567890,441234567891" />
              </div>
              <div className="form-group">
                <label className="form-label">IFC Profile</label>
                <select className="select" value={ims.ifc_profile_id} onChange={e => setI('ifc_profile_id', e.target.value)}>
                  <option value="">— None —</option>
                  {ifcProfiles.map(p => <option key={p.ifc_profile_id} value={String(p.ifc_profile_id)}>{p.name}</option>)}
                </select>
              </div>
            </>
          )}

          {/* ── STEP 4: Review ── */}
          {step === 3 && (
            <>
              <div style={{ fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: 16 }}>
                Review the details below then click <strong>Finish</strong> to provision the subscriber.
              </div>
              <div className="table-container">
                <table>
                  <tbody>
                    <tr><td style={{ fontWeight: 600, width: 160 }}>IMSI</td><td className="mono">{auc.imsi}</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>ICCID</td><td className="mono">{auc.iccid || '—'}</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>Ki</td><td className="mono" style={{ color: 'var(--text-muted)' }}>{'•'.repeat(8)}…{'•'.repeat(8)}</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>OPc</td><td className="mono" style={{ color: 'var(--text-muted)' }}>{'•'.repeat(8)}…{'•'.repeat(8)}</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>AMF</td><td className="mono">{auc.amf}</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>Algorithm</td><td>{auc.algorithm_profile_id ? `Profile #${auc.algorithm_profile_id}` : 'Default Milenage'}</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>MSISDN</td><td className="mono">{sub.msisdn || '—'}</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>APNs</td><td>{selectedApnIds.length > 0 ? selectedApnIds.map(id => { const a = apnList.find(x => String(x.apn_id) === id); return a ? a.apn : id }).join(', ') : '—'}</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>UE AMBR DL</td><td className="mono">{Number(sub.ue_ambr_dl).toLocaleString()} bps</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>UE AMBR UL</td><td className="mono">{Number(sub.ue_ambr_ul).toLocaleString()} bps</td></tr>
                    <tr><td style={{ fontWeight: 600 }}>IMS Subscriber</td><td>{skipIMS ? <span style={{ color: 'var(--text-muted)' }}>Skipped</span> : <span style={{ color: 'var(--success)' }}>Yes — {sub.msisdn}</span>}</td></tr>
                  </tbody>
                </table>
              </div>

              {submitting && (
                <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {['auc', 'subscriber', 'ims', 'done'].map(stage => {
                    const labels = { auc: 'Creating AUC…', subscriber: 'Creating Subscriber…', ims: 'Creating IMS Subscriber…', done: 'Done!' }
                    const stages = ['auc', 'subscriber', 'ims', 'done']
                    const stageIdx = stages.indexOf(stage)
                    const progressIdx = stages.indexOf(progress)
                    const done = progressIdx > stageIdx
                    const active = progress === stage
                    const skip = stage === 'ims' && skipIMS
                    if (skip) return null
                    return (
                      <div key={stage} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: '0.82rem', color: done ? 'var(--success)' : active ? 'var(--text)' : 'var(--text-muted)' }}>
                        {done ? <Check size={14} style={{ color: 'var(--success)' }} /> : active ? <Spinner size="sm" /> : <div style={{ width: 14 }} />}
                        {labels[stage]}
                      </div>
                    )
                  })}
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="modal-footer">
          <button type="button" className="btn btn-ghost" onClick={step === 0 ? onClose : back} disabled={submitting}>
            {step === 0 ? 'Cancel' : 'Back'}
          </button>
          {step === 2 && !skipIMS && (
            <button type="button" className="btn btn-ghost" onClick={() => { setSkipIMS(true); setStep(3) }} disabled={submitting}>
              Skip IMS
            </button>
          )}
          {step < 3 ? (
            <button type="button" className="btn btn-primary" onClick={advance} disabled={!canAdvance()}>
              Next <ChevronRight size={14} />
            </button>
          ) : (
            <button type="button" className="btn btn-primary" onClick={handleFinish} disabled={submitting || progress === 'done'}>
              {submitting && progress !== 'done' ? <Spinner size="sm" /> : <Check size={14} />}
              {progress === 'done' ? 'Done!' : 'Finish'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
