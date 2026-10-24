// Messages with Snag (via snag-host; see PROTOCOL.md) and between the extension's own parts.
import type { FoundLink } from "../media/links";
import type {
  Cookie,
  Download,
  LinksFound,
  PlaylistFound,
  ProbeResult,
  Settings,
} from "./wire.gen";

export type {
  Choice,
  Cookie,
  Download,
  Playlist,
  PlaylistFound,
  ProbeResult,
  Settings,
  Subtitle,
} from "./wire.gen";

export type Kind = "http" | "ytdlp";

export type StreamType = "hls" | "dash";

/** clear_done and the *_all actions take no id. */
export type ControlAction =
  | "pause"
  | "resume"
  | "retry"
  | "remove"
  | "open"
  | "reveal"
  | "clear_done"
  | "pause_all"
  | "resume_all";

/** Sent along so sites that check them answer as they would the browser. */
export interface Credentials {
  headers: Record<string, string>;
  cookies: Cookie[];
}

export interface AddRequest extends Credentials {
  type: "add";
  url: string;
  kind: Kind;
  page: string;
  title: string;
  label: string;
  size: number;
  filename?: string;
  format?: string;
  sort?: string;
  audio?: string;
  subs?: string;
  sidecar?: boolean;
  sub_auto?: boolean;
  sub_url?: string;
  sub_text?: string;
  /** Seconds. */
  clip_from?: number;
  clip_to?: number;
  checksum?: string;
  checksum_files?: string[];
}

/** Only ever sent to Snag on this computer. */
export interface SpotifySession {
  token: string;
  client_token: string;
  app_version: string;
  user_agent: string;
  hashes: Record<string, string>;
}

export interface HaveItem {
  url: string;
  label?: string;
  filename?: string;
  format?: string;
  audio?: string;
}

export type Request =
  | { type: "ping" }
  | { type: "subscribe" }
  | AddRequest
  | { type: "have"; page: string; title: string; items: HaveItem[] }
  | ({ type: "probe"; url: string; stream: boolean } & Credentials)
  | ({ type: "sniff"; url: string } & Credentials)
  | ({
      type: "playlist";
      url: string;
      title?: string;
      /** TMDB API mirror the page loaded its show from, without "/3". */
      tmdb?: string;
      spotify?: SpotifySession;
      /** false: only report whether there is a list, without opening Snag's window. */
      open?: boolean;
      /** Open the window ready to follow the list. */
      subscribe?: boolean;
      feeds?: string[];
      /** The page itself was already checked; look only in feeds. */
      feeds_only?: boolean;
    } & Credentials)
  | ({ type: "resolved"; token: string; url: string } & Credentials)
  | { type: "resolve_failed"; token: string; error: string }
  | ({
      type: "links";
      page: string;
      title: string;
      source: "page" | "selection";
      links: FoundLink[];
    } & Credentials)
  | { type: "kind"; url: string }
  | { type: "show"; id: string }
  | { type: ControlAction; id?: string; delete_file?: boolean };

export interface Replies {
  ping: { version: number; ytdlp: boolean };
  subscribe: { downloads: Download[] };
  add: { download: Download; duplicate: boolean };
  have: { have: (Download | null)[] };
  probe: { probe: ProbeResult };
  sniff: { kind: StreamType | null; text?: string };
  /** null for a single-video page. */
  playlist: { playlist: PlaylistFound | null };
  links: { links: LinksFound };
  kind: { kind: Kind };
}

export type ReplyTo<T extends Request["type"]> = T extends keyof Replies ? Replies[T] : object;

export type FromSnag =
  | { type: "reply"; rid: number; ok: boolean; error?: string; [field: string]: unknown }
  | { type: "added" | "update"; download: Download }
  | { type: "removed"; id: string }
  /** See background/resolve.ts. */
  | { type: "resolve"; token: string; page: string }
  | { type: "config"; config: Settings }
  | { type: "ready" }
  | { type: "daemon_gone" }
  | { type: "error"; error: string };

export type MediaType = "video" | "audio" | StreamType | "subtitles";

export interface PlaylistInfo {
  type: "dash" | "master" | "subtitles" | "audio" | "media";
  duration?: number; // seconds
  variants?: number; // a master's qualities
  height?: number; // a master's highest
  live?: boolean;
}

export interface PageFound {
  media: Media[];
  captions: Media[];
}

export interface Media {
  key: string; // the address without its query, so the same file is listed once
  url: string;
  media: MediaType;
  kind: Kind;
  mime: string;
  size: number;
  time: number; // first seen (ms since 1970)
  frame: number;
  referer?: string;
  filename?: string;
  info?: PlaylistInfo;
}

export interface Item {
  url: string;
  kind: Kind;
  media?: MediaType;
  mime?: string;
  size?: number;
  filename?: string;
  format?: string;
  sort?: string;
  audio?: string;
  subs?: string;
  sidecar?: boolean;
  sub_auto?: boolean;
  sub_url?: string;
  sub_text?: string;
  clip_from?: number;
  clip_to?: number;
  label?: string;
  title?: string;
}

export type Added = Download & { duplicate: boolean };

export interface SeenEntry {
  url: string;
  initiator: string;
}

export interface ExtensionSettings {
  overlay: boolean; // the Download button on videos
  intercept: boolean; // take over the browser's downloads
  minSize: number; // bytes; any download this big is taken over, whatever its type
}

export interface Commands {
  settings: { args: object; result: ExtensionSettings };
  overlay: { args: { visible?: boolean }; result: { visible: boolean } };
  seen: { args: { entries: SeenEntry[] }; result: PageFound };
  media: { args: object; result: Media[] };
  probe: { args: { frameUrl?: string }; result: ProbeResult };
  probeStream: { args: { url: string }; result: ProbeResult };
  download: { args: { item: Item; referer?: string }; result: Added };
  have: { args: { items: HaveItem[] }; result: (Download | null)[] };
  control: { args: { action: ControlAction; id?: string; deleteFile?: boolean }; result: unknown };
  pageTitle: { args: object; result: string };
  show: { args: { id?: string }; result: unknown };
  /** tmdb: the TMDB API mirror the page read its show from, per the page's own resource list. */
  playlist: {
    args: { url?: string; open?: boolean; tmdb?: string; subscribe?: boolean };
    result: PlaylistFound | null;
  };
  status: { args: object; result: { ok: boolean; error?: string } };
}

export type CommandName = keyof Commands;

/** tabId defaults to the sender's tab. */
export type Command = {
  [C in CommandName]: { cmd: C; tabId?: number } & Commands[C]["args"];
}[CommandName];

export type Answer<T = unknown> = { ok: true; value: T } | { ok: false; error: string };

export interface TitleQuestion {
  cmd: "title";
}

export interface FeedsQuestion {
  cmd: "feeds";
}

export interface ChecksumQuestion {
  cmd: "checksums";
  url: string;
}

export type OverlayMessage = { type: "overlay"; visible: boolean };
