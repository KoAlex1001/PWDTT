import { useState } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import Sidebar from './Sidebar';
import Settings from '../modals/Settings';

export default function Layout() {
  const [settingsOpen, setSettingsOpen] = useState(false);
  const { pathname } = useLocation();

  return (
    <div className="app-shell">
      <Sidebar
        onSettings={() => setSettingsOpen(true)}
        pathname={pathname}
      />
      <div className="app-content">
        <Outlet />
      </div>
      {settingsOpen && <Settings onClose={() => setSettingsOpen(false)} />}
    </div>
  );
}
