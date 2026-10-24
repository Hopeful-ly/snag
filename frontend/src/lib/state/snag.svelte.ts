import type {
  Added,
  Api,
  Countdown,
  Download,
  Player,
  Queue,
  QueueState,
  Settings,
  Setup,
  StartAtLogin,
  Subscription,
  SubscriptionChange,
} from "../api/client";
import {
  queueOfView,
  queueView,
  subscriptionOfView,
  subscriptionView,
  titleOf,
  type FilterId,
} from "../filters";
import { arranged, mainQueue } from "../queues";
import type { Action, QueueAction } from "./actions";
import { Downloads } from "./downloads.svelte";

type ToastKind = "info" | "good" | "bad";

const toastTime: Record<ToastKind, number> = { info: 2600, good: 2600, bad: 5000 };
const linkPattern = /^https?:\/\/\S+$/;

// Every action marks itself busy until the Go side answers; failures become a toast.
export class Snag {
  readonly downloads = new Downloads();
  settings = $state<Settings | null>(null);
  home = $state("");
  ready = $state(false);

  filter = $state<FilterId>("all");
  search = $state("");
  selected = $state("");
  busy = $state<Record<string, boolean>>({});

  addOpen = $state(false);
  settingsOpen = $state(false);
  subscriptionsOpen = $state(false);
  subscribeFocus = $state(false); // the subscriptions open at the box for following a new one
  connectOpen = $state(false);
  setup = $state<Setup | null>(null);
  players = $state<Player[]>([]);
  // Whether Snag starts at login (the system's to say, so asked, not a setting); null until known.
  startAtLogin = $state<StartAtLogin | null>(null);
  menu = $state<{ id: string; x: number; y: number } | null>(null);
  queueMenu = $state<{ id: string; x: number; y: number } | null>(null);

  queues = $state<QueueState[]>([]); // Main first
  countdown = $state<Countdown | null>(null); // a finished queue's action, waiting its minute
  // queue is null when creating; the whole thing is null while the sheet is closed.
  queueSheet = $state<{ queue: QueueState | null; deleting?: boolean } | null>(null);
  whereFor = $state<string | null>(null);
  checkFor = $state<string | null>(null);
  detailsFor = $state<string | null>(null);
  subscriptions = $state<Subscription[]>([]);
  subscriptionSheet = $state<string | null>(null);
  subscriptionMenu = $state<{ id: string; x: number; y: number } | null>(null);
  queue = $derived(this.queues.find((q) => q.id === queueOfView(this.filter)) ?? null);
  subscription = $derived(
    this.subscriptions.find((s) => s.id === subscriptionOfView(this.filter)) ?? null,
  );
  title = $derived(
    this.queue
      ? this.queue.name
      : this.subscription
        ? this.subscription.title
        : titleOf(this.filter),
  );
  toast = $state<{ text: string; kind: ToastKind; key: number } | null>(null);

  visible = $derived(this.downloads.visible(this.filter, this.search));

  private toastTimer: ReturnType<typeof setTimeout> | undefined;
  private stopFollowing: (() => void) | undefined;

  constructor(readonly api: Api) {}

  async start(): Promise<void> {
    this.stopFollowing = this.api.follow({
      changed: (d) => this.changed(d),
      removed: (id) => this.removed(id),
      reset: (list) => {
        this.downloads.list = list;
        if (this.selected && !list.some((d) => d.id === this.selected)) this.selected = "";
      },
      settings: (s) => (this.settings = s),
      connected: () => {
        if (this.setup) this.setup = { ...this.setup, connected: true, connectedBefore: true };
      },
      queues: (list) => {
        this.queues = list;
        // A queue that was deleted shows no more: back to everything.
        if (queueOfView(this.filter) && !this.queue) this.filter = "all";
      },
      countdown: (c) => (this.countdown = c),
      subscriptions: (list) => {
        this.subscriptions = list;
        if (this.subscriptionSheet && !list.some((s) => s.id === this.subscriptionSheet))
          this.subscriptionSheet = null;
        // An unsubscribed list's downloads stay, under All.
        if (subscriptionOfView(this.filter) && !this.subscription) this.filter = "all";
      },
    });
    for (;;) {
      try {
        const [list, settings, home, setup, queues, countdown] = await Promise.all([
          this.api.list(),
          this.api.settings(),
          this.api.home(),
          this.api.setup(),
          this.api.queues(),
          this.api.countdown(),
        ]);
        this.queues = queues;
        this.countdown = countdown;
        this.downloads.list = list;
        this.settings = settings;
        this.home = home;
        this.setup = setup;
        this.ready = true;
        this.api.players().then(
          (players) => (this.players = players),
          () => {},
        );
        void this.refreshStartAtLogin();
        this.api.subscriptions().then(
          (list) => (this.subscriptions = list),
          () => {},
        );
        // Until the browser extension has talked to Snag once, show how to connect it.
        if (!setup.connectedBefore) this.connectOpen = true;
        return;
      } catch {
        await new Promise((resolve) => setTimeout(resolve, 500)); // the Go side is still starting
      }
    }
  }

