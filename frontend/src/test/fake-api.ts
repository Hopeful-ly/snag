// The Go side in memory: every call is recorded, and calls can fail or wait. Used by tests and
// preview.html.
import type {
  Added,
  Api,
  Browser,
  Clash,
  Countdown,
  Download,
  Found,
  Handlers,
  Link,
  LinksAdded,
  LinksPick,
  LinksView,
  Phone,
  Player,
  PlaylistAdded,
  PlaylistPick,
  PlaylistView,
  Queue,
  QueueState,
  SaveAnswer,
  Settings,
  Subscription,
  SubscriptionChange,
} from "../lib/api/client";
import { defaultNaming } from "../lib/naming";
import { NameKind } from "../../bindings/github.com/Roham1299/snag/internal/playlist/models";
import { Group } from "../../bindings/github.com/Roham1299/snag/internal/links/models";

const MB = 2 ** 20;
const minute = 60_000;

export function sampleDownload(fields: Partial<Download>): Download {
  return {
    id: "d1",
    url: "https://example.com/file.zip",
    page: "",
    title: "",
    kind: "http",
    label: "",
    status: "queued",
    name: "file.zip",
    path: "",
    directory: "/home/me/Downloads",
    error: "",
    note: "",
    size: 0,
    estimated: false,
    done: 0,
    speed: 0,
    eta: null,
    connections: 0,
    resumable: true,
    created: Date.now(),
    finished: 0,
    watch: "",
    watchIn: null,
    queue: "main",
    position: 0,
    subscription: "",
    where: false,
    check: null,
    stage: "",
    stageDone: 0,
    stageOf: 0,
    pieces: 0,
    piecesDone: 0,
    heard: 0,
    ...fields,
  };
}

// sampleQueues are Main, running; Tonight, on a schedule; and Big games, stopped.
export function sampleQueues(): QueueState[] {
  return [
    {
      id: "main",
      name: "Main",
      atOnce: 3,
      retries: 3,
      retryWait: 5,
      schedule: { on: false, start: "", stop: "", days: [] },
      running: true,
    },
    {
      id: "night",
      name: "Tonight",
      atOnce: 2,
      retries: 3,
      retryWait: 5,
      schedule: { on: true, start: "02:00", stop: "07:00", days: [] },
      whenDone: "shutdown",
      running: false,
    },
    {
      id: "games",
      name: "Big games",
      atOnce: 1,
      retries: 3,
      retryWait: 5,
      schedule: { on: false, start: "", stop: "", days: [] },
      speedLimit: 2 * MB,
      running: false,
    },
  ];
}

// samples are one download in each state, newest first, in Main's line oldest first.
export function samples(now = Date.now()): Download[] {
  const list = [
    sampleDownload({
      id: "pending",
      url: "https://cdn.example.com/v/master.m3u8",
      page: "https://www.example.com/watch/beach",
      kind: "stream",
      label: "1080p",
      status: "pending",
      name: "A Day at the Beach",
      title: "A Day at the Beach",
      size: 734 * MB,
      estimated: true,
      created: now - minute,
    }),
    sampleDownload({
      id: "running",
      url: "https://releases.example.com/ubuntu-26.04-desktop-amd64.iso",
      status: "downloading",
      name: "ubuntu-26.04-desktop-amd64.iso",
      size: 5800 * MB,
      done: 2400 * MB,
      speed: 18 * MB,
      eta: 189,
      connections: 8,
      created: now - 2 * minute,
    }),
    sampleDownload({
      id: "paused",
      url: "https://files.example.com/Holiday-in-Lisbon.mp4",
      status: "paused",
      name: "Holiday-in-Lisbon.mp4",
      size: 420 * MB,
      done: 130 * MB,
      created: now - 3 * minute,
    }),
    sampleDownload({
      id: "failed",
      url: "https://files.example.com/Nivalis-Nights.zip",
      status: "error",
      name: "Nivalis-Nights.zip",
      error: "server said 404 Not Found",
      created: now - 4 * minute,
    }),
    sampleDownload({
      id: "done",
      url: "https://docs.example.com/report_2026.pdf",
      page: "https://www.example.com/reports",
      status: "done",
      name: "report_2026.pdf",
      path: "/home/me/Downloads/report_2026.pdf",
      size: 2.4 * MB,
      done: 2.4 * MB,
      created: now - 60 * minute,
      finished: now - 50 * minute,
    }),
  ];
  return list.map((d, i) => ({ ...d, position: list.length - i }));
}

