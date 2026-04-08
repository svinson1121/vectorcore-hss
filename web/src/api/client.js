const BASE = '/api/v1'

async function request(method, path, body) {
  const opts = { method, headers: {} }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(`${BASE}${path}`, opts)
  if (res.status === 204) return null
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const data = await res.json()
      msg = data.detail || data.message || data.error || msg
      if (data.errors && data.errors.length > 0) {
        msg += ': ' + data.errors.map(e => `${e.path || e.location || '?'} — ${e.message}${e.value !== undefined ? ` (got ${JSON.stringify(e.value)})` : ''}`).join('; ')
      }
    } catch {}
    throw new Error(msg)
  }
  return res.json()
}

// AUC (SIM cards)
export const getAUCs = ({ search = '', limit = 0, offset = 0 } = {}) => {
  const p = new URLSearchParams()
  if (search) p.set('search', search)
  if (limit > 0) { p.set('limit', String(limit)); p.set('offset', String(offset)) }
  const qs = p.toString()
  return request('GET', `/subscriber/auc${qs ? `?${qs}` : ''}`)
}
export const createAUC = (data) => request('POST', '/subscriber/auc', data)
export const updateAUC = (id, data) => request('PUT', `/subscriber/auc/${id}`, data)
export const deleteAUC = (id) => request('DELETE', `/subscriber/auc/${id}`)

// Subscribers
export const getSubscribers = ({ search = '', limit = 0, offset = 0 } = {}) => {
  const p = new URLSearchParams()
  if (search) p.set('search', search)
  if (limit > 0) { p.set('limit', String(limit)); p.set('offset', String(offset)) }
  const qs = p.toString()
  return request('GET', `/subscriber${qs ? `?${qs}` : ''}`)
}
export const getSubscriber = (id) => request('GET', `/subscriber/${id}`)
export const getSubscriberByIMSI = (imsi) => request('GET', `/subscriber/imsi/${imsi}`)
export const createSubscriber = (data) => request('POST', '/subscriber', data)
export const updateSubscriber = (id, data) => request('PUT', `/subscriber/${id}`, data)
export const deleteSubscriber = (id) => request('DELETE', `/subscriber/${id}`)

// APNs
export const getAPNs = () => request('GET', '/apn')
export const getAPN = (id) => request('GET', `/apn/${id}`)
export const createAPN = (data) => request('POST', '/apn', data)
export const updateAPN = (id, data) => request('PUT', `/apn/${id}`, data)
export const deleteAPN = (id) => request('DELETE', `/apn/${id}`)

// Charging Rules
export const getChargingRules = () => request('GET', '/apn/charging_rule')
export const createChargingRule = (data) => request('POST', '/apn/charging_rule', data)
export const updateChargingRule = (id, data) => request('PUT', `/apn/charging_rule/${id}`, data)
export const deleteChargingRule = (id) => request('DELETE', `/apn/charging_rule/${id}`)

// TFT
export const getTFTs = () => request('GET', '/apn/charging_rule/tft')
export const createTFT = (data) => request('POST', '/apn/charging_rule/tft', data)
export const updateTFT = (id, data) => request('PUT', `/apn/charging_rule/tft/${id}`, data)
export const deleteTFT = (id) => request('DELETE', `/apn/charging_rule/tft/${id}`)

// IMS Subscribers
export const getIMSSubscribers = ({ search = '', limit = 0, offset = 0 } = {}) => {
  const p = new URLSearchParams()
  if (search) p.set('search', search)
  if (limit > 0) { p.set('limit', String(limit)); p.set('offset', String(offset)) }
  const qs = p.toString()
  return request('GET', `/ims_subscriber${qs ? `?${qs}` : ''}`)
}
export const createIMSSubscriber = (data) => request('POST', '/ims_subscriber', data)
export const updateIMSSubscriber = (id, data) => request('PUT', `/ims_subscriber/${id}`, data)
export const deleteIMSSubscriber = (id) => request('DELETE', `/ims_subscriber/${id}`)

// IFC Profiles
export const getIFCProfiles = () => request('GET', '/ims_subscriber/ifc_profile')
export const createIFCProfile = (data) => request('POST', '/ims_subscriber/ifc_profile', data)
export const updateIFCProfile = (id, data) => request('PUT', `/ims_subscriber/ifc_profile/${id}`, data)
export const deleteIFCProfile = (id) => request('DELETE', `/ims_subscriber/ifc_profile/${id}`)

// Algorithm Profiles
export const getAlgorithmProfiles = () => request('GET', '/subscriber/auc/profile')
export const createAlgorithmProfile = (data) => request('POST', '/subscriber/auc/profile', data)
export const updateAlgorithmProfile = (id, data) => request('PUT', `/subscriber/auc/profile/${id}`, data)
export const deleteAlgorithmProfile = (id) => request('DELETE', `/subscriber/auc/profile/${id}`)

// EIR History
export const getEIRHistory = () => request('GET', '/eir/history')

// TAC
export const getTACs = () => request('GET', '/eir/tac')
export const lookupIMEI = (imei) => request('GET', `/eir/tac/imei/${encodeURIComponent(imei)}`)

