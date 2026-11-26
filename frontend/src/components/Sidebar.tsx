import { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import {
  IconPlugConnected,
  IconTerminal2,
  IconSettings2,
  IconSun,
  IconMoon,
} from '@tabler/icons-react';
import { themeStore } from '../lib/stores/themeStore';

const NAV = [
  { path: '/', label: 'Подключение', icon: <IconPlugConnected stroke={1.9} size={19} /> },
  { path: '/logs', label: 'Логи', icon: <IconTerminal2 stroke={1.9} size={19} /> },
];

interface Props {
  onSettings?: () => void;
  pathname?: string;
}

export default function Sidebar({ onSettings, pathname: pathnameProp }: Props) {
  const navigate = useNavigate();
  const location = useLocation();
  const pathname = pathnameProp ?? location.pathname;
  const [theme, setTheme] = useState(() => themeStore.get());

  const toggleTheme = () => {
    themeStore.toggle();
    setTheme(themeStore.get());
  };

  return (
    <>
      <style>{`
        .sidebar { width: 100%; height: 56px; flex-shrink: 0; display: flex; flex-direction: row; align-items: center; justify-content: space-between; padding: 0 10px; background: var(--sidebar-bg); border: 1px solid var(--sidebar-border); border-radius: 14px; box-shadow: var(--shadow-sm); overflow: hidden; }
        .sidebar-top, .sidebar-bottom { display: flex; flex-direction: row; align-items: center; gap: 6px; }
        .sidebar-brand { width: 30px; height: 30px; display: flex; align-items: center; justify-content: center; margin-right: 4px; }
        .sidebar-brand img { width: 22px; height: 22px; display: block; }
        .sidebar-sep { width: 1px; height: 22px; background: var(--sidebar-border); margin: 0 2px 0 4px; }
        .nav-btn { width: 38px; height: 38px; border: none; border-radius: 10px; background: transparent; color: var(--sidebar-text); cursor: pointer; display: flex; align-items: center; justify-content: center; transition: background 0.16s ease, color 0.16s ease, transform 0.16s ease; }
        .nav-btn:hover { background: var(--bg-3); color: var(--text); }
        .nav-btn:active { transform: scale(0.94); }
        .nav-btn--active, .nav-btn--active:hover { background: var(--sidebar-btn-active); color: var(--sidebar-icon-active); }
        .theme-toggle { width: 32px; height: 32px; border: none; border-radius: 9px; background: transparent; color: var(--sidebar-text); cursor: pointer; display: flex; align-items: center; justify-content: center; transition: background 0.16s ease, color 0.16s ease; }
        .theme-toggle:hover { background: var(--bg-3); color: var(--text); }
        .theme-toggle svg { transition: transform 0.35s ease, opacity 0.2s ease; }
        .theme-toggle:active svg { transform: rotate(140deg) scale(0.85); opacity: 0.5; }
      `}</style>
      <aside className="sidebar">
        <div className="sidebar-top">
          <div className="sidebar-brand" title="PWDTT">
            <img src="/favicon.svg" alt="" draggable={false} />
          </div>
          <div className="sidebar-sep" />
          {NAV.map(({ path, label, icon }) => (
            <button
              type="button"
              key={path}
              className={`nav-btn${pathname === path ? ' nav-btn--active' : ''}`}
              onClick={() => navigate(path)}
              title={label}
              aria-label={label}
            >
              {icon}
            </button>
          ))}
        </div>
        <div className="sidebar-bottom">
          <button
            type="button"
            className="theme-toggle"
            onClick={toggleTheme}
            title={theme === 'light' ? 'Тёмная тема' : 'Светлая тема'}
            aria-label="Переключить тему"
          >
            {theme === 'light' ? <IconMoon size={16} stroke={1.9} /> : <IconSun size={16} stroke={1.9} />}
          </button>
          <button type="button" className="nav-btn" onClick={onSettings} title="Настройки" aria-label="Настройки">
            <IconSettings2 stroke={1.9} size={19} />
          </button>
        </div>
      </aside>
    </>
  );
}