// samplePlaylist is a show of four episodes in two seasons, the second one playing, the last
// without a title.
export function samplePlaylist(): PlaylistView {
  const episode = (n: number) => ({
    url: `https://v.example.com/lanterns/1/${n}`,
    title: `Episode ${n}`,
    duration: 1500,
    group: n < 3 ? "Season 1" : "Season 2",
  });
  return {
    id: "p1",
    title: "Lanterns: Season 1",
    site: "Example",
    page: "https://v.example.com/lanterns/1/2",
    entries: [
      episode(1),
      episode(2),
      episode(3),
      { url: "https://v.example.com/lanterns/1/4", group: "Season 2" },
    ],
    current: 1,
    folder: "/home/me/Downloads/Lanterns Season 1",
    folderName: "Lanterns Season 1",
    naming: defaultNaming.episodes,
    kind: NameKind.KindEpisodes,
    defaults: defaultNaming,
    fields: {
      songs: ["artist", "title", "album", "track", "year", "n", "playlist"].map((name) => ({
        name,
        about: "",
      })),
      videos: ["n", "title", "artist", "playlist"].map((name) => ({ name, about: "" })),
      episodes: ["show", "season", "episode", "title"].map((name) => ({ name, about: "" })),
    },
    quality: "best",
    qualities: [
      { id: "best", label: "Best" },
      { id: "720", label: "720p" },
      { id: "mp3", label: "Audio MP3", audioOnly: true },
    ],
    canFollow: true,
    following: null,
  };
}

// sampleLinks is a download page's links: two archives (one in a folder), a program, a PDF
// behind a link without a name, a video page, a web page and an image.
function sampleLinks(): LinksView {
  const link = (url: string, name: string, group: Group, more: Partial<Link> = {}): Link => ({
    url: `https://example.org/pub/${url}`,
    name,
    group,
    ...more,
  });
  return {
    id: "l1",
    page: "https://example.org/pub/",
    title: "Example downloads",
    source: "page",
    folder: "/home/me/Downloads",
    links: [
      link("tool-1.0.zip", "tool-1.0.zip", Group.Archives, { text: "Tool 1.0" }),
      link("iso/distro.iso", "distro.iso", Group.Archives, { dir: "iso" }),
      link("setup.exe", "setup.exe", Group.Programs),
      link("get?id=7", "get", Group.Pages, { text: "Manual" }),
      {
        url: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
        name: "The launch video",
        group: Group.VideoPages,
        text: "The launch video",
      },
      link("about.html", "about.html", Group.Pages, { text: "About us" }),
      link("logo.png", "logo.png", Group.Images),
    ],
  };
}

// sampleSubscriptions are a channel that brought two videos, and a podcast whose site fails.
export function sampleSubscriptions(now = Date.now()): Subscription[] {
  const base = {
    site: "",
    podcast: false,
    quality: "best",
    qualities: [
      { id: "best", label: "Best" },
      { id: "720", label: "720p" },
      { id: "mp3", label: "Audio MP3", audioOnly: true },
    ],
    directory: "/home/me/Downloads/Lanterns",
    queue: "main",
    naming: "",
    everyHours: 6,
    paused: false,
    filters: {},
    error: "",
    added: 0,
    last: 0,
    checking: false,
    seen: 40,
    created: now - 86_400_000,
  };
  return [
    {
      ...base,
      id: "s1",
      title: "Lanterns Studio",
      source: "https://www.youtube.com/@lanterns/videos",
      site: "YoutubeTab",
      checked: now - 2 * 3_600_000,
      next: now + 4 * 3_600_000,
      added: 5,
      last: 2,
    },
    {
      ...base,
      id: "s2",
      title: "The Harbour Hour",
      source: "https://feeds.example.org/harbour.rss",
      podcast: true,
      quality: "published",
      qualities: [
        { id: "published", label: "As published", audioOnly: true },
        { id: "mp3", label: "Audio MP3", audioOnly: true },
      ],
      directory: "/home/me/Podcasts/Harbour",
      everyHours: 24,
      checked: now - 3_600_000,
      next: now + 3_600_000,
      error: "the feed's site answered 500 Internal Server Error",
    },
  ];
}

