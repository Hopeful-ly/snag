// See internal/remote. The page takes a PhoneApi so tests can pass a fake.

// No paths, nothing from the disk.
export interface Item {
  id: string;
  name: string;
  status: string;
  done: number;
  size: number; // 0 while unknown
  speed: number; // bytes per second
}

export interface Request {
  op: "look" | "add" | "list" | "pause" | "resume" | "forget" | "invite";
  link?: string;
  choice?: number; // for add: an index into Look's choices; none for Snag's default
  id?: string;
}

// Look is what a link offers. No choices: a file, added as it is. list: a playlist, a channel or
// an album, which Snag opens on the computer to pick from.
export interface Look {
  title?: string;
  site?: string;
  duration?: number; // seconds
  choices?: Option[];
  list?: boolean;
  count?: number;
}

export interface Option {
  label: string; // "1080p", "Audio MP3"
  detail: string; // "MP4 · HDR"
  size: number; // estimated; 0 when unknown
}

export interface Response {
  items?: Item[];
  item?: Item;
  look?: Look;
  code?: string; // for invite: a one-time code to pair another copy of the page
  error?: string;
}

// Unpaired is thrown when Snag doesn't know this phone's token: it was removed in Settings.
export class Unpaired extends Error {}

export interface PhoneApi {
  pair(code: string, name: string): Promise<string>;
  send(token: string, req: Request): Promise<Response>;
}

async function post(
  path: string,
  body: unknown,
  token = "",
): Promise<Response & { token?: string }> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  let res: globalThis.Response;
  try {
    res = await fetch(path, { method: "POST", headers, body: JSON.stringify(body) });
  } catch {
    throw new Error(
      "Can't reach Snag. Check that the computer is on, Snag is running, and this phone is on the same Wi-Fi.",
    );
  }
  if (res.status === 401) throw new Unpaired("This phone isn't paired with Snag any more.");
  if (res.status === 429) throw new Error("Too many requests: wait a minute.");
  let out: Response & { token?: string };
  try {
    out = await res.json();
  } catch {
    throw new Error(`Snag answered ${res.status}`);
  }
  if (out.error) throw new Error(out.error);
  return out;
}

export const httpApi: PhoneApi = {
  async pair(code, name) {
    const out = await post("/pair", { code, name });
    if (!out.token) throw new Error("Snag didn't give a token");
    return out.token;
  },
  send: (token, req) => post("/api", req, token),
};

export type Platform = "ios" | "android" | "other";

export function platformOf(ua: string, touchPoints = 0): Platform {
  // An iPad's Safari says it's a Mac; only the touch screen tells them apart.
  if (/iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && touchPoints > 1)) return "ios";
  return /Android/.test(ua) ? "android" : "other";
}

// "iPhone, Safari", or "iPhone, home screen" for the home-screen copy.
export function deviceName(ua: string, homeScreen = false): string {
  const device = /iPad/.test(ua)
    ? "iPad"
    : /iPhone|iPod/.test(ua)
      ? "iPhone"
      : /Android/.test(ua)
        ? "Android"
        : /Macintosh/.test(ua)
          ? "Mac"
          : /Windows/.test(ua)
            ? "Windows"
            : /Linux/.test(ua)
              ? "Linux"
              : "A phone";
  const browser = homeScreen
    ? "home screen"
    : /EdgA?\//.test(ua)
      ? "Edge"
      : /SamsungBrowser\//.test(ua)
        ? "Samsung Internet"
        : /Firefox\/|FxiOS\//.test(ua)
          ? "Firefox"
          : /Chrome\/|CriOS\//.test(ua)
            ? "Chrome"
            : /Safari\//.test(ua)
              ? "Safari"
              : "";
  return browser ? `${device}, ${browser}` : device;
}
