import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import RemoteApp from "../RemoteApp.svelte";
import {
  deviceName,
  httpApi,
  platformOf,
  Unpaired,
  type Item,
  type PhoneApi,
  type Platform,
  type Request,
} from "../lib/remote/api";

const MB = 1024 * 1024;

// A stand-in for Snag's server for phones: one pairing code, one token, downloads in memory.
class FakePhoneApi implements PhoneApi {
  calls: string[] = [];
  paired = true; // false once the phone is removed in Settings
  items: Item[] = [
    {
      id: "d1",
      name: "Talk.mp4",
      status: "downloading",
      done: 30 * MB,
      size: 120 * MB,
      speed: 2 * MB,
    },
    { id: "d2", name: "Album.zip", status: "done", done: 80 * MB, size: 80 * MB, speed: 0 },
  ];

  async pair(code: string, name: string) {
    this.calls.push(`pair ${code} ${name}`);
    if (code !== "code1" && code !== "invite1") throw new Error("that code doesn't match");
    return "token1";
  }

  async send(token: string, req: Request) {
    if (!this.paired || token !== "token1")
      throw new Unpaired("This phone isn't paired with Snag any more.");
    if (req.op !== "list") this.calls.push([req.op, req.link ?? req.id].filter(Boolean).join(" "));
    switch (req.op) {
      case "list":
        return { items: structuredClone(this.items) };
      case "add": {
        const item = { id: "d3", name: "clip.mp4", status: "queued", done: 0, size: 0, speed: 0 };
        this.items.unshift(item);
        return { item };
      }
      case "invite":
        return { code: "invite1" };
      default:
        return {};
    }
  }
}

function show(
  api = new FakePhoneApi(),
  { platform = "other", homeScreen = false }: { platform?: Platform; homeScreen?: boolean } = {},
) {
  const user = userEvent.setup();
  render(RemoteApp, {
    props: { api, storage: localStorage, every: 60_000, platform, homeScreen },
  });
  return { api, user };
}