// subscriptionDownloads are what the sample channel brought: one video finished, one coming.
export function subscriptionDownloads(now = Date.now()): Download[] {
  return [
    sampleDownload({
      id: "lanterns-2",
      url: "https://www.youtube.com/watch?v=lan2",
      kind: "stream",
      status: "downloading",
      name: "Paper lanterns at dusk",
      size: 310 * MB,
      done: 120 * MB,
      speed: 6 * MB,
      directory: "/home/me/Downloads/Lanterns",
      subscription: "s1",
      created: now - 2 * 3_600_000,
    }),
    sampleDownload({
      id: "lanterns-1",
      url: "https://www.youtube.com/watch?v=lan1",
      kind: "stream",
      status: "done",
      name: "Folding a lantern in ten minutes.mp4",
      path: "/home/me/Downloads/Lanterns/Folding a lantern in ten minutes.mp4",
      size: 180 * MB,
      done: 180 * MB,
      directory: "/home/me/Downloads/Lanterns",
      subscription: "s1",
      created: now - 2 * 3_600_000,
      finished: now - 3_500_000,
    }),
  ];
}

// settled are the samples with nothing waiting for the Save dialog, which would open at once.
export function settled(): Download[] {
  return samples().filter((d) => d.status !== "pending");
}

function sampleSettings(): Settings {
  return {
    downloadDir: "/home/me/Downloads",
    connections: 8,
    maxActive: 3,
    notify: true,
    ask: true,
    naming: { ...defaultNaming },
    recentDirs: ["/home/me/Videos/Trips"],
    queues: [],
  };
}

type Method = Exclude<keyof Api, "follow">;

export class FakeApi implements Api {
  downloads: Download[];
  settingsValue = sampleSettings();
  clipboard = "";
  setupValue = {
    extensionDir: "/home/me/.local/share/snag/extension",
    connected: false,
    connectedBefore: true,
  };
  registered = ["Vivaldi", "Firefox"]; // what setting the browsers up again reports
  folderPick = "";
  clashes = new Map<string, Clash>(); // by "folder/name": what saving there would meet
  browsersValue: Browser[] = [
    { id: "vivaldi:/usr/bin/vivaldi-stable", name: "Vivaldi" },
    { id: "brave:/usr/bin/brave", name: "Brave" },
  ];
  playersValue: Player[] = [
    { id: "org.kde.haruna.desktop", name: "Haruna", default: true },
    { id: "mpv.desktop", name: "mpv Media Player", default: false },
  ];
  queuesValue: QueueState[] = sampleQueues().slice(0, 1); // just Main, unless a test says
  countdownValue: Countdown | null = null;
  playlistValue = samplePlaylist();
  playlistHaveValue: Record<string, boolean[]> = {}; // by quality: what Snag has of the list
  playlistNote = ""; // what downloading the list says went otherwise
  playlistLinks = new Set<string>(); // links add opens the playlist window for
  linksValue = sampleLinks();
  linksHaveValue: boolean[] = [];
  // What asking the servers finds, by url: the rest say nothing new.
  lookValue: Record<string, Partial<Link>> = {
    "https://example.org/pub/tool-1.0.zip": { size: 12 * MB },
    "https://example.org/pub/get?id=7": {
      server: "Manual.pdf",
      size: 2 * MB,
      group: Group.Documents,
    },
  };
  importValue = ""; // the set importLinks opens, "" for the user picking no file
  startAtLoginValue = { on: true, available: true, desktop: "", command: "" };
  subscriptionsValue: Subscription[] = [];
  calls: string[] = []; // "pause d1", "add https://…", in order
  failing = new Map<Method, string>(); // a method's calls fail with this message
  holding = new Set<Method>(); // a method's calls wait until release()
  private releases: (() => void)[] = [];
  private handlers: Handlers | null = null;

  constructor(downloads = samples()) {
    this.downloads = downloads;
  }

  release(): void {
    const waiting = this.releases;
    this.releases = [];
    waiting.forEach((go) => go());
  }

  emitReset(list: Download[]): void {
    this.downloads = list;
    this.handlers?.reset(list);
  }

