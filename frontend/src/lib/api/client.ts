// The window takes an Api so tests can pass a fake.
import { Events, Window } from "@wailsio/runtime";
import {
  DownloadsService,
  LinksService,
  PhoneService,
  PlaylistsService,
  QueuesService,
  SettingsService,
  SetupService,
  SubscriptionsService,
  SystemService,
} from "../../../bindings/github.com/Roham1299/snag/internal/app";
import type * as app from "../../../bindings/github.com/Roham1299/snag/internal/app/models";
import type * as config from "../../../bindings/github.com/Roham1299/snag/internal/config/models";

export type Download = app.Download;
export type Added = app.Added;
export type SaveAnswer = app.SaveAnswer;
export type Clash = app.Clash;
export type Player = app.Player;
export type Browser = app.Browser;
export type Setup = app.Setup;
export type Settings = config.Config;
export type Queue = app.Queue;
export type QueueState = app.QueueState;
export type Countdown = app.Countdown;
export type PlaylistView = app.PlaylistView;
export type PlaylistEntry = PlaylistView["entries"][number];
export type PlaylistPick = app.PlaylistPick;
export type PlaylistAdded = app.PlaylistAdded;
export type Found = app.Found;
export type Check = app.Check;
export type Said = app.Said;
export type StartAtLogin = app.StartAtLogin;
export type Subscription = app.Subscription;
export type SubscriptionChange = Parameters<typeof SubscriptionsService.Change>[1];
export type LinksView = app.LinksView;
export type Link = LinksView["links"][number];
export type LinksPick = app.LinksPick;
export type LinksAdded = app.LinksAdded;
export type Phone = app.Phone;
export type Pairing = app.Pairing;

export interface Api {
  list(): Promise<Download[]>;
  get(id: string): Promise<Download>;
  choose(id: string): Promise<void>;
  // place: "youtube" or "soundcloud".
  searchElsewhere(id: string, place: string): Promise<Found[]>;
  // remember makes place where the rest of its queue's unmatched songs are looked for.
  useFound(id: string, address: string, place: string, remember: boolean): Promise<void>;
  add(link: string): Promise<Added>;
  pause(id: string): Promise<void>;
  resume(id: string): Promise<void>;
  remove(id: string, deleteFile: boolean): Promise<void>;
  confirm(id: string, answer: SaveAnswer): Promise<void>;
  // filename "" means its own name; null when the name is free.
  clash(id: string, directory: string, filename: string): Promise<Clash | null>;
  open(id: string): Promise<void>;
  // player is from players, or "" for the default.
  watch(id: string, player?: string): Promise<void>;
  // Default first.
  players(): Promise<Player[]>;
  copyStreamAddress(id: string): Promise<void>;
  reveal(id: string): Promise<void>;
  copyLink(id: string): Promise<void>;
  // Oldest first.
  activity(id: string): Promise<Said[]>;
  pauseAll(): Promise<void>;
  resumeAll(): Promise<void>;
  clearDone(): Promise<void>;
  // text is a pasted checksum, or "" for the one Snag found.
  check(id: string, text: string): Promise<void>;
  // Replaces the file once complete.
  redownload(id: string): Promise<Download>;
  copyChecksum(id: string): Promise<string>;

  settings(): Promise<Settings>;
  setSettings(settings: Settings): Promise<Settings>;
  chooseFolder(start: string): Promise<string>;
  startAtLogin(): Promise<StartAtLogin>;
  setStartAtLogin(on: boolean): Promise<StartAtLogin>;
  places(): Promise<string[]>;
  home(): Promise<string>;
  clipboardText(): Promise<string>;
  copyText(text: string): Promise<void>;
  version(): Promise<string>;
  author(): Promise<{ name: string; page: string }>;
  openAuthorPage(): Promise<void>;
  openFirefoxAddon(): Promise<void>;
  update(): Promise<{ version: string } | null>;
  openUpdatePage(): Promise<void>;
  setup(): Promise<Setup>;
  showExtensionFolder(): Promise<void>;
  browsers(): Promise<Browser[]>;
  openExtensionsPage(id: string): Promise<void>;
  registerBrowsers(): Promise<string[]>;

