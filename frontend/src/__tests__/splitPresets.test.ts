import { describe, it, expect } from 'vitest';
import { SPLIT_PRESETS } from '../lib/splitPresets';

const IPV4_CIDR = /^(?:(?:25[0-5]|2[0-4]\d|1?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|1?\d?\d)\/(?:3[0-2]|[12]?\d)$/;
const DOMAIN = /^(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$/i;

// Бэкенд маскирует host-биты: пресет с невыровненным CIDR никогда не станет активным.
const ipToInt = (ip: string) => ip.split('.').reduce((acc, octet) => (acc * 256 + Number(octet)) >>> 0, 0);

describe('SPLIT_PRESETS', () => {
  it('содержит пресет локальной сети', () => {
    const local = SPLIT_PRESETS.find(p => p.id === 'local');
    expect(local).toBeDefined();
    expect(local!.routes).toContain('192.168.0.0/16');
    expect(local!.routes).toContain('10.0.0.0/8');
  });

  it('содержит пресеты популярных сервисов', () => {
    const expected: Record<string, string> = {
      instagram: '157.240.0.0/17',
      youtube: '142.250.0.0/15',
      telegram: '91.108.0.0/20',
      ai: 'chatgpt.com',
      rutracker: 'rutracker.org',
    };
    for (const [id, entry] of Object.entries(expected)) {
      const preset = SPLIT_PRESETS.find(p => p.id === id);
      expect(preset, id).toBeDefined();
      expect([...preset!.routes, ...(preset!.domains ?? [])], id).toContain(entry);
    }
  });

  it('id и названия пресетов уникальны', () => {
    expect(new Set(SPLIT_PRESETS.map(p => p.id)).size).toBe(SPLIT_PRESETS.length);
    expect(new Set(SPLIT_PRESETS.map(p => p.label)).size).toBe(SPLIT_PRESETS.length);
  });

  it('адреса и домены корректны, без дубликатов', () => {
    for (const preset of SPLIT_PRESETS) {
      const domains = preset.domains ?? [];
      expect(preset.routes.length + domains.length).toBeGreaterThan(0);
      expect(new Set(preset.routes).size).toBe(preset.routes.length);
      expect(new Set(domains).size).toBe(domains.length);
      for (const route of preset.routes) {
        expect(route, `${preset.id}: ${route}`).toMatch(IPV4_CIDR);
        const [addr, bits] = route.split('/');
        const mask = (+bits === 0 ? 0 : (0xffffffff << (32 - +bits)) >>> 0);
        expect((ipToInt(addr) & mask) >>> 0, `${preset.id}: ${route}`).toBe(ipToInt(addr));
      }
      for (const domain of domains) {
        expect(domain, `${preset.id}: ${domain}`).toMatch(DOMAIN);
      }
    }
  });
});