  emit(d: Download): void {
    const i = this.downloads.findIndex((x) => x.id === d.id);
    if (i >= 0) this.downloads[i] = d;
    else this.downloads.unshift(d);
    this.handlers?.changed(d);
  }

  emitQueues(): void {
    this.handlers?.queues(structuredClone(this.queuesValue));
  }

  startCountdown(c: Countdown): void {
    this.countdownValue = c;
    this.handlers?.countdown(c);
  }

  emitRemoved(id: string): void {
    this.downloads = this.downloads.filter((d) => d.id !== id);
    this.handlers?.removed(id);
  }

  private async call(method: Method, ...args: unknown[]): Promise<void> {
    this.calls.push(
      [method, ...args.map((a) => (typeof a === "object" ? JSON.stringify(a) : String(a)))].join(
        " ",
      ),
    );
    if (this.holding.has(method)) await new Promise<void>((go) => this.releases.push(go));
    const failure = this.failing.get(method);
    if (failure) throw new Error(failure);
  }

  private setStatus(id: string, status: string): void {
    const d = this.downloads.find((x) => x.id === id);
    if (d) this.emit({ ...d, status });
  }

  async list() {
    await this.call("list");
    return structuredClone(this.downloads);
  }

  async get(id: string) {
    await this.call("get", id);
    const d = this.downloads.find((x) => x.id === id);
    if (!d) throw new Error("no such download");
    return structuredClone(d);
  }

  async choose(id: string) {
    await this.call("choose", id);
  }

  // What looking somewhere else finds, until a test says otherwise.
  searchValue: Found[] = [];

  async searchElsewhere(id: string, place: string) {
    await this.call("searchElsewhere", id, place);
    return structuredClone(this.searchValue);
  }

  async useFound(id: string, address: string, place: string, remember: boolean) {
    await this.call("useFound", id, address, place, remember);
    const d = this.downloads.find((x) => x.id === id);
    if (!d) return;
    d.url = address;
    d.where = false;
    d.status = "queued";
    d.error = "";
    this.handlers?.changed(structuredClone(d));
    if (remember && place) {
      const q = this.queuesValue.find((x) => x.id === (d.queue || "main"));
      if (q) q.fallback = place;
    }
  }

  async queues() {
    return structuredClone(this.queuesValue);
  }

  async saveQueue(q: Queue) {
    await this.call("saveQueue", q);
    const saved: Queue = { ...q, id: q.id || `q${this.queuesValue.length}` };
    if (this.queuesValue.some((x) => x.id === saved.id))
      this.queuesValue = this.queuesValue.map((x) => (x.id === saved.id ? { ...x, ...saved } : x));
    else this.queuesValue.push({ ...saved, running: false });
    this.emitQueues();
    return structuredClone(saved);
  }

  async deleteQueue(id: string, downloads: boolean) {
    await this.call("deleteQueue", id, downloads);
    for (const d of this.downloads.filter((x) => x.queue === id)) {
      if (downloads && d.status !== "done") this.emitRemoved(d.id);
      else this.emit({ ...d, queue: "main" });
    }
    this.queuesValue = this.queuesValue.filter((q) => q.id !== id);
    this.emitQueues();
  }

  private setRunning(id: string, running: boolean) {
    this.queuesValue = this.queuesValue.map((q) => (q.id === id ? { ...q, running } : q));
    this.emitQueues();
  }

  async startQueue(id: string) {
    await this.call("startQueue", id);
    this.setRunning(id, true);
  }

  async stopQueue(id: string) {
    await this.call("stopQueue", id);
    this.setRunning(id, false);
  }

  async moveToQueue(ids: string[], queue: string) {
    await this.call("moveToQueue", ids, queue);
    let last = Math.max(0, ...this.downloads.map((d) => d.position));
    for (const id of ids) {
      const d = this.downloads.find((x) => x.id === id);
      if (d) this.emit({ ...d, queue, position: ++last });
    }
  }

  async reverseQueue(id: string) {
    await this.call("reverseQueue", id);
    const line = this.downloads
      .filter((d) => d.queue === id && !["done", "pending", "removing"].includes(d.status))
      .sort((a, b) => (a.position ?? 0) - (b.position ?? 0));
    const places = line.map((d) => d.position ?? 0);
    const next = this.downloads.map((d) => {
      const at = line.findIndex((x) => x.id === d.id);
      return at < 0 ? d : { ...d, position: places[places.length - 1 - at] ?? 0 };
    });
    this.emitReset(next);
  }

