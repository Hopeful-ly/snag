// Sample windows with no Go side, for styling: `npm run dev`, then open /preview.html.
// Flags: ?menu ?settings ?connect ?save (with ?taken, ?asked) ?queues ?queue=night ?newqueue
// ?countdown ?playlist ?links ?subscriptions (with ?following or ?following=s1) ?checksums
// ?finishing.
import { mount } from "svelte";
import App from "../App.svelte";
import LinksWindow from "../LinksWindow.svelte";
import PlaylistWindow from "../PlaylistWindow.svelte";
import SaveWindow from "../SaveWindow.svelte";
import {
  FakeApi,
  sampleDownload,
  samplePlaylist,
  sampleQueues,
  sampleSubscriptions,
  subscriptionDownloads,
} from "./fake-api";
import "../lib/styles/tokens.css";
import "../lib/styles/base.css";

const api = new FakeApi();
const ask = new URLSearchParams(location.search);
if (ask.has("connect")) api.setupValue.connectedBefore = false;
if (ask.has("taken")) {
  api.clashes.set("/home/me/Downloads/A Day at the Beach.mp4", {
    existing: "A Day at the Beach.mp4",
    inUse: false,
    keepBoth: "A Day at the Beach (1)",
  });
}
if (ask.has("subscriptions")) {
  api.subscriptionsValue = sampleSubscriptions();
  api.downloads = [...subscriptionDownloads(), ...api.downloads];
}
if (ask.has("hyprland"))
  api.startAtLoginValue = {
    on: true,
    available: true,
    desktop: "Hyprland",
    command: "/home/me/.local/lib/snag/snag --at-login",
  };
if (ask.has("finishing")) {
  const running = api.downloads.find((d) => d.id === "running");
  if (running)
    Object.assign(running, {
      stage: "Fixing the video's container",
      speed: 0,
      eta: null,
      done: running.size * 0.995,
      pieces: 415,
      piecesDone: 415,
      heard: Date.now(),
      estimated: true,
    });
  api.activityValue.running = [
    { at: Date.now() - 9000, text: "Started" },
    { at: Date.now() - 6000, text: "[hlsnative] Downloading m3u8 manifest" },
    { at: Date.now() - 5000, text: "[download] Destination: ubuntu-26.04-desktop-amd64.mp4" },
    { at: Date.now() - 2000, text: "Fixing the video's container…" },
    {
      at: Date.now() - 2000,
      text: '[FixupM3u8] Fixing MPEG-TS in MP4 container of "ubuntu-26.04-desktop-amd64.mp4"',
    },
  ];
}
if (ask.has("checksums")) {
  const check = (result: string, from: string) => ({
    algo: "SHA-256",
    weak: false,
    from,
    result,
    want: "3f0b".repeat(16),
    got: result === "mismatch" ? "9c1e".repeat(16) : result === "match" ? "3f0b".repeat(16) : "",
    error: "",
    read: 0,
  });
  const done = (id: string, name: string, c: ReturnType<typeof check>) =>
    sampleDownload({
      id,
      name,
      status: "done",
      path: `/home/me/Downloads/${name}`,
      size: 2.1 * 2 ** 30,
      done: 2.1 * 2 ** 30,
      finished: Date.now() - 600_000,
      check: c,
    });
  api.downloads.unshift(
    done("c1", "ubuntu-26.04-desktop-amd64.iso", check("", "page")),
    done("c2", "Fedora-Workstation-Live-44.iso", check("match", "file")),
    done("c3", "tool-setup.exe", check("mismatch", "github")),
  );
}
if (ask.has("queues")) {
  api.queuesValue = sampleQueues();
  const GB = 2 ** 30;
  api.downloads.push(
    sampleDownload({
      id: "n1",
      name: "debian-13.2-amd64-DVD-1.iso",
      size: 3.9 * GB,
      queue: "night",
      position: 20,
    }),
    sampleDownload({
      id: "n2",
      name: "Planet Earth III, episode 4.mp4",
      kind: "stream",
      label: "1080p",
      size: 0.6 * GB,
      estimated: true,
      queue: "night",
      position: 21,
    }),
    sampleDownload({
      id: "n3",
      name: "blender-5.1-windows-x64.zip",
      size: 0.38 * GB,
      queue: "night",
      position: 22,
    }),
    sampleDownload({
      id: "n4",
      name: "Lecture 12 – Distributed systems.mp4",
      kind: "stream",
      label: "720p",
      size: 0.14 * GB,
      estimated: true,
      queue: "night",
      position: 23,
    }),
  );
  if (ask.has("countdown"))
    api.countdownValue = {
      queue: "night",
      name: "Tonight",
      action: "shutdown",
      at: new Date(Date.now() + 54_000).toISOString() as unknown as never,
    };
}
const target = document.getElementById("app");
if (!target) throw new Error("missing #app element");

if (ask.has("playlist")) {
  const season = samplePlaylist();
  season.entries = Array.from({ length: 12 }, (_, i) => ({
    url: `https://v.example.com/lanterns/1/${i + 1}`,
    title: `Episode ${i + 1}: ${["The Harbour", "Low Tide", "Night Ferry", "Signal Fires"][i % 4]}`,
    duration: 1380 + i * 61,
    group: i < 6 ? "Season 1" : "Season 2",
  }));
  season.current = 3;
  api.playlistValue = season;
  api.playlistHaveValue = { best: season.entries.map((_, i) => i === 4) };
  mount(PlaylistWindow, { target, props: { api, id: season.id } });
} else if (ask.has("links")) {
  api.linksHaveValue = api.linksValue.links.map((_, i) => i === 2);
  mount(LinksWindow, { target, props: { api, id: api.linksValue.id } });
} else if (ask.has("save")) {
  mount(SaveWindow, { target, props: { api, id: "pending" } });
  if (ask.has("asked")) {
    setTimeout(() => document.querySelector<HTMLElement>("[aria-label=Download]")?.click(), 500);
  }
} else {
  mount(App, { target, props: { api } });
  setTimeout(() => {
    if (ask.has("settings")) document.querySelector<HTMLElement>("[aria-label=Settings]")?.click();
    const filter = ask.get("filter");
    if (filter)
      document.querySelector<HTMLElement>(`nav button:nth-of-type(${Number(filter)})`)?.click();
    const queue = ask.get("queue");
    if (queue)
      document
        .querySelector<HTMLElement>(
          `nav button[aria-label^="${queue === "night" ? "Tonight" : queue}"]`,
        )
        ?.click();
    const following = ask.get("following");
    if (following)
      document.querySelector<HTMLElement>(`nav button[aria-label^="Lanterns"]`)?.click();
    else if (ask.has("following"))
      document.querySelector<HTMLElement>(`nav button[aria-label^="All subscriptions"]`)?.click();
    if (ask.has("newqueue"))
      document.querySelector<HTMLElement>("[aria-label='New queue']")?.click();
    if (ask.has("edit"))
      setTimeout(
        () => document.querySelector<HTMLElement>("[aria-label='Edit queue']")?.click(),
        200,
      );
    if (ask.has("menu"))
      document
        .querySelectorAll("[role=option]")[1]
        ?.dispatchEvent(
          new MouseEvent("contextmenu", { bubbles: true, clientX: 520, clientY: 200 }),
        );
  }, 300);
}