  stop(): void {
    this.stopFollowing?.();
  }

  private changed(d: Download): void {
    const before = this.downloads.get(d.id);
    this.downloads.upsert(d);
    // A song YouTube Music just failed to find asks where else to look, unless one already is.
    if (d.where && !before?.where && this.whereFor === null) this.whereFor = d.id;
  }

  private removed(id: string): void {
    this.downloads.remove(id);
    if (this.selected === id) this.selected = "";
    if (this.checkFor === id) this.checkFor = null;
    if (this.detailsFor === id) this.detailsFor = null;
  }

  busyActionOf(id: string): string {
    for (const key in this.busy) if (key.startsWith(id + ":")) return key.slice(id.length + 1);
    return "";
  }

  async request(key: string, call: () => Promise<unknown>): Promise<boolean> {
    if (key) this.busy[key] = true;
    try {
      await call();
      return true;
    } catch (error) {
      this.showToast(messageOf(error), "bad");
      return false;
    } finally {
      if (key) delete this.busy[key];
    }
  }

  // arg is the player for watchWith, the queue for move.
  async act(id: string, action: Action, arg?: string): Promise<void> {
    const player = arg;
    const d = this.downloads.get(id);
    if (!d) return;
    this.selected = id;
    const api = this.api;
    switch (action) {
      case "choose":
        await this.request(`${id}:choose`, () => api.choose(id));
        return;
      case "where":
        this.whereFor = id;
        return;
      case "checkSheet":
        this.checkFor = id;
        return;
      case "details":
        this.detailsFor = id;
        return;
      case "check":
        await this.request(`${id}:check`, () => api.check(id, ""));
        return;
      case "redownload":
        if (await this.request(`${id}:redownload`, () => api.redownload(id)))
          this.showToast(`Downloading ${d.name} again`, "good");
        return;
      case "copy":
        if (await this.request("", () => api.copyLink(id))) this.showToast("Link copied", "good");
        return;
      case "watchWith":
        await this.request(`${id}:watch`, () => api.watch(id, player));
        return;
      case "copyAddress":
        if (await this.request("", () => api.copyStreamAddress(id)))
          this.showToast("Stream address copied: paste it into any player", "good");
        return;
      case "move":
        await this.moveTo([id], arg ?? mainQueue);
        return;
      case "newQueue":
        this.queueSheet = { queue: null };
        this.movingToNew = [id];
        return;
      case "startNow":
        await this.request(`${id}:startNow`, () => api.startNow(id));
        return;
      case "toFront":
        await this.request(`${id}:toFront`, () => api.toFront(id));
        return;
      case "remove":
      case "delete":
        await this.request(`${id}:remove`, async () => {
          await api.remove(id, action === "delete");
          if (action === "delete") this.showToast(`Deleted ${d.name}`, "good");
        });
        return;
      default:
        await this.request(`${id}:${action}`, () => api[action](id));
    }
  }

  async actOnSubscription(id: string, action: SubscriptionAction): Promise<void> {
    const s = this.subscriptions.find((x) => x.id === id);
    if (!s) return;
    switch (action) {
      case "check":
        if (await this.request(`subscription:${id}:check`, () => this.api.checkSubscription(id)))
          this.showToast(`Looking for what's new in ${s.title}`, "info");
        return;
      case "pause":
      case "resume":
        await this.request(`subscription:${id}:save`, () =>
          this.api.changeSubscription(id, { ...changeOf(s), paused: action === "pause" }),
        );
        return;
      case "edit":
        this.subscriptionSheet = id;
        return;
      case "downloads":
        this.show(subscriptionView(id));
        return;
      case "page":
        await this.request(`subscription:${id}:page`, () => this.api.openSubscriptionPage(id));
        return;
      case "remove":
        if (await this.request(`subscription:${id}:remove`, () => this.api.unsubscribe(id)))
          this.showToast(`Unsubscribed from ${s.title}; what it downloaded stays`, "good");
        return;
    }
  }