  async reorder(queue: string, ids: string[]) {
    await this.call("reorder", queue, ids);
    const places = ids
      .map((id) => this.downloads.find((d) => d.id === id)?.position ?? 0)
      .sort((a, b) => a - b);
    ids.forEach((id, i) => {
      const d = this.downloads.find((x) => x.id === id);
      const place = places[i] ?? 0;
      if (d && d.position !== place) this.emit({ ...d, position: place });
    });
  }

  async arrangeQueues(ids: string[]) {
    await this.call("arrangeQueues", ids);
    const at = (id: string) =>
      id === "main" ? -1 : ids.indexOf(id) < 0 ? ids.length : ids.indexOf(id);
    this.queuesValue = [...this.queuesValue].sort((a, b) => at(a.id) - at(b.id));
    this.emitQueues();
  }

  async toFront(id: string) {
    await this.call("toFront", id);
    const d = this.downloads.find((x) => x.id === id);
    if (d) this.emit({ ...d, position: Math.min(...this.downloads.map((x) => x.position)) - 1 });
  }

  async startNow(id: string) {
    await this.call("startNow", id);
    this.setStatus(id, "downloading");
  }

  async countdown() {
    return this.countdownValue;
  }

  async cancelCountdown() {
    await this.call("cancelCountdown");
    this.countdownValue = null;
    this.handlers?.countdown(null);
  }

  async actNow() {
    await this.call("actNow");
    this.countdownValue = null;
    this.handlers?.countdown(null);
  }

  async playlist(id: string) {
    await this.call("playlist", id);
    if (id !== this.playlistValue.id) throw new Error("that playlist is gone");
    return structuredClone(this.playlistValue);
  }

  async playlistHave(_id: string, quality: string) {
    return this.playlistHaveValue[quality] ?? this.playlistValue.entries.map(() => false);
  }

  async downloadPlaylist(id: string, pick: PlaylistPick): Promise<PlaylistAdded> {
    await this.call("downloadPlaylist", id, pick);
    const queue = this.queuesValue.find((q) => q.id === pick.queue) ?? {
      id: `q${this.queuesValue.length}`,
      name: this.playlistValue.title,
      atOnce: 2,
      retries: 3,
      retryWait: 5,
      schedule: { on: false, start: "", stop: "", days: [] },
    };
    return {
      queue,
      added: pick.entries.length,
      already: 0,
      ...(this.playlistNote && { note: this.playlistNote }),
    };
  }

  async links(id: string) {
    await this.call("links", id);
    if (id !== this.linksValue.id) throw new Error("those links are gone");
    return structuredClone(this.linksValue);
  }

  async linksHave() {
    return this.linksHaveValue.length
      ? [...this.linksHaveValue]
      : this.linksValue.links.map(() => false);
  }

  async lookLinks(id: string, indexes: number[]) {
    await this.call("lookLinks", id, indexes);
    return indexes.map((i) => {
      const l = this.linksValue.links[i]!;
      const found = l.group === Group.VideoPages ? {} : (this.lookValue[l.url] ?? {});
      const looked = { ...l, ...found, looked: l.group !== Group.VideoPages };
      this.linksValue.links[i] = looked;
      return structuredClone(looked);
    });
  }

  async downloadLinks(id: string, pick: LinksPick): Promise<LinksAdded> {
    await this.call("downloadLinks", id, pick);
    return { added: pick.entries.length, already: 0 };
  }

  async addLinks(text: string) {
    await this.call("addLinks", text);
    return this.linksValue.id;
  }

  async importLinks() {
    await this.call("importLinks");
    return this.importValue;
  }

  emitSubscriptions(): void {
    this.handlers?.subscriptions(structuredClone(this.subscriptionsValue));
  }

  async subscriptions() {
    return structuredClone(this.subscriptionsValue);
  }