  // Main first; a queue without an id is new.
  queues(): Promise<QueueState[]>;
  saveQueue(q: Queue): Promise<Queue>;
  deleteQueue(id: string, downloads: boolean): Promise<void>;
  startQueue(id: string): Promise<void>;
  stopQueue(id: string): Promise<void>;
  moveToQueue(ids: string[], queue: string): Promise<void>;
  reorder(queue: string, ids: string[]): Promise<void>;
  reverseQueue(id: string): Promise<void>;
  // Main stays first.
  arrangeQueues(ids: string[]): Promise<void>;
  toFront(id: string): Promise<void>;
  // Regardless of whether its queue runs or has room.
  startNow(id: string): Promise<void>;
  // null for none.
  countdown(): Promise<Countdown | null>;
  cancelCountdown(): Promise<void>;
  actNow(): Promise<void>;

  playlist(id: string): Promise<PlaylistView>;
  playlistHave(id: string, quality: string): Promise<boolean[]>;
  downloadPlaylist(id: string, pick: PlaylistPick): Promise<PlaylistAdded>;

  links(id: string): Promise<LinksView>;
  linksHave(id: string): Promise<boolean[]>;
  lookLinks(id: string, indexes: number[]): Promise<Link[]>;
  downloadLinks(id: string, pick: LinksPick): Promise<LinksAdded>;
  // importLinks returns "" when the user picks no file.
  addLinks(text: string): Promise<string>;
  importLinks(): Promise<string>;

  subscriptions(): Promise<Subscription[]>;
  changeSubscription(id: string, change: SubscriptionChange): Promise<Subscription>;
  checkSubscription(id: string): Promise<void>;
  openSubscriptionPage(id: string): Promise<void>;
  unsubscribe(id: string): Promise<void>;
  findSubscription(link: string): Promise<void>;

  // onPhone returns an unsubscribe function.
  phone(): Promise<Phone>;
  setPhone(on: boolean): Promise<Phone>;
  pairPhone(): Promise<Pairing>;
  stopPairing(): Promise<void>;
  removePhone(id: string): Promise<Phone>;
  onPhone(handler: (p: Phone) => void): () => void;

  closeWindow(): Promise<void>;
  // Resizes the window so the page shows height pixels.
  fitWindow(height: number): Promise<void>;

  // Returns an unsubscribe function.
  follow(handlers: Handlers): () => void;
}

export interface Handlers {
  changed(d: Download): void;
  removed(id: string): void;
  // After many downloads were added at once.
  reset(list: Download[]): void;
  settings(s: Settings): void;
  connected(): void;
  queues(list: QueueState[]): void;
  countdown(c: Countdown | null): void;
  subscriptions(list: Subscription[]): void;
}

// fitWindow calls run one at a time, and a stale one does nothing, so a slow earlier request
// can't resize the window after a later one.
let fitGeneration = 0;
let fitting: Promise<void> = Promise.resolve();

