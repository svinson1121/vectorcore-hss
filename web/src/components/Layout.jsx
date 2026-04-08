import React, { useState } from 'react'
import { Outlet } from 'react-router-dom'
import Sidebar from './Sidebar.jsx'
import TopBar from './TopBar.jsx'
import SubscriberWizard from '../pages/SubscriberWizard.jsx'

export default function Layout() {
  const [wizardOpen, setWizardOpen] = useState(false)

  return (
    <div className="layout">
      <Sidebar onOpenWizard={() => setWizardOpen(true)} />
      <div className="main-content">
        <TopBar />
        <main className="page">
          <Outlet />
        </main>
      </div>
      {wizardOpen && <SubscriberWizard onClose={() => setWizardOpen(false)} />}
    </div>
  )
}