  async changeSubscription(id: string, change: SubscriptionChange) {
    await this.call("changeSubscription", id, change);
    const i = this.subscriptionsValue.findIndex((s) => s.id === id);
    if (i < 0) throw new Error("no such subscription");
    const changed = { ...this.subscriptionsValue[i]!, ...change };
    this.subscriptionsValue[i] = changed;
    this.emitSubscriptions();
    return structuredClone(changed);
  }

  async checkSubscription(id: string) {
    await this.call("checkSubscription", id);
    this.subscriptionsValue = this.subscriptionsValue.map((s) =>
      s.id === id ? { ...s, checking: true, paused: false } : s,
    );
    this.emitSubscriptions();
  }

  async openSubscriptionPage(id: string) {
    await this.call("openSubscriptionPage", id);
  }

  async findSubscription(link: string) {
    await this.call("findSubscription", link);
  }

  async unsubscribe(id: string) {
    await this.call("unsubscribe", id);
    this.subscriptionsValue = this.subscriptionsValue.filter((s) => s.id !== id);
    this.emitSubscriptions();
  }

  async closeWindow() {
    await this.call("closeWindow");
  }

  fitted: number[] = [];
  async fitWindow(height: number) {
    this.fitted.push(height);
  }

  async add(link: string): Promise<Added> {
    await this.call("add", link);
    if (this.playlistLinks.has(link))
      return { download: sampleDownload({}), duplicate: false, playlist: this.playlistValue.id };
    const existing = this.downloads.find((d) => d.url === link);
    if (existing) return { download: existing, duplicate: true };
    const d = sampleDownload({
      id: `new${this.downloads.length}`,
      url: link,
      name: link.split("/").pop() ?? link,
    });
    this.emit(d);
    return { download: d, duplicate: false };
  }

  async pause(id: string) {
    await this.call("pause", id);
    this.setStatus(id, "paused");
  }

  async resume(id: string) {
    await this.call("resume", id);
    this.setStatus(id, "queued");
  }

  async remove(id: string, deleteFile: boolean) {
    await this.call("remove", id, deleteFile);
    this.emitRemoved(id);
  }

  async confirm(id: string, answer: SaveAnswer) {
    await this.call("confirm", id, answer);
    this.setStatus(id, answer.start ? "queued" : "paused");
  }

  async clash(_id: string, directory: string, filename: string) {
    return this.clashes.get(`${directory}/${filename}`) ?? null;
  }

  async open(id: string) {
    await this.call("open", id);
  }

  async watch(id: string, player?: string) {
    await this.call("watch", id, ...(player ? [player] : []));
  }

  async players() {
    return structuredClone(this.playersValue);
  }

  async copyStreamAddress(id: string) {
    await this.call("copyStreamAddress", id);
  }

  async reveal(id: string) {
    await this.call("reveal", id);
  }

  async copyLink(id: string) {
    await this.call("copyLink", id);
  }

  activityValue: Record<string, { at: number; text: string }[]> = {};

  async activity(id: string) {
    return structuredClone(this.activityValue[id] ?? []);
  }

  // What checking a file finds, by download; a match unless a test says otherwise.
  checkResults: Record<string, "match" | "mismatch" | "failed"> = {};
  fileChecksum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855";

  async check(id: string, text: string) {
    await this.call("check", id, text);
    const d = this.downloads.find((x) => x.id === id);
    if (!d) throw new Error("no such download");
    const given = text.trim();
    const check = given
      ? {
          algo: "SHA-256",
          weak: false,
          from: "you",
          want: given,
          got: "",
          error: "",
          read: 0,
          result: "",
        }
      : d.check;
    if (!check?.want) throw new Error("there's no checksum to check it against: paste one");
    this.emit({ ...d, check: { ...check, result: "checking", read: 0 } });
    const result = this.checkResults[id] ?? "match";
    this.emit({
      ...d,
      check: {
        ...check,
        result,
        got: result === "failed" ? "" : result === "match" ? check.want : this.fileChecksum,
        error: result === "failed" ? "permission denied" : "",
      },
    });
  }

  async redownload(id: string) {
    await this.call("redownload", id);
    const d = this.downloads.find((x) => x.id === id);
    if (!d) throw new Error("no such download");
    this.emitRemoved(id);
    const fresh: Download = {
      ...d,
      id: `${id}-again`,
      status: "queued",
      done: 0,
      finished: 0,
      check: null,
    };
    this.emit(fresh);
    return structuredClone(fresh);
  }

