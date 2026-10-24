// What Snag sends the extension. Made from the Go types by internal/protocol's
// TestTypeScriptTypesAreCurrent; don't edit by hand.

export interface Download {
  id: string;
  url: string;
  page: string;
  title: string;
  kind: string;
  label: string;
  status: string;
  path: string;
  name: string;
  size: number;
  estimated: boolean;
  done: number;
  speed: number;
  eta: number | null;
  error: string;
  created: number;
  finished: number;
  connections: number;
  resumable: boolean;
  check: Check | null;
}

export interface Check {
  algo: string;
  from: string;
  result: string;
}

export interface Settings {
  download_dir: string;
  connections: number;
  max_active: number;
  notify: boolean;
  ask: boolean;
  recent_dirs: string[];
}

export interface ProbeResult {
  supported: boolean;
  reason?: string;
  title?: string;
  thumbnail?: string;
  duration?: number;
  site?: string;
  url?: string;
  choices?: Choice[];
  variants?: string[];
  subtitles?: Subtitle[];
  playlist?: Playlist | null;
}

export interface Choice {
  label: string;
  detail: string;
  format: string;
  sort?: string;
  size: number;
  height?: number;
  audioOnly?: boolean;
  audio?: string;
}

export interface Subtitle {
  code: string;
  name: string;
  auto?: boolean;
}

export interface Playlist {
  title: string;
  count: number;
}

export interface PlaylistFound {
  id: string;
  title: string;
  count: number;
}

export interface LinksFound {
  id: string;
  count: number;
}

export interface Cookie {
  name: string;
  value: string;
  domain: string;
  path?: string;
  secure?: boolean;
  hostOnly?: boolean;
  expirationDate?: number;
}