export const wailsApi: Api = {
  list: () => DownloadsService.List(),
  get: (id) => DownloadsService.Get(id),
  choose: (id) => DownloadsService.Choose(id),
  searchElsewhere: (id, place) => DownloadsService.SearchElsewhere(id, place),
  useFound: (id, address, place, remember) =>
    DownloadsService.UseFound(id, address, place, remember),
  add: (link) => DownloadsService.Add(link),
  pause: (id) => DownloadsService.Pause(id),
  resume: (id) => DownloadsService.Resume(id),
  remove: (id, deleteFile) => DownloadsService.Remove(id, deleteFile),
  confirm: (id, answer) => DownloadsService.Confirm(id, answer),
  clash: (id, directory, filename) => DownloadsService.Clash(id, directory, filename),
  open: (id) => DownloadsService.Open(id),
  watch: (id, player) => DownloadsService.Watch(id, player ?? ""),
  players: () => DownloadsService.Players(),
  copyStreamAddress: (id) => DownloadsService.CopyStreamAddress(id),
  reveal: (id) => DownloadsService.Reveal(id),
  copyLink: (id) => DownloadsService.CopyLink(id),
  activity: (id) => DownloadsService.Activity(id),
  pauseAll: () => DownloadsService.PauseAll(),
  resumeAll: () => DownloadsService.ResumeAll(),
  clearDone: () => DownloadsService.ClearDone(),
  check: (id, text) => DownloadsService.Check(id, text),
  redownload: (id) => DownloadsService.Redownload(id),
  copyChecksum: (id) => DownloadsService.CopyChecksum(id),

  settings: () => SettingsService.Get(),
  setSettings: (settings) => SettingsService.Set(settings),
  chooseFolder: (start) => SettingsService.ChooseFolder(start),
  startAtLogin: () => SettingsService.StartAtLogin(),
  setStartAtLogin: (on) => SettingsService.SetStartAtLogin(on),
  places: () => SettingsService.Places(),
  home: () => SettingsService.Home(),
  clipboardText: () => SystemService.ClipboardText(),
  copyText: (text) => SystemService.CopyText(text),
  setup: () => SetupService.Status(),
  showExtensionFolder: () => SetupService.ShowExtensionFolder(),
  browsers: () => SetupService.Browsers(),
  openExtensionsPage: (id) => SetupService.OpenExtensionsPage(id),
  registerBrowsers: () => SetupService.RegisterBrowsers(),
  version: () => SystemService.Version(),
  author: () => SystemService.Author(),
  openAuthorPage: () => SystemService.OpenAuthorPage(),
  openFirefoxAddon: () => SystemService.OpenFirefoxAddon(),
  update: () => SystemService.Update(),
  openUpdatePage: () => SystemService.OpenUpdatePage(),
  queues: () => QueuesService.List(),
  saveQueue: (q) => QueuesService.Save(q),
  deleteQueue: (id, downloads) => QueuesService.Delete(id, downloads),
  startQueue: (id) => QueuesService.Start(id),
  stopQueue: (id) => QueuesService.Stop(id),
  moveToQueue: (ids, queue) => QueuesService.Move(ids, queue),
  reorder: (queue, ids) => QueuesService.Reorder(queue, ids),
  reverseQueue: (id) => QueuesService.Reverse(id),
  arrangeQueues: (ids) => QueuesService.Arrange(ids),
  toFront: (id) => QueuesService.ToFront(id),
  startNow: (id) => QueuesService.StartNow(id),
  countdown: () => QueuesService.Countdown(),
  cancelCountdown: () => QueuesService.CancelCountdown(),
  actNow: () => QueuesService.ActNow(),
  playlist: (id) => PlaylistsService.Get(id),
  playlistHave: (id, quality) => PlaylistsService.Have(id, quality),
  downloadPlaylist: (id, pick) => PlaylistsService.Download(id, pick),
  links: (id) => LinksService.Get(id),
  linksHave: (id) => LinksService.Have(id),
  lookLinks: (id, indexes) => LinksService.Look(id, indexes),
  downloadLinks: (id, pick) => LinksService.Download(id, pick),
  addLinks: (text) => DownloadsService.AddLinks(text),
  importLinks: () => DownloadsService.ImportLinks(),
  subscriptions: () => SubscriptionsService.List(),
  changeSubscription: (id, change) => SubscriptionsService.Change(id, change),
  checkSubscription: (id) => SubscriptionsService.CheckNow(id),
  openSubscriptionPage: (id) => SubscriptionsService.OpenPage(id),
  unsubscribe: (id) => SubscriptionsService.Remove(id),
  findSubscription: (link) => SubscriptionsService.Find(link),
  phone: () => PhoneService.Status(),
  setPhone: (on) => PhoneService.Set(on),
  pairPhone: () => PhoneService.Pair(),
  stopPairing: () => PhoneService.StopPairing(),
  removePhone: (id) => PhoneService.Remove(id),
  onPhone: (handler) => Events.On("phone:changed", (e) => handler(e.data)),

  closeWindow: () => Window.Close(),
  fitWindow(height) {
    // The window's size includes the frame, the page's doesn't. Applying an earlier measurement
    // last would shrink the window and leave the page clipped.
    const generation = ++fitGeneration;
    const apply = async () => {
      if (generation !== fitGeneration) return;
      const size = await Window.Size();
      if (generation !== fitGeneration) return;
      const wanted = Math.ceil(height) + Math.max(size.height - window.innerHeight, 0);
      if (wanted !== size.height) await Window.SetSize(size.width, wanted);
    };
    const run = fitting.then(apply, apply);
    fitting = run.then(
      () => {},
      () => {},
    );
    return run;
  },

  follow(handlers) {
    const stops = [
      Events.On("downloads:changed", (e) => handlers.changed(e.data)),
      Events.On("downloads:removed", (e) => handlers.removed(e.data)),
      Events.On("downloads:reset", (e) => handlers.reset(e.data ?? [])),
      Events.On("settings:changed", (e) => handlers.settings(e.data)),
      Events.On("browser:connected", () => handlers.connected()),
      Events.On("queues:changed", (e) => handlers.queues(e.data)),
      Events.On("queue:countdown", (e) => handlers.countdown(e.data ?? null)),
      Events.On("subscriptions:changed", (e) => handlers.subscriptions(e.data ?? [])),
    ];
    return () => stops.forEach((stop) => stop());
  },
};
