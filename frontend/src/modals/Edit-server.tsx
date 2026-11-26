import { useState } from 'react';
import type React from 'react';
import { IconCircleHalf2, IconHash, IconEye, IconEyeOff, IconX } from '@tabler/icons-react';
import type { Server } from '../lib/types';
import Hash from './Hash';
import { SaveProfile, DeleteProfile } from '../../wailsjs/go/backend/App';
import { toastStore } from '../lib/stores/toastStore';

interface Props {
  server: Server;
  onClose: () => void;
  onSave: (server: Server) => void;
  onDelete: (id: string) => void;
}

export default function EditServer({ server, onClose, onSave, onDelete }: Props) {
  const [name, setName] = useState(server.name);
  const lastColon = server.host.lastIndexOf(':');
  const ip = lastColon !== -1 ? server.host.slice(0, lastColon) : server.host;
  const port0 = lastColon !== -1 ? server.host.slice(lastColon + 1) : '56000';
  const [serverIp, setServerIp] = useState(ip);
  const [serverPort, setServerPort] = useState(port0);
  const [password, setPassword] = useState(server.password);
  const [showPassword, setShowPassword] = useState(false);
  const [hashes, setHashes] = useState<[string,string,string,string]>(
    server.hashes ?? ['', '', '', '']
  );
  const [hashOpen, setHashOpen] = useState(false);

  const filledHashes = hashes.filter(h => h.trim()).length;
  const powerMax = Math.max(9, filledHashes * 27);
  const [power, setPower] = useState<number>(server.power ?? Math.max(9, filledHashes * 9));

  const handleHashSave = (h: [string, string, string, string]) => {
    setHashes(h);
    const filled = h.filter(x => x.trim()).length;
    const newMax = Math.max(9, filled * 27);
    setPower(p => Math.min(p, newMax) || Math.max(9, filled * 9));
  };

  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!name.trim() || !serverIp.trim() || saving) return;
    setSaving(true);
    const updated: Server = {
      ...server,
      name: name.trim(),
      host: `${serverIp.trim()}:${serverPort.trim() || '56000'}`,
      password,
      hashes,
      power,
    };
    // Сначала сохраняем новый, потом удаляем старый
    try {
      await SaveProfile(updated.name, {
        peer: updated.host,
        password: updated.password,
        hashes,
        workers: power,
        turn: '', port: '', device_id: '', listen: '',
        turn_tcp: false,
      });
    } catch {
      // Не двигаем UI-состояние: иначе localStorage разойдётся с диском,
      // а авто-реимпорт профилей создаст дубликат
      toastStore.show('Ошибка сохранения', 3000);
      setSaving(false);
      return;
    }
    if (server.name !== updated.name) {
      await DeleteProfile(server.name).catch(() => { toastStore.show('Ошибка удаления', 3000); });
    }
    onSave(updated);
    setSaving(false);
    onClose();
  };

  const handleDelete = async () => {
    await DeleteProfile(server.name).catch(() => {});
    onDelete(server.id);
    onClose();
  };

  return (
    <>
      <style>{`
        .es-overlay { position: fixed; inset: 0; background: var(--overlay-bg); backdrop-filter: blur(4px); display: flex; align-items: center; justify-content: center; z-index: 100; animation: overlay-in 0.25s ease-out; }
        .es-modal { background: var(--surface); border-radius: 16px; padding: 18px; width: 360px; max-width: 95vw; box-shadow: var(--shadow); border: 1px solid var(--border); max-height: 90vh; overflow-y: auto; animation: modal-in 0.25s ease-out; }
        .es-header { display: flex; align-items: center; gap: 9px; margin-bottom: 14px; color: var(--text); }
        .es-title { font-size: 15px; font-weight: 600; flex: 1; color: var(--text); }
        .es-close { display: flex; align-items: center; justify-content: center; width: 28px; height: 28px; background: none; border: none; border-radius: 8px; cursor: pointer; color: var(--text-3); transition: background 0.15s, color 0.15s; }
        .es-close:hover { background: var(--bg-3); color: var(--text); }
        .es-input { width: 100%; padding: 9px 12px; border: 1px solid var(--input-border); border-radius: 9px; font-size: 13px; font-family: 'Geist', sans-serif; outline: none; margin-bottom: 8px; box-sizing: border-box; color: var(--text); background: var(--input-bg); transition: border-color 0.15s, box-shadow 0.15s; }
        .es-input:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-dim); }
        .es-input::placeholder { color: var(--text-4); }
        .es-hash-btn { width: 100%; margin-top: 3px; margin-bottom: 8px; padding: 10px; border: 1px solid var(--border); border-radius: 9px; background: var(--surface); color: var(--text); font-size: 13px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 7px; transition: border-color 0.15s, background 0.15s; }
        .es-hash-btn:hover { border-color: var(--accent); background: var(--bg-2); }
        .es-btn-row { display: flex; gap: 8px; margin-top: 3px; }
        .es-btn { flex: 1; padding: 10px; border: none; border-radius: 9px; font-size: 13px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; transition: background 0.15s, opacity 0.15s; }
        .es-btn--save { background: var(--accent); color: var(--accent-fg); }
        .es-btn--save:hover:not(:disabled) { background: var(--accent-hover); }
        .es-btn--save:disabled { opacity: 0.4; cursor: not-allowed; }
        .es-btn--delete { background: rgba(239, 68, 68, 0.1); color: var(--danger); border: 1px solid rgba(239, 68, 68, 0.3); }
        .es-btn--delete:hover { background: var(--danger); color: #fff; border-color: var(--danger); }
        .es-slider-wrap { padding: 3px 0 9px; border-bottom: 1px solid var(--border-2); margin-bottom: 8px; }
        .es-slider-label { display: flex; justify-content: space-between; font-size: 12.5px; color: var(--text); margin-bottom: 7px; }
        .es-slider { width: 100%; -webkit-appearance: none; appearance: none; height: 4px; border-radius: 2px; outline: none; cursor: pointer; background: linear-gradient(to right, var(--accent) calc(var(--v) * 1%), var(--border) calc(var(--v) * 1%)); }
        .es-slider::-webkit-slider-thumb { -webkit-appearance: none; width: 16px; height: 16px; border-radius: 50%; background: var(--surface); border: 2px solid var(--accent); cursor: pointer; }
        .es-pw-wrap { position: relative; display: flex; align-items: center; margin-bottom: 8px; }
        .es-pw-toggle { position: absolute; right: 9px; background: none; border: none; cursor: pointer; color: var(--text-4); padding: 0; display: flex; align-items: center; }
        .es-pw-toggle:hover { color: var(--text-2); }
      `}</style>
      <div className="es-overlay">
        <div className="es-modal" onClick={e => e.stopPropagation()}>
          <div className="es-header">
            <IconCircleHalf2 size={18} />
            <span className="es-title">Редактирование сервера</span>
            <button type="button" className="es-close" onClick={onClose} aria-label="Закрыть"><IconX size={16} /></button>
          </div>

          <input className="es-input" placeholder="Название сервера" value={name} onChange={e => setName(e.target.value)} />
          <div style={{ display: 'flex', gap: 8 }}>
            <input className="es-input" style={{ flex: 1 }} placeholder="IP сервера" value={serverIp} onChange={e => setServerIp(e.target.value)} />
            <input className="es-input" style={{ width: 100 }} placeholder="Порт" value={serverPort} onChange={e => setServerPort(e.target.value)} />
          </div>
          <div className="es-pw-wrap">
            <input className="es-input" placeholder="Пароль туннеля" type={showPassword ? 'text' : 'password'} value={password} onChange={e => setPassword(e.target.value)} style={{ paddingRight: 36, marginBottom: 0 }} />
            <button type="button" className="es-pw-toggle" onClick={() => setShowPassword(v => !v)} aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}>
              {showPassword ? <IconEyeOff size={18} /> : <IconEye size={18} />}
            </button>
          </div>

          <div className="es-slider-wrap">
            <div className="es-slider-label">
              <span>Мощность (всего потоков)</span>
              <span>{filledHashes === 0 ? 'нет хешей' : power}</span>
            </div>
            <input
              type="range" min={9} max={powerMax} step={9}
              value={Math.min(power, powerMax)}
              className="es-slider"
              disabled={filledHashes === 0}
              style={{ '--v': filledHashes > 0 ? Math.round((Math.min(power, powerMax) - 9) / Math.max(powerMax - 9, 1) * 100) : 0 } as React.CSSProperties}
              onChange={e => setPower(+e.target.value)}
              aria-label="Мощность — всего потоков, шаг 9"
            />
          </div>

          <button
            type="button"
            className="es-hash-btn"
            onClick={() => setHashOpen(true)}
          >
            <IconHash stroke={2} size={16} />
            Хеши ({filledHashes}/4)
          </button>

          <div className="es-btn-row">
            <button type="button" className="es-btn es-btn--save" onClick={handleSave} disabled={!name.trim() || !serverIp.trim() || saving}>
              {saving ? 'Сохранение...' : 'Сохранить'}
            </button>
            <button type="button" className="es-btn es-btn--delete" onClick={handleDelete}>Удалить</button>
          </div>
        </div>
      </div>
      {hashOpen && (
        <Hash
          hashes={hashes}
          onClose={() => setHashOpen(false)}
          onSave={handleHashSave}
        />
      )}
    </>
  );
}