  async copyChecksum(id: string) {
    await this.call("copyChecksum", id);
    this.clipboard = this.fileChecksum;
    return this.fileChecksum;
  }

  async pauseAll() {
    await this.call("pauseAll");
  }

  async resumeAll() {
    await this.call("resumeAll");
  }

  async clearDone() {
    await this.call("clearDone");
    for (const d of this.downloads.filter((x) => x.status === "done")) this.emitRemoved(d.id);
  }

  async settings() {
    await this.call("settings");
    return { ...this.settingsValue };
  }

  async setSettings(settings: Settings) {
    await this.call("setSettings", settings);
    this.settingsValue = { ...settings };
    return { ...settings };
  }

  async startAtLogin() {
    return { ...this.startAtLoginValue };
  }

  async setStartAtLogin(on: boolean) {
    await this.call("setStartAtLogin", on);
    this.startAtLoginValue = { ...this.startAtLoginValue, on };
    return { ...this.startAtLoginValue };
  }

  async chooseFolder(start: string) {
    await this.call("chooseFolder", start);
    return this.folderPick;
  }

  async places() {
    return [
      this.settingsValue.downloadDir,
      "/home/me/Videos",
      ...(this.settingsValue.recentDirs ?? []),
    ];
  }

  async home() {
    return "/home/me";
  }

  async clipboardText() {
    return this.clipboard;
  }

  async copyText(text: string) {
    await this.call("copyText", text);
  }

  async setup() {
    return { ...this.setupValue };
  }

  async showExtensionFolder() {
    await this.call("showExtensionFolder");
  }

  async browsers() {
    return structuredClone(this.browsersValue);
  }

  async openExtensionsPage(id: string) {
    await this.call("openExtensionsPage", id);
  }

  async registerBrowsers() {
    await this.call("registerBrowsers");
    return this.registered;
  }

  connect(): void {
    this.setupValue = { ...this.setupValue, connected: true, connectedBefore: true };
    this.handlers?.connected();
  }

  async version() {
    return "test";
  }

  async author() {
    return { name: "Roham1299", page: "https://github.com/Roham1299" };
  }

  async openAuthorPage() {
    await this.call("openAuthorPage");
  }

  updateValue: { version: string } | null = null;

  async update() {
    return this.updateValue;
  }

  async openUpdatePage() {
    await this.call("openUpdatePage");
  }

  phoneValue: Phone = { on: false, address: "", network: "", devices: [] };
  phoneNetwork = "192.168.1.0/24"; // the network turning it on finds; "" for none
  private phoneHandler: ((p: Phone) => void) | null = null;

  async phone() {
    return structuredClone(this.phoneValue);
  }

  async setPhone(on: boolean) {
    await this.call("setPhone", on);
    if (on && !this.phoneNetwork)
      throw new Error("this computer isn't on a home network Snag can listen on");
    this.phoneValue = on
      ? { ...this.phoneValue, on, network: this.phoneNetwork, address: "http://192.168.1.5:41234" }
      : { ...this.phoneValue, on, network: "", address: "" };
    return structuredClone(this.phoneValue);
  }

  async pairPhone() {
    await this.call("pairPhone");
    return {
      link: "http://192.168.1.5:41234/#pair=code1",
      path: "M0 0h7v1h-7z",
      size: 21,
      until: Date.now() + 10 * 60_000,
    };
  }

  async stopPairing() {
    await this.call("stopPairing");
  }

  async removePhone(id: string) {
    await this.call("removePhone", id);
    this.phoneValue.devices = this.phoneValue.devices.filter((d) => d.id !== id);
    return structuredClone(this.phoneValue);
  }

  onPhone(handler: (p: Phone) => void) {
    this.phoneHandler = handler;
    return () => (this.phoneHandler = null);
  }

  pairedPhone(name: string): void {
    const now = Date.now();
    this.phoneValue.devices.push({
      id: `p${this.phoneValue.devices.length + 1}`,
      name,
      added: now,
      seen: now,
    });
    this.phoneHandler?.(structuredClone(this.phoneValue));
  }

  follow(handlers: Handlers) {
    this.handlers = handlers;
    return () => (this.handlers = null);
  }
}
