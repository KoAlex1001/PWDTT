export interface Server {
  id: string;
  name: string;
  host: string;
  password: string;
  deviceId?: string;
  ping?: number;
  icon?: string;
  hashes?: [string, string, string, string];
  power?: number;
}

export type SplitTunnelMode = 'include' | 'exclude';

export interface SplitTunnelSettings {
  enabled: boolean;
  mode: SplitTunnelMode;
  routes: string[];
  domains: string[];
}

export interface AppSettings {
  autoStart: boolean;
  startMinimized: boolean;
  obfsMode: 'audio' | 'video';
  obfsAccepted: boolean;
  turnTcp: boolean;
  splitTunnel: SplitTunnelSettings;
}

export type TunnelState = 'idle' | 'connecting' | 'connected' | 'disconnecting';

export const DEFAULT_SETTINGS: AppSettings = {
  autoStart: true,
  startMinimized: true,
  obfsMode: 'audio',
  obfsAccepted: false,
  turnTcp: false,
  splitTunnel: { enabled: false, mode: 'include', routes: [], domains: [] },
};