  openSubscriptionMenu(id: string, x: number, y: number): void {
    this.subscriptionMenu = { id, x, y };
  }

  async checkAllSubscriptions(): Promise<void> {
    const due = this.subscriptions.filter((s) => !s.paused && !s.checking);
    if (!due.length) return;
    const ok = await this.request("global:subscriptions:check", () =>
      Promise.all(due.map((s) => this.api.checkSubscription(s.id))),
    );
    if (ok)
      this.showToast(
        due.length === 1
          ? `Looking for what's new in ${due[0]!.title}`
          : `Looking for what's new in ${due.length} subscriptions`,
        "info",
      );
  }

  async findSubscription(link: string): Promise<boolean> {
    return this.request("global:subscriptions:find", () => this.api.findSubscription(link.trim()));
  }

  show(filter: FilterId): void {
    this.filter = filter;
    this.settingsOpen = false;
    this.subscriptionsOpen = false;
  }

  // subscribe focuses the box for following a new one.
  openSubscriptions(subscribe = false): void {
    this.settingsOpen = false;
    this.subscriptionsOpen = true;
    this.subscribeFocus = subscribe;
  }

  // Downloads to move into the queue being made, once it is saved.
  movingToNew: string[] = [];

  queueNamed(id: string): QueueState | undefined {
    return this.queues.find((q) => q.id === (id || mainQueue));
  }

  async startQueue(id: string): Promise<void> {
    await this.request(`queue:${id}:run`, () => this.api.startQueue(id));
  }

  async stopQueue(id: string): Promise<void> {
    await this.request(`queue:${id}:run`, () => this.api.stopQueue(id));
  }

  async reverseQueue(id: string): Promise<void> {
    await this.request(`queue:${id}:reverse`, () => this.api.reverseQueue(id));
  }

  async actOnQueue(id: string, action: QueueAction): Promise<void> {
    const queue = this.queueNamed(id);
    if (!queue) return;
    switch (action) {
      case "start":
        return this.startQueue(id);
      case "stop":
        return this.stopQueue(id);
      case "reverse":
        return this.reverseQueue(id);
      case "edit":
        this.queueSheet = { queue };
        return;
      case "delete":
        this.queueSheet = { queue, deleting: true };
        return;
      case "up":
      case "down": {
        const ids = this.queues.map((q) => q.id);
        const i = ids.indexOf(id);
        const neighbour = ids[action === "up" ? i - 1 : i + 1];
        if (neighbour && neighbour !== mainQueue)
          await this.arrangeQueues(arranged(ids, id, neighbour, action === "down"));
        return;
      }
    }
  }

  async arrangeQueues(ids: string[]): Promise<void> {
    const before = this.queues;
    this.queues = ids.flatMap((id) => before.find((q) => q.id === id) ?? []);
    if (!(await this.request("queue:arrange", () => this.api.arrangeQueues(ids))))
      this.queues = before;
  }

  openQueueMenu(id: string, x: number, y: number): void {
    this.queueMenu = { id, x, y };
  }

  async saveQueue(q: Queue): Promise<boolean> {
    let saved: Queue | undefined;
    const ok = await this.request("queue:save", async () => (saved = await this.api.saveQueue(q)));
    if (!ok || !saved) return false;
    this.queues = await this.api.queues().catch(() => this.queues);
    const moving = this.movingToNew;
    this.movingToNew = [];
    if (moving.length) await this.moveTo(moving, saved.id);
    if (!q.id) {
      this.filter = queueView(saved.id); // a new queue opens
      this.showToast(`Made the queue ${saved.name}`, "good");
    }
    return true;
  }

  // downloads: delete its unfinished downloads too, rather than moving them to Main.
  async deleteQueue(id: string, downloads: boolean): Promise<boolean> {
    const name = this.queueNamed(id)?.name ?? "the queue";
    const ok = await this.request("queue:delete", () => this.api.deleteQueue(id, downloads));
    if (ok) {
      if (this.filter === queueView(id)) this.filter = "all";
      this.queues = this.queues.filter((q) => q.id !== id);
      this.showToast(
        downloads
          ? `Deleted ${name} and its downloads`
          : `Deleted ${name}; its downloads are in Main`,
        "good",
      );
    }
    return ok;
  }

