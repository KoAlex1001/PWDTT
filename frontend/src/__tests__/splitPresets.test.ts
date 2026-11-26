import { describe, it, expect } from 'vitest';
import { SPLIT_PRESETS } from '../lib/splitPresets';

const IPV4_CIDR = /^(?:(?:25[0-5]|2[0-4]\d|1?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|1?\d?\d)\/(?:3[0-2]|[12]?\d)$/;

describe('SPLIT_PRESETS', () => {
  it('содержит пресет локальной сети', () => {
    const local = SPLIT_PRESETS.find(p => p.id === 'local');
    expect(local).toBeDefined();
    expect(local!.routes).toContain('192.168.0.0/16');
    expect(local!.routes).toContain('10.0.0.0/8');
  });

  it('id и названия пресетов уникальны', () => {
    expect(new Set(SPLIT_PRESETS.map(p => p.id)).size).toBe(SPLIT_PRESETS.length);
    expect(new Set(SPLIT_PRESETS.map(p => p.label)).size).toBe(SPLIT_PRESETS.length);
  });

  it('все адреса — корректные IPv4 CIDR без дубликатов', () => {
    for (const preset of SPLIT_PRESETS) {
      expect(preset.routes.length).toBeGreaterThan(0);
      expect(new Set(preset.routes).size).toBe(preset.routes.length);
      for (const route of preset.routes) {
        expect(route, `${preset.id}: ${route}`).toMatch(IPV4_CIDR);
      }
    }
  });
});
