import { useState, useEffect, useRef, useCallback } from 'react';
import { IconSettings2, IconX, IconHeartFilled, IconBug, IconAlertTriangle } from '@tabler/icons-react';
import { settingsStore } from '../lib/store';
import type { AppSettings, SplitTunnelSettings } from '../lib/types';
import { SPLIT_PRESETS } from '../lib/splitPresets';
import type { SplitPreset } from '../lib/splitPresets';
import { SetAutoStart, GetAutoStart, GetStartMinimized, SetStartMinimized, GetVersion, GenerateReport, GetObfsAccepted, SetObfsAccepted, SetObfsMode, GetSplitTunnel, SetSplitTunnel } from '../../wailsjs/go/backend/App';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { logStore } from '../lib/stores/logStore';
import { toastStore } from '../lib/stores/toastStore';
import './Settings.css';

interface Props {
  onClose: () => void;
}

export default function Settings({ onClose }: Props) {
  const [settings, setSettings] = useState<AppSettings>(() => settingsStore.get());
  const [version, setVersion] = useState('...');
  const copiedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [showObfsModal, setShowObfsModal] = useState(false);
  const [pendingObfsMode, setPendingObfsMode] = useState<'audio' | 'video'>('audio');
  const [splitRoutesText, setSplitRoutesText] = useState(() => settings.splitTunnel.routes.join('\n'));
  const [splitDomainsText, setSplitDomainsText] = useState(() => settings.splitTunnel.domains.join('\n'));
  const [splitError, setSplitError] = useState('');

  const update = useCallback(<K extends keyof AppSettings>(key: K, value: AppSettings[K]) => {
    setSettings(s => {
      const next = { ...s, [key]: value };
      settingsStore.save(next);
      return next;
    });
  }, []);

  // Сохраняет настройки split tunneling в бэкенд; возвращает нормализованный
  // вариант (одиночные IP → /32, host-биты маскируются).
  const saveSplit = useCallback(async (next: SplitTunnelSettings) => {
    try {
      const saved = await SetSplitTunnel(next);
      const normalized: SplitTunnelSettings = {
        enabled: !!saved.enabled,
        mode: saved.mode === 'exclude' ? 'exclude' : 'include',
        routes: saved.routes ?? [],
        domains: saved.domains ?? [],
      };
      update('splitTunnel', normalized);
      setSplitRoutesText(normalized.routes.join('\n'));
      setSplitDomainsText(normalized.domains.join('\n'));
      setSplitError('');
      return true;
    } catch (e) {
      setSplitError(e instanceof Error ? e.message : String(e));
      return false;
    }
  }, [update]);

  // Применяет изменение настроек вместе с текущим текстом полей ввода,
  // чтобы ввод не терялся при переключении режима/тумблера.
  const commitSplit = (patch: Partial<SplitTunnelSettings>) => {
    const routes = splitRoutesText
      .split(/[\n,;]+/)
      .map(s => s.trim())
      .filter(Boolean);
    const domains = splitDomainsText
      .split(/[\n,;]+/)
      .map(s => s.trim())
      .filter(Boolean);
    void saveSplit({ ...settings.splitTunnel, routes, domains, ...patch });
  };

  const parseEntries = (text: string) => text.split(/[\n,;]+/).map(s => s.trim()).filter(Boolean);

  // Пресет включён, когда все его подсети и домены уже есть в списках.
  const isPresetActive = (preset: SplitPreset) => {
    const routes = new Set(parseEntries(splitRoutesText));
    const domains = new Set(parseEntries(splitDomainsText));
    return preset.routes.every(r => routes.has(r))
      && (preset.domains ?? []).every(d => domains.has(d));
  };

  // Клик добавляет подсети и домены пресета, повторный клик — убирает.
  const togglePreset = (preset: SplitPreset) => {
    const nextRoutes = new Set(parseEntries(splitRoutesText));
    const nextDomains = new Set(parseEntries(splitDomainsText));
    const active = isPresetActive(preset);
    for (const route of preset.routes) {
      if (active) nextRoutes.delete(route); else nextRoutes.add(route);
    }
    for (const domain of preset.domains ?? []) {
      if (active) nextDomains.delete(domain); else nextDomains.add(domain);
    }
    const routes = Array.from(nextRoutes);
    const domains = Array.from(nextDomains);
    setSplitRoutesText(routes.join('\n'));
    setSplitDomainsText(domains.join('\n'));
    void saveSplit({ ...settings.splitTunnel, routes, domains });
  };

  const presetHint = (preset: SplitPreset) => {
    const parts: string[] = [];
    if (preset.routes.length) parts.push(`${preset.routes.length} подсетей`);
    if (preset.domains?.length) parts.push(`${preset.domains.length} доменов`);
    return `${parts.join(', ')} — нажмите, чтобы добавить или убрать`;
  };

  useEffect(() => {
    return () => { if (copiedTimerRef.current) clearTimeout(copiedTimerRef.current); };
  }, []);

  useEffect(() => {
    GetAutoStart().then(v => {
      if (v !== settings.autoStart) update('autoStart', v);
    }).catch(() => { toastStore.show('Не удалось загрузить настройки', 3000); });
    GetStartMinimized().then(v => {
      if (v !== settings.startMinimized) update('startMinimized', v);
    }).catch(() => {});
    GetObfsAccepted().then(v => {
      update('obfsAccepted', v);
    }).catch(() => {});
    GetSplitTunnel().then(v => {
      const split: SplitTunnelSettings = {
        enabled: !!v.enabled,
        mode: v.mode === 'exclude' ? 'exclude' : 'include',
        routes: v.routes ?? [],
        domains: v.domains ?? [],
      };
      update('splitTunnel', split);
      setSplitRoutesText(split.routes.join('\n'));
      setSplitDomainsText(split.domains.join('\n'));
    }).catch(() => {});
    GetVersion().then(setVersion).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleReport = async () => {
    const logs = logStore.getAll();
    const report = await GenerateReport(logs.map(e => ({
      level: e.level,
      message: e.message,
      time: e.time,
      count: e.count,
    })));
    const mode = settings.obfsMode || 'audio';
    const accepted = settings.obfsAccepted ? 'да' : 'нет';
    const transport = settings.turnTcp ? 'TCP' : 'UDP';
    const split = settings.splitTunnel.enabled
      ? `${settings.splitTunnel.mode === 'include' ? 'include' : 'exclude'} (${settings.splitTunnel.routes.length} routes, ${settings.splitTunnel.domains.length} domains)`
      : 'off';
    const full = report + `\n## Settings\n- Obfuscation: ${mode}\n- Obfs accepted: ${accepted}\n- Transport: ${transport}\n- Start minimized: ${settings.startMinimized ? 'yes' : 'no'}\n- Split tunneling: ${split}\n`;
    await navigator.clipboard.writeText(full);
    setCopiedReport(true);
    if (copiedTimerRef.current) clearTimeout(copiedTimerRef.current);
    copiedTimerRef.current = setTimeout(() => setCopiedReport(false), 2000);
  };

  const [copiedReport, setCopiedReport] = useState(false);

  const handleObfsClick = async (mode: 'audio' | 'video') => {
    if (settings.obfsAccepted) {
      update('obfsMode', mode);
      await SetObfsMode(mode);
    } else {
      setPendingObfsMode(mode);
      setShowObfsModal(true);
    }
  };

  const handleObfsAccept = async () => {
    try {
      await SetObfsAccepted(true);
      await SetObfsMode(pendingObfsMode);
      update('obfsAccepted', true);
      update('obfsMode', pendingObfsMode);
      setShowObfsModal(false);
    } catch {
      toastStore.show('Не удалось сохранить настройки', 3000);
    }
  };

  return (
    <>
      <div className="st-overlay" onClick={onClose}>
        <div className="st-modal" onClick={e => e.stopPropagation()}>
          <div className="st-header">
            <IconSettings2 stroke={2} size={20} />
            <span className="st-title">Настройки</span>
            <button type="button" className="st-close" onClick={onClose} aria-label="Закрыть"><IconX size={18} /></button>
          </div>

          <div className="st-row">
            <span>Запускать при старте</span>
            <button type="button" className={`st-toggle st-toggle--${settings.autoStart ? 'on' : 'off'}`} aria-label={settings.autoStart ? 'Отключить автозапуск' : 'Включить автозапуск'} onClick={() => {
              const next = !settings.autoStart;
              update('autoStart', next);
              SetAutoStart(next);
            }} />
          </div>

          <div className="st-row">
            <span>Запускать свёрнутым <span className="st-row-note">при автозапуске</span></span>
            <button
              type="button"
              className={`st-toggle st-toggle--${settings.startMinimized ? 'on' : 'off'}`}
              aria-label={settings.startMinimized ? 'Показывать окно при автозапуске' : 'Запускать свёрнутым'}
              onClick={() => {
                const next = !settings.startMinimized;
                update('startMinimized', next);
                SetStartMinimized(next).catch(() => {
                  toastStore.show('Не удалось сохранить настройки', 3000);
                });
              }}
            />
          </div>

          <div className="st-row">
            <span>Режим обфускации {!settings.obfsAccepted && <span className="st-badge">только latest</span>}</span>
            <div className="st-segment">
              <button type="button" className={`st-seg-btn${settings.obfsMode === 'audio' ? ' st-seg-btn--active' : ''}`} onClick={() => handleObfsClick('audio')}>Audio</button>
              <button type="button" className={`st-seg-btn${settings.obfsMode === 'video' ? ' st-seg-btn--active' : ''}`} onClick={() => handleObfsClick('video')}>Video</button>
            </div>
          </div>

          <div className="st-row">
            <span>Транспорт TURN</span>
            <div className="st-segment">
              <button type="button" className={`st-seg-btn${!settings.turnTcp ? ' st-seg-btn--active' : ''}`} onClick={() => update('turnTcp', false)}>UDP</button>
              <button type="button" className={`st-seg-btn${settings.turnTcp ? ' st-seg-btn--active' : ''}`} onClick={() => update('turnTcp', true)}>TCP</button>
            </div>
          </div>

          <div className="st-row st-split-row">
            <div className="st-split-head">
              <span>Раздельное туннелирование</span>
              <button
                type="button"
                className={`st-toggle st-toggle--${settings.splitTunnel.enabled ? 'on' : 'off'}`}
                aria-label={settings.splitTunnel.enabled ? 'Отключить раздельное туннелирование' : 'Включить раздельное туннелирование'}
                onClick={() => commitSplit({ enabled: !settings.splitTunnel.enabled })}
              />
            </div>

            {settings.splitTunnel.enabled && (
              <div className="st-split-body">
                <div className="st-segment st-split-mode">
                  <button
                    type="button"
                    className={`st-seg-btn${settings.splitTunnel.mode === 'include' ? ' st-seg-btn--active' : ''}`}
                    onClick={() => commitSplit({ mode: 'include' })}
                    title="В туннель пойдёт только перечисленное"
                  >Только список</button>
                  <button
                    type="button"
                    className={`st-seg-btn${settings.splitTunnel.mode === 'exclude' ? ' st-seg-btn--active' : ''}`}
                    onClick={() => commitSplit({ mode: 'exclude' })}
                    title="Перечисленное пойдёт напрямую, остальное — в туннель"
                  >Всё кроме</button>
                </div>

                <div className="st-split-presets">
                  <span className="st-split-presets-label">Пресеты:</span>
                  <div className="st-preset-chips">
                    {SPLIT_PRESETS.map(preset => (
                      <button
                        type="button"
                        key={preset.id}
                        className={`st-preset${isPresetActive(preset) ? ' st-preset--active' : ''}`}
                        onClick={() => togglePreset(preset)}
                        title={presetHint(preset)}
                      >
                        {preset.label}
                      </button>
                    ))}
                  </div>
                </div>

                <label className="st-split-label" htmlFor="st-split-routes">Адреса и подсети</label>
                <textarea
                  id="st-split-routes"
                  className="st-split-input"
                  value={splitRoutesText}
                  onChange={e => {
                    setSplitRoutesText(e.target.value);
                    if (splitError) setSplitError('');
                  }}
                  onBlur={() => commitSplit({})}
                  placeholder={'10.0.0.0/8\n192.168.1.0/24\n1.2.3.4'}
                  spellCheck={false}
                  rows={4}
                  aria-label="Список адресов и подсетей для раздельного туннелирования"
                />

                <label className="st-split-label" htmlFor="st-split-domains">Домены</label>
                <textarea
                  id="st-split-domains"
                  className="st-split-input"
                  value={splitDomainsText}
                  onChange={e => {
                    setSplitDomainsText(e.target.value);
                    if (splitError) setSplitError('');
                  }}
                  onBlur={() => commitSplit({})}
                  placeholder={'example.com\nyandex.ru\nhttps://ozon.ru'}
                  spellCheck={false}
                  rows={3}
                  aria-label="Список доменов для раздельного туннелирования"
                />

                {splitError && <div className="st-split-error">{splitError}</div>}

                <div className="st-split-hint">
                  IPv4-адреса и подсети (CIDR) и/или домены, по одному в строке.
                  Домены резолвятся в IPv4-адреса при подключении, поэтому при смене
                  адресов сервиса нужно переподключиться; маски (*.example.com)
                  не поддерживаются. В режиме «Только список» IPv6 на время
                  подключения блокируется, чтобы сайты не уходили мимо VPN.
                  {settings.splitTunnel.mode === 'include'
                    ? ' В туннель пойдёт только этот список.'
                    : ' Эти адреса пойдут напрямую, остальное — в туннель.'}
                  {' '}Изменения применяются при следующем подключении.
                </div>
              </div>
            )}
          </div>

          <div className="st-info">
            <div className="st-info-name">PWDTT</div>
            <div className="st-info-ver">v{version}</div>
          </div>

          <div className="st-actions">
            <button type="button" className={`st-action${copiedReport ? ' st-action--copied' : ''}`} onClick={handleReport}>
              <IconBug size={14} />
              {copiedReport ? 'Скопировано!' : 'Отчёт'}
            </button>
          </div>

          <button type="button" className="st-donate" onClick={() => BrowserOpenURL('https://pay.cloudtips.ru/p/1ea077ba')}>
            <IconHeartFilled size={16} />
            Поддержать проект
          </button>
        </div>
      </div>

      {showObfsModal && (
        <div className="st-overlay" onClick={() => setShowObfsModal(false)}>
          <div className="st-modal st-obfs-modal" onClick={e => e.stopPropagation()}>
            <div className="st-obfs-icon">
              <IconAlertTriangle size={32} />
            </div>
            <div className="st-obfs-title">Режим обфускации</div>
            <div className="st-obfs-text">
              Обфускация поддерживается только на последней версии сервера WDTT.
              При использовании на старых серверах соединение может не состояться.
              Если вы не уверены в совместимости, рекомендуется использовать режим "Audio".
            </div>
            <div className="st-obfs-text st-obfs-disclaimer">
              Разработчик не несёт ответственности за проблемы при использовании на несовместимых серверах.
            </div>
            <div className="st-obfs-actions">
              <button type="button" className="st-obfs-btn st-obfs-btn--cancel" onClick={() => setShowObfsModal(false)}>Отмена</button>
              <button type="button" className="st-obfs-btn st-obfs-btn--accept" onClick={handleObfsAccept}>Принимаю</button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
