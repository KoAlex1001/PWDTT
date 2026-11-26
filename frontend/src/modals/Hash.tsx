import { useState } from 'react';
import { IconHash, IconX } from '@tabler/icons-react';
import { toastStore } from '../lib/stores/toastStore';

const normalize = (v: string) => {
  const idx = v.toLowerCase().indexOf('/call/join/');
  if (idx !== -1) v = v.slice(idx + '/call/join/'.length);
  return v.split(/[?#/]/)[0].trim();
};

interface Props {
  hashes: [string, string, string, string];
  onClose: () => void;
  onSave: (hashes: [string, string, string, string]) => void;
}

export default function Hash({ hashes, onClose, onSave }: Props) {
  const [values, setValues] = useState<[string, string, string, string]>([...hashes] as [string, string, string, string]);

  const set = (i: number, v: string) => {
    const next = [...values] as [string, string, string, string];
    next[i] = v;
    setValues(next);
  };

  const save = () => {
    const normalized = values.map(normalize) as [string, string, string, string];
    const nonEmpty = normalized.filter(v => v !== '');
    if (new Set(nonEmpty).size !== nonEmpty.length) {
      toastStore.show('Обнаружены дублирующиеся хеши');
      return;
    }
    onSave(normalized);
    onClose();
  };

  return (
    <>
      <style>{`
        .hash-overlay { position: fixed; inset: 0; background: var(--overlay-bg); backdrop-filter: blur(4px); display: flex; align-items: center; justify-content: center; z-index: 200; animation: overlay-in 0.25s ease-out; }
        .hash-modal { background: var(--surface); border-radius: 16px; padding: 18px; width: 360px; max-width: 95vw; box-shadow: var(--shadow); border: 1px solid var(--border); animation: modal-in 0.25s ease-out; }
        .hash-header { display: flex; align-items: center; gap: 9px; margin-bottom: 14px; color: var(--text); }
        .hash-title { font-size: 15px; font-weight: 600; flex: 1; color: var(--text); }
        .hash-close { display: flex; align-items: center; justify-content: center; width: 28px; height: 28px; background: none; border: none; border-radius: 8px; cursor: pointer; color: var(--text-3); transition: background 0.15s, color 0.15s; }
        .hash-close:hover { background: var(--bg-3); color: var(--text); }
        .hash-input { width: 100%; padding: 9px 12px; border: 1px solid var(--input-border); border-radius: 9px; font-size: 13px; font-family: 'Geist', sans-serif; outline: none; margin-bottom: 8px; box-sizing: border-box; color: var(--text); background: var(--input-bg); transition: border-color 0.15s, box-shadow 0.15s; }
        .hash-input:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-dim); }
        .hash-input::placeholder { color: var(--text-4); }
        .hash-btn { width: 100%; padding: 10px; border: none; border-radius: 9px; background: var(--accent); color: var(--accent-fg); font-size: 13px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; margin-top: 3px; transition: background 0.15s; }
        .hash-btn:hover { background: var(--accent-hover); }
      `}</style>
      <div className="hash-overlay">
        <div className="hash-modal" onClick={e => e.stopPropagation()}>
          <div className="hash-header">
            <IconHash stroke={2} size={18} />
            <span className="hash-title">Hash</span>
            <button type="button" className="hash-close" onClick={onClose} aria-label="Закрыть"><IconX size={16} /></button>
          </div>
          {values.map((v, i) => (
            <input key={['hash-a', 'hash-b', 'hash-c', 'hash-d'][i]} className="hash-input" placeholder={`Hash - ${i + 1}`} value={v} onChange={e => set(i, e.target.value)} />
          ))}
          <button type="button" className="hash-btn" onClick={save}>Сохранить</button>
        </div>
      </div>
    </>
  );
}