describe("the phone page", () => {
  beforeEach(() => {
    localStorage.clear();
    history.replaceState(null, "", "/");
  });

  it("asks to pair when it has no token", () => {
    show();
    expect(screen.getByText(/scan the QR code with this phone's camera/)).toBeInTheDocument();
    expect(screen.queryByLabelText("Link")).not.toBeInTheDocument();
  });

  it("pairs with the code from the QR code, once, and keeps the token", async () => {
    history.replaceState(null, "", "/#pair=code1");
    const { api } = show();
    expect(await screen.findByText(/Paired with Snag/)).toBeInTheDocument();
    expect(api.calls[0]).toMatch(/^pair code1 /);
    expect(location.hash).toBe(""); // the code isn't kept
    expect(localStorage.getItem("snag-token")).toBe("token1");
    expect(await screen.findByRole("listitem", { name: "Talk.mp4" })).toBeInTheDocument();
  });

  it("says when pairing didn't work", async () => {
    history.replaceState(null, "", "/#pair=old");
    show();
    expect(await screen.findByText("Couldn't pair: that code doesn't match")).toBeInTheDocument();
    expect(localStorage.getItem("snag-token")).toBeNull();
  });

  it("lists the downloads with their progress, and sends a link", async () => {
    localStorage.setItem("snag-token", "token1");
    const { api, user } = show();
    const talk = await screen.findByRole("listitem", { name: "Talk.mp4" });
    expect(talk).toHaveTextContent("30 MB of 120 MB · 2.0 MB/s");
    expect(within(talk).getByRole("progressbar")).toHaveAttribute("aria-valuenow", "25");
    expect(screen.getByRole("listitem", { name: "Album.zip" })).toHaveTextContent("Done · 80 MB");

    const box = screen.getByLabelText("Link");
    await user.type(box, " https://example.com/clip ");
    await user.click(screen.getByRole("button", { name: "Send to Snag" }));
    expect(await screen.findByText("Downloading clip.mp4")).toBeInTheDocument();
    expect(api.calls).toContain("add https://example.com/clip");
    expect(box).toHaveValue("");
    expect(await screen.findByRole("listitem", { name: "clip.mp4" })).toHaveTextContent("Waiting");
  });

  it("pauses, resumes and clears", async () => {
    localStorage.setItem("snag-token", "token1");
    const { api, user } = show();
    const talk = await screen.findByRole("listitem", { name: "Talk.mp4" });
    await user.click(within(talk).getByRole("button", { name: "Pause" }));
    await user.click(
      within(screen.getByRole("listitem", { name: "Album.zip" })).getByRole("button", {
        name: "Clear from the list",
      }),
    );
    await waitFor(() => expect(api.calls).toEqual(["pause d1", "forget d2"]));
  });

  it("forgets its token once Snag no longer knows the phone", async () => {
    localStorage.setItem("snag-token", "token1");
    const api = new FakePhoneApi();
    api.paired = false;
    show(api);
    expect(
      await screen.findByText("This phone isn't paired with Snag any more."),
    ).toBeInTheDocument();
    expect(localStorage.getItem("snag-token")).toBeNull();
    expect(screen.queryByLabelText("Link")).not.toBeInTheDocument();
  });
});

describe("the home screen", () => {
  beforeEach(() => {
    localStorage.clear();
    history.replaceState(null, "", "/");
  });

  it("readies an iPhone's address with a code for the home screen copy", async () => {
    localStorage.setItem("snag-token", "token1");
    const { api, user } = show(undefined, { platform: "ios" });
    await user.click(await screen.findByText("Put Snag on your home screen"));
    await user.click(screen.getByRole("button", { name: "Get it ready" }));
    expect(await screen.findByText("Now tap Share, then Add to Home Screen")).toBeInTheDocument();
    expect(api.calls).toContain("invite");
    expect(location.hash).toBe("#join=invite1");
  });

  it("pairs the home screen copy with that code, under its own name", async () => {
    history.replaceState(null, "", "/#join=invite1");
    const { api } = show(undefined, { platform: "ios", homeScreen: true });
    expect(await screen.findByText(/Paired with Snag/)).toBeInTheDocument();
    expect(api.calls[0]).toMatch(/^pair invite1 .*, home screen$/);
    expect(location.hash).toBe("");
    expect(screen.queryByText("Put Snag on your home screen")).not.toBeInTheDocument();
  });

  it("leaves the code alone in Safari, already paired", async () => {
    localStorage.setItem("snag-token", "token1");
    history.replaceState(null, "", "/#join=invite1");
    const { api } = show(undefined, { platform: "ios" });
    expect(await screen.findByRole("listitem", { name: "Talk.mp4" })).toBeInTheDocument();
    expect(api.calls).toEqual([]);
    expect(location.hash).toBe("");
  });

  it("tells Android to use the browser's menu", async () => {
    localStorage.setItem("snag-token", "token1");
    const { user } = show(undefined, { platform: "android" });
    await user.click(await screen.findByText("Put Snag on your home screen"));
    expect(screen.getByText(/It stays paired/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Get it ready" })).not.toBeInTheDocument();
  });

  it("isn't offered on a computer", async () => {
    localStorage.setItem("snag-token", "token1");
    show();
    await screen.findByRole("listitem", { name: "Talk.mp4" });
    expect(screen.queryByText("Put Snag on your home screen")).not.toBeInTheDocument();
  });
});

describe("httpApi", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("says what to check when Snag can't be reached", async () => {
    vi.stubGlobal("fetch", () => Promise.reject(new TypeError("Failed to fetch")));
    await expect(httpApi.send("token1", { op: "list" })).rejects.toThrow(/Can't reach Snag/);
  });
});

describe("platformOf", () => {
  it("tells iPhones, iPads and Android apart from computers", () => {
    expect(platformOf("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)")).toBe("ios");
    expect(platformOf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605", 5)).toBe("ios");
    expect(platformOf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605", 0)).toBe(
      "other",
    );
    expect(platformOf("Mozilla/5.0 (Linux; Android 15; Pixel 9) Chrome/130.0")).toBe("android");
    expect(platformOf("Mozilla/5.0 (X11; Linux x86_64) Firefox/131.0")).toBe("other");
  });
});

describe("deviceName", () => {
  it("names the phone and its browser", () => {
    const names: Record<string, string> = {
      "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1":
        "iPhone, Safari",
      "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/130.0 Mobile/15E148 Safari/604.1":
        "iPhone, Chrome",
      "Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36":
        "Android, Chrome",
      "Mozilla/5.0 (Linux; Android 15; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/27.0 Chrome/125.0 Mobile Safari/537.36":
        "Android, Samsung Internet",
      "Mozilla/5.0 (Android 15; Mobile; rv:131.0) Gecko/131.0 Firefox/131.0": "Android, Firefox",
      "": "A phone",
    };
    for (const [ua, name] of Object.entries(names)) expect(deviceName(ua)).toBe(name);
  });
});
