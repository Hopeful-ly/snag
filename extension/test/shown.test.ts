import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { actShown } from "../src/page/shown";

type Script = chrome.scripting.RegisteredContentScript;

// The browser's scripting API as far as Snag uses it, keeping what is registered.
const registered = new Map<string, Script>();
const scripting = {
  registerContentScripts: vi.fn(async (scripts: Script[]) => {
    for (const s of scripts) registered.set(s.id, s);
  }),
  updateContentScripts: vi.fn(async (scripts: Script[]) => {
    for (const s of scripts) registered.set(s.id, { ...registered.get(s.id)!, ...s });
  }),
  unregisterContentScripts: vi.fn(async ({ ids }: { ids: string[] }) => {
    for (const id of ids) registered.delete(id);
  }),
  getRegisteredContentScripts: vi.fn(async () => [...registered.values()]),
};

let shown: typeof import("../src/background/shown");
beforeAll(async () => {
  vi.stubGlobal("chrome", { scripting });
  shown = await import("../src/background/shown");
});
afterEach(() => registered.clear());

describe("a page in a background tab", () => {
  it("acts as if shown", () => {
    vi.useFakeTimers();
    Object.defineProperty(Document.prototype, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    actShown(window);
    expect(document.visibilityState).toBe("visible");
    expect(document.hidden).toBe(false);
    const draw = vi.fn();
    requestAnimationFrame(draw);
    vi.advanceTimersByTime(20);
    expect(draw).toHaveBeenCalledOnce();
    cancelAnimationFrame(requestAnimationFrame(draw));
    vi.advanceTimersByTime(20);
    expect(draw).toHaveBeenCalledOnce();
    vi.useRealTimers();
  });
});

describe("the page script's registration", () => {
  it("covers the episode's site and its frames' sites while the tab loads", async () => {
    const s = (await shown.Shown.start("https://streamer.example/watch/tv/1396/1/2"))!;
    s.watch(7);
    const [script] = [...registered.values()];
    expect(script).toMatchObject({
      matches: ["https://streamer.example/*"],
      js: ["shown.js"],
      world: "MAIN",
      allFrames: true,
      runAt: "document_start",
    });
    shown.frameRequested(7, "https://player.example/embed/1");
    shown.frameRequested(8, "https://elsewhere.example/");
    shown.frameRequested(7, "https://streamer.example/frame");
    await s.add("https://player.example/embed/2");
    expect(registered.get(script!.id)?.matches).toEqual([
      "https://streamer.example/*",
      "https://player.example/*",
    ]);
    await s.stop();
    expect(registered.size).toBe(0);
    shown.frameRequested(7, "https://later.example/");
    expect(registered.size).toBe(0);
  });

  it("clears what an earlier background left, and nothing else", async () => {
    registered.set("snag-shown-1-0", { id: "snag-shown-1-0", js: ["shown.js"] });
    registered.set("other", { id: "other", js: ["x.js"] });
    await shown.clearShown();
    expect([...registered.keys()]).toEqual(["other"]);
  });

  it("names a site by its address, for web pages only", () => {
    expect(shown.siteOf("https://streamer.example:8443/tv/1?play=true")).toBe(
      "https://streamer.example/*",
    );
    expect(shown.siteOf("about:blank")).toBe("");
    expect(shown.siteOf("not a link")).toBe("");
  });
});
