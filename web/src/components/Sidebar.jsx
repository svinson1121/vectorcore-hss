import React from 'react'
import { NavLink } from 'react-router-dom'
import { LayoutDashboard, Users, CreditCard, Phone, Globe, ShieldCheck, Globe2, Activity, BarChart2, Settings, UserPlus } from 'lucide-react'

const NAV_ITEMS = [
  { to: '/dashboard', label: 'Dashboard', icon: <LayoutDashboard size={16} /> },
  { to: '/auc', label: 'SIM Cards / AUC', icon: <CreditCard size={16} /> },
  { to: '/subscribers', label: 'Subscribers', icon: <Users size={16} /> },
  { to: '/ims-subscribers', label: 'IMS Subscribers', icon: <Phone size={16} /> },
  { to: '/apns', label: 'APNs', icon: <Globe size={16} /> },
  { to: '/eir', label: 'EIR', icon: <ShieldCheck size={16} /> },
  { to: '/roaming', label: 'Roaming', icon: <Globe2 size={16} /> },
  { to: '/sessions', label: 'Sessions', icon: <Activity size={16} /> },
  { to: '/metrics', label: 'Metrics', icon: <BarChart2 size={16} /> },
  { to: '/oam', label: 'OAM', icon: <Settings size={16} /> },
]

export default function Sidebar({ onOpenWizard }) {
  return (
    <aside className="sidebar">
      <div className="sidebar-header" style={{ textAlign: 'center' }}>
        <div className="sidebar-logo">VectorCore</div>
        <div className="sidebar-logo-sub">Home Subscriber Server</div>
        <div style={{ fontSize: '0.65rem', color: 'var(--text-muted)', marginTop: 2, letterSpacing: '0.04em' }}>
          HLR / HSS / PCRF / UDM-UDR
        </div>
      </div>

      <nav className="sidebar-nav" aria-label="Primary navigation">
        {NAV_ITEMS.map(({ to, label, icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
          >
            {icon}
            {label}
          </NavLink>
        ))}
        <button className="nav-item" style={{ width: '100%', background: 'none', border: 'none', cursor: 'pointer', textAlign: 'left' }} onClick={onOpenWizard}>
          <UserPlus size={16} />
          Subscriber Wizard
        </button>
      </nav>
    </aside>
  )
}
