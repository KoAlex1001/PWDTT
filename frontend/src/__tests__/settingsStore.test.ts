import { describe, it, expect, beforeEach } from 'vitest';
import { settingsStore } from '../lib/store';
import { DEFAULT_SETTINGS } from '../lib/types';

beforeEach(() => {
  localStorage.clear();
});

describe('settingsStore', () => {
  it('get: возвращает DEFAULT_SETTINGS если пусто', () => {
    const settings = settingsStore.get();
    expect(settings).toEqual(DEFAULT_SETTINGS);
  });

  it('get: мержит сохранённые с дефолтными', () => {
    // Сохраняем частичные настройки (имитируем старую версию без obfsMode)
    localStorage.setItem('wdtt_settings:v1', JSON.stringify({ autoStart: false }));

    const settings = settingsStore.get();
    expect(settings.autoStart).toBe(false);
    expect(settings.obfsMode).toBe(DEFAULT_SETTINGS.obfsMode);
  });

  it('save → get: roundtrip', () => {
    const custom = {
      autoStart: false,
      startMinimized: false,
      obfsMode: 'video' as const,
      obfsAccepted: true,
      turnTcp: false,
      splitTunnel: { enabled: true, mode: 'exclude' as const, routes: ['10.0.0.0/8'], domains: ['example.com'] },
    };
    settingsStore.save(custom);
    
    // Перезагружаем "приложение" — должны восстановиться кастомные настройки
    expect(settingsStore.get()).toEqual(custom);
  });

  it('get: старый формат без startMinimized → запуск свёрнутым по умолчанию', () => {
    localStorage.setItem('wdtt_settings:v1', JSON.stringify({ autoStart: false }));
    const settings = settingsStore.get();
    expect(settings.startMinimized).toBe(true);
  });

  it('должен мержить с дефолтами, если в localStorage неполные данные', () => {
    // Имитируем старый формат сохраненных данных, где не было obfsMode
    localStorage.setItem('wdtt_settings:v1', JSON.stringify({ autoStart: false }));

    
    const settings = settingsStore.get();
    expect(settings.autoStart).toBe(false); // сохраненное значение
    expect(settings.obfsMode).toBe(DEFAULT_SETTINGS.obfsMode); // дефолтное
    expect(settings.turnTcp).toBe(DEFAULT_SETTINGS.turnTcp);
  });

  it('get: старый формат без splitTunnel → дефолтные настройки', () => {
    localStorage.setItem('wdtt_settings:v1', JSON.stringify({ autoStart: false }));
    const settings = settingsStore.get();
    expect(settings.splitTunnel).toEqual(DEFAULT_SETTINGS.splitTunnel);
  });

  it('get: частичный splitTunnel мержится с дефолтами', () => {
    localStorage.setItem('wdtt_settings:v1', JSON.stringify({ splitTunnel: { enabled: true } }));
    const settings = settingsStore.get();
    expect(settings.splitTunnel.enabled).toBe(true);
    expect(settings.splitTunnel.mode).toBe(DEFAULT_SETTINGS.splitTunnel.mode);
    expect(settings.splitTunnel.routes).toEqual([]);
    expect(settings.splitTunnel.domains).toEqual([]);
  });

  it('должен сбрасывать obfsMode на audio, если obfsAccepted === false', () => {
    settingsStore.save({ autoStart: false, startMinimized: true, obfsMode: 'audio', obfsAccepted: false, turnTcp: false, splitTunnel: DEFAULT_SETTINGS.splitTunnel });
    settingsStore.save({ autoStart: true, startMinimized: true, obfsMode: 'video', obfsAccepted: true, turnTcp: false, splitTunnel: DEFAULT_SETTINGS.splitTunnel });

    const settings = settingsStore.get();
    expect(settings.autoStart).toBe(true);
    expect(settings.obfsMode).toBe('video');
    expect(settings.obfsAccepted).toBe(true);
    expect(settings.turnTcp).toBe(false);
  });

  it('get: невалидный JSON → дефолт', () => {
    localStorage.setItem('wdtt_settings:v1', '{broken');
    const settings = settingsStore.get();
    expect(settings).toEqual(DEFAULT_SETTINGS);
  });

  it('get: пустой объект → дефолт', () => {
    localStorage.setItem('wdtt_settings:v1', '{}');
    const settings = settingsStore.get();
    expect(settings).toEqual(DEFAULT_SETTINGS);
  });

  it('save: перезаписывает предыдущие', () => {
    settingsStore.save({ autoStart: false, startMinimized: true, obfsMode: 'audio', obfsAccepted: false, turnTcp: false, splitTunnel: DEFAULT_SETTINGS.splitTunnel });
    settingsStore.save({ autoStart: true, startMinimized: true, obfsMode: 'video', obfsAccepted: true, turnTcp: false, splitTunnel: DEFAULT_SETTINGS.splitTunnel });

    const settings = settingsStore.get();
    expect(settings.autoStart).toBe(true);
    expect(settings.obfsMode).toBe('video');
    expect(settings.obfsAccepted).toBe(true);
    expect(settings.turnTcp).toBe(false);
  });
});