  unfinishedIn(queue: string): number {
    return this.downloads.list.filter((d) => d.queue === queue && d.status !== "done").length;
  }

  async moveTo(ids: string[], queue: string): Promise<void> {
    const name = this.queueNamed(queue)?.name ?? "the queue";
    if (await this.request("", () => this.api.moveToQueue(ids, queue)))
      this.showToast(
        ids.length === 1 ? `Moved to ${name}` : `Moved ${ids.length} downloads to ${name}`,
        "good",
      );
  }

  async reorder(queue: string, ids: string[]): Promise<void> {
    await this.request("", () => this.api.reorder(queue, ids));
  }

  async cancelCountdown(): Promise<void> {
    await this.request("countdown:cancel", () => this.api.cancelCountdown());
    this.countdown = null;
  }

  async actNow(): Promise<void> {
    await this.request("countdown:now", () => this.api.actNow());
    this.countdown = null;
  }

  async pauseAll(): Promise<void> {
    await this.request("global:pauseAll", () => this.api.pauseAll());
  }

  async resumeAll(): Promise<void> {
    await this.request("global:resumeAll", () => this.api.resumeAll());
  }

  async clearDone(): Promise<void> {
    await this.request("global:clearDone", async () => {
      await this.api.clearDone();
      this.showToast("Cleared finished downloads", "good");
    });
  }

  async addLink(link: string): Promise<boolean> {
    link = link.trim();
    if (linksIn(link) > 1) {
      const ok = await this.request("global:add", () => this.api.addLinks(link));
      if (ok) this.addOpen = false;
      return ok;
    }
    if (!linkPattern.test(link)) {
      this.showToast(
        "That doesn't look like a link (it should start with http:// or https://)",
        "bad",
      );
      return false;
    }
    let added: Added | undefined;
    await this.request("global:add", async () => (added = await this.api.add(link)));
    if (!added) return false;
    this.addOpen = false;
    // A playlist's link opens its own window for choosing from it; nothing was added yet.
    if (added.playlist) return true;
    if (added.duplicate) this.showToast("Already in your list", "info");
    else if (added.download.status !== "pending")
      this.showToast(`Downloading ${added.download.name}`, "good");
    return true;
  }

  // "" unless the clipboard holds a link, or several.
  async pasteLink(): Promise<string> {
    const text = (await this.api.clipboardText().catch(() => "")).trim();
    if (linksIn(text) > 1) return text.replace(/\s+/g, " ");
    return linkPattern.test(text) ? text : "";
  }

  async importLinks(): Promise<void> {
    let id = "";
    await this.request("global:import", async () => (id = await this.api.importLinks()));
    if (id) this.addOpen = false;
  }

  // The system's startup settings may have changed it.
  async refreshStartAtLogin(): Promise<void> {
    this.startAtLogin = await this.api.startAtLogin().catch(() => this.startAtLogin);
  }

  async setStartAtLogin(on: boolean): Promise<void> {
    await this.request("global:settings:startAtLogin", async () => {
      this.startAtLogin = await this.api.setStartAtLogin(on);
    });
  }

  async setSetting<K extends keyof Settings>(key: K, value: Settings[K]): Promise<void> {
    if (!this.settings) return;
    const changed = { ...this.settings, [key]: value };
    await this.request(`global:settings:${String(key)}`, async () => {
      this.settings = await this.api.setSettings(changed);
    });
  }

  showToast(text: string, kind: ToastKind = "info"): void {
    clearTimeout(this.toastTimer);
    this.toast = { text, kind, key: Date.now() };
    this.toastTimer = setTimeout(() => (this.toast = null), toastTime[kind]);
  }

  openMenu(id: string, x: number, y: number): void {
    this.selected = id;
    this.menu = { id, x, y };
  }

  short(path: string): string {
    return this.home && path.startsWith(this.home) ? "~" + path.slice(this.home.length) : path;
  }
}

export type SubscriptionAction =
  "check" | "pause" | "resume" | "edit" | "downloads" | "page" | "remove";

export function changeOf(s: Subscription): SubscriptionChange {
  return {
    title: s.title,
    quality: s.quality,
    directory: s.directory,
    queue: s.queue,
    naming: s.naming,
    everyHours: s.everyHours,
    paused: s.paused,
    filters: { ...s.filters },
  };
}

function linksIn(text: string): number {
  return text.match(/https?:\/\/\S+/g)?.length ?? 0;
}

function messageOf(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "That didn't work";
}
