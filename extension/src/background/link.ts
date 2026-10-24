// The native-messaging line to Snag: the browser starts snag-host on the first request.
import { api } from "../shared/browser";
import type { FromSnag, ReplyTo, Request } from "../shared/protocol";

const HOST = "snag.host";

let port: chrome.runtime.Port | null = null;
let lastRid = 0;
let problem = ""; // why the line is down, as the browser or the helper put it
const pending = new Map<number, { resolve: (reply: never) => void; reject: (e: Error) => void }>();

type Listener = (message: FromSnag) => void;
const listeners: Listener[] = [];
const openers: (() => void)[] = [];

export function onMessage(listener: Listener): void {
  listeners.push(listener);
}

export function onOpen(f: () => void): void {
  openers.push(f);
}

// The browser's own words for a missing or stopped helper mean little to people.
function friendly(error: string): string {
  if (/not found|Specified native messaging host/i.test(error)) {
    return "Snag isn't set up for this browser. Start Snag once, then try again.";
  }
  if (/exited|disconnected/i.test(error)) return "Snag's helper stopped. Start Snag and try again.";
  return error || "Snag isn't running.";
}

/** Opens the line if it isn't open; null if the browser couldn't start the helper. */
function connect(): chrome.runtime.Port | null {
  if (port) return port;
  problem = "";
  let p: chrome.runtime.Port;
  try {
    p = api.runtime.connectNative(HOST);
  } catch (error) {
    problem = String((error as Error)?.message || error);
    return null;
  }
  port = p;
  p.onMessage.addListener(received);
  p.onDisconnect.addListener((gone: chrome.runtime.Port & { error?: Error }) => {
    const error = api.runtime.lastError?.message || gone?.error?.message || "";
    if (port === gone) port = null;
    problem = error || problem || "Snag stopped";
    const text = friendly(problem);
    for (const { reject } of pending.values()) reject(new Error(text));
    pending.clear();
  });
  for (const f of openers) f();
  return p;
}

export function send<R extends Request>(request: R, timeout = 20_000): Promise<ReplyTo<R["type"]>> {
  return new Promise((resolve, reject) => {
    const p = connect();
    if (!p) return reject(new Error(friendly(problem)));
    const rid = ++lastRid;
    pending.set(rid, { resolve: resolve as (reply: never) => void, reject });
    try {
      p.postMessage({ ...request, rid });
    } catch (error) {
      pending.delete(rid);
      return reject(error);
    }
    setTimeout(() => {
      if (pending.delete(rid)) reject(new Error("Snag didn't answer in time"));
    }, timeout);
  });
}

function received(message: FromSnag): void {
  if (message.type === "reply") {
    const waiter = pending.get(message.rid);
    if (!waiter) return;
    pending.delete(message.rid);
    if (message.ok) waiter.resolve(message as never);
    else waiter.reject(new Error(message.error || "failed"));
    return;
  }
  if (message.type === "error") problem = message.error;
  for (const listener of listeners) listener(message);
}