// Backup
export const triggerBackup = () => request('POST', '/oam/backup')

// EIR
export const getEIRs = () => request('GET', '/eir')
export const createEIR = (data) => request('POST', '/eir', data)
export const updateEIR = (id, data) => request('PUT', `/eir/${id}`, data)
export const deleteEIR = (id) => request('DELETE', `/eir/${id}`)

// Roaming Rules
export const getRoamingRules = () => request('GET', '/roaming_rules')
export const createRoamingRule = (data) => request('POST', '/roaming_rules', data)
export const updateRoamingRule = (id, data) => request('PUT', `/roaming_rules/${id}`, data)
export const deleteRoamingRule = (id) => request('DELETE', `/roaming_rules/${id}`)

// Sessions
export const getServingAPNs = () => request('GET', '/oam/serving_apn')
export const getPDUSessions = () => request('GET', '/oam/pdu_session')

// Diameter Peers
export const getDiameterPeers = () => request('GET', '/oam/diameter/peers').then(r => r?.peers ?? r)

// CLR
export const sendCLR = (imsi) => request('POST', `/subscriber/clr/${encodeURIComponent(imsi)}`)

// Operation Log
export const getOperationLogs = () => request('GET', '/oam/operation_log')
export const rollbackOperation = (id) => request('POST', `/oam/operation_log/${id}/rollback`)

// Emergency Sessions
export const getEmergencySessions = () => request('GET', '/oam/emergency_subscriber')

// Subscriber Attributes
export const getSubscriberAttributes = () => request('GET', '/subscriber/attributes')
export const createSubscriberAttribute = (data) => request('POST', '/subscriber/attributes', data)
export const updateSubscriberAttribute = (id, data) => request('PUT', `/subscriber/attributes/${id}`, data)
export const deleteSubscriberAttribute = (id) => request('DELETE', `/subscriber/attributes/${id}`)

// Subscriber Routings
export const getSubscriberRoutings = () => request('GET', '/subscriber/routing')
export const createSubscriberRouting = (data) => request('POST', '/subscriber/routing', data)
export const updateSubscriberRouting = (id, data) => request('PUT', `/subscriber/routing/${id}`, data)
export const deleteSubscriberRouting = (id) => request('DELETE', `/subscriber/routing/${id}`)

// TAC CRUD
export const getTACCount = () => request('GET', '/eir/tac?limit=10000').then(r => Array.isArray(r) ? r.length : 0)
export const createTAC = (data) => request('POST', '/eir/tac', data)
export const updateTAC = (tac, data) => request('PUT', `/eir/tac/${encodeURIComponent(tac)}`, data)
export const deleteTAC = (tac) => request('DELETE', `/eir/tac/${encodeURIComponent(tac)}`)

// Metrics / OAM
export const getMetricsSummary = () => request('GET', '/oam/metrics')
export const getVersion = () => request('GET', '/oam/version')

// Health
export async function getHealth() {
  const res = await fetch('/api/v1/oam/health')
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

// Raw Prometheus
export async function getPrometheusText() {
  const res = await fetch('/metrics')
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.text()
}

// Prometheus text parser
export function parsePrometheusText(text) {
  const metrics = {}
  if (!text) return metrics
  const lines = text.split('\n')
  let currentHelp = {}
  let currentType = {}
  for (const raw of lines) {
    const line = raw.trim()
    if (!line || line.startsWith('#')) {
      if (line.startsWith('# HELP ')) {
        const rest = line.slice(7)
        const sp = rest.indexOf(' ')
        currentHelp[rest.slice(0, sp)] = rest.slice(sp + 1)
      } else if (line.startsWith('# TYPE ')) {
        const parts = line.slice(7).split(' ')
        currentType[parts[0]] = parts[1]
      }
      continue
    }
    const braceOpen = line.indexOf('{')
    const spaceIdx = line.lastIndexOf(' ')
    let name, labelsStr, value
    if (braceOpen !== -1) {
      const braceClose = line.indexOf('}')
      name = line.slice(0, braceOpen)
      labelsStr = line.slice(braceOpen + 1, braceClose)
      const rest = line.slice(braceClose + 1).trim()
      value = parseFloat(rest.split(' ')[0])
    } else {
      name = line.slice(0, spaceIdx)
      labelsStr = ''
      value = parseFloat(line.slice(spaceIdx + 1).split(' ')[0])
    }
    const labels = {}
    if (labelsStr) {
      const re = /(\w+)="([^"]*)"/g
      let m
      while ((m = re.exec(labelsStr)) !== null) labels[m[1]] = m[2]
    }
    if (!metrics[name]) {
      metrics[name] = { name, help: currentHelp[name] || '', type: currentType[name] || 'untyped', samples: [] }
    }
    metrics[name].samples.push({ labels, value })
  }
  return metrics
}

export function sumMetric(metrics, name) {
  const m = metrics[name]
  if (!m) return 0
  return m.samples.reduce((acc, s) => acc + (isNaN(s.value) ? 0 : s.value), 0)
}

export function getMetricByLabel(metrics, name, labelKey, labelValue) {
  const m = metrics[name]
  if (!m) return null
  return m.samples.find(s => s.labels[labelKey] === labelValue) || null
}
