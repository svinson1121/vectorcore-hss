import React from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout.jsx'
import Dashboard from './pages/Dashboard.jsx'
import Subscribers from './pages/Subscribers.jsx'
import SubscriberAttributes from './pages/SubscriberAttributes.jsx'
import SubscriberRoutings from './pages/SubscriberRoutings.jsx'
import AUC from './pages/AUC.jsx'
import IMSSubscribers from './pages/IMSSubscribers.jsx'
import APNs from './pages/APNs.jsx'
import ChargingRules from './pages/ChargingRules.jsx'
import EIR from './pages/EIR.jsx'
import Roaming from './pages/Roaming.jsx'
import Sessions from './pages/Sessions.jsx'
import Metrics from './pages/Metrics.jsx'
import OAM from './pages/OAM.jsx'
import IFCProfiles from './pages/IFCProfiles.jsx'
import AlgorithmProfiles from './pages/AlgorithmProfiles.jsx'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<Dashboard />} />
        <Route path="subscribers" element={<Subscribers />} />
        <Route path="subscriber-attributes" element={<SubscriberAttributes />} />
        <Route path="subscriber-routings" element={<SubscriberRoutings />} />
        <Route path="auc" element={<AUC />} />
        <Route path="ims-subscribers" element={<IMSSubscribers />} />
        <Route path="ifc-profiles" element={<IFCProfiles />} />
        <Route path="algorithm-profiles" element={<AlgorithmProfiles />} />
        <Route path="apns" element={<APNs />} />
        <Route path="charging" element={<ChargingRules />} />
        <Route path="eir" element={<EIR />} />
        <Route path="roaming" element={<Roaming />} />
        <Route path="sessions" element={<Sessions />} />
        <Route path="metrics" element={<Metrics />} />
        <Route path="oam" element={<OAM />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Route>
    </Routes>
  )
}
