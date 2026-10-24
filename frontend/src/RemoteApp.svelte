<script lang="ts">
  import { onMount, untrack } from "svelte";
  import IconButton from "./lib/components/IconButton.svelte";
  import ProgressBar from "./lib/components/ProgressBar.svelte";
  import { size } from "./lib/format";
  import {
    deviceName,
    platformOf,
    Unpaired,
    type Item,
    type PhoneApi,
    type Platform,
    type Request,
  } from "./lib/remote/api";

  // The QR code opens this page with a pairing code after #, traded for a token the browser keeps.
  // On iPhone the home-screen copy has storage separate from Safari's, so it can't reuse Safari's
  // token: the paired page puts a one-time code after #join= in its address, which the home-screen
  // copy is made from and pairs with on first open.
  let {
    api,
    storage = localStorage,
    every = 2000,
    platform = platformOf(navigator.userAgent, navigator.maxTouchPoints),
    homeScreen = isHomeScreen(),
  }: {
    api: PhoneApi;
    storage?: Storage;
    every?: number;
    platform?: Platform;
    homeScreen?: boolean;
  } = $props();

  const tokenKey = "snag-token";
  let token = $state(untrack(() => storage.getItem(tokenKey)) ?? "");
  let items = $state<Item[] | null>(null);
  let link = $state("");
  let sending = $state(false);
  let pairing = $state(false);
  let busy = $state<Record<string, boolean>>({});
  let message = $state<{ text: string; tone: "good" | "bad" } | null>(null);
  let inviting = $state(false);
  let invited = $state(false);

  function isHomeScreen() {
    return (
      ("standalone" in navigator && navigator.standalone === true) ||
      !!window.matchMedia?.("(display-mode: standalone)").matches
    );
  }

  const say = (text: string, tone: "good" | "bad" = "good") => (message = { text, tone });
  const messageOf = (error: unknown) => (error instanceof Error ? error.message : String(error));

  function forget(text: string) {
    storage.removeItem(tokenKey);
    token = "";
    items = null;
    say(text, "bad");
  }

  async function send(req: Request) {
    try {
      return await api.send(token, req);
    } catch (error) {
      if (error instanceof Unpaired) forget(error.message);
      throw error;
    }
  }

  async function pair(code: string) {
    pairing = true;
    try {
      token = await api.pair(code, deviceName(navigator.userAgent, homeScreen));
      storage.setItem(tokenKey, token);
      say("Paired with Snag.");
    } catch (error) {
      say(`Couldn't pair: ${messageOf(error)}`, "bad");
    } finally {
      pairing = false;
    }
  }

  async function refresh() {
    if (!token) return;
    try {
      items = (await send({ op: "list" })).items ?? [];
    } catch (error) {
      if (!(error instanceof Unpaired)) say(messageOf(error), "bad");
    }
  }

  async function add(e: SubmitEvent) {
    e.preventDefault();
    const sent = link.trim();
    if (!sent || sending) return;
    sending = true;
    try {
      const { item } = await send({ op: "add", link: sent });
      link = "";
      say(item ? `Downloading ${item.name}` : "Sent to Snag");
      await refresh();
    } catch (error) {
      if (!(error instanceof Unpaired)) say(messageOf(error), "bad");
    } finally {
      sending = false;
    }
  }

  async function act(op: "pause" | "resume" | "forget", id: string) {
    busy[id] = true;
    try {
      await send({ op, id });
      await refresh();
    } catch (error) {
      if (!(error instanceof Unpaired)) say(messageOf(error), "bad");
    } finally {
      delete busy[id];
    }
  }

  async function invite() {
    inviting = true;
    try {
      const { code } = await send({ op: "invite" });
      if (!code) throw new Error("Snag didn't give a code");
      history.replaceState(null, "", `${location.pathname}#join=${code}`);
      invited = true;
    } catch (error) {
      if (!(error instanceof Unpaired)) say(messageOf(error), "bad");
    } finally {
      inviting = false;
    }
  }

  function line(d: Item): string {
    const of = d.size > 0 ? `${size(d.done) || "0 B"} of ${size(d.size)}` : size(d.done);
    switch (d.status) {
      case "downloading":
        return [of, d.speed > 0 ? `${size(d.speed)}/s` : ""].filter(Boolean).join(" · ");
      case "queued":
        return "Waiting";
      case "pending":
        return "Waiting to be told where to go, on the computer";
      case "pausing":
      case "paused":
        return ["Paused", of].filter(Boolean).join(" · ");
      case "done":
        return ["Done", size(d.size || d.done)].filter(Boolean).join(" · ");
      case "error":
        return "Failed";
      default:
        return d.status;
    }
  }

  onMount(() => {
    const hash = new URLSearchParams(location.hash.slice(1));
    const join = hash.get("join");
    const code = hash.get("pair") ?? (join && (homeScreen || !token) ? join : null);
    if (location.hash) history.replaceState(null, "", location.pathname); // a code works once
    if (code) {
      void pair(code).then(refresh);
    } else {
      void refresh();
    }
    let timer: ReturnType<typeof setInterval> | undefined;
    const follow = () => {
      clearInterval(timer);
      timer = undefined;
      if (document.visibilityState !== "hidden") {
        void refresh();
        timer = setInterval(refresh, every);
      }
    };
    follow();
    document.addEventListener("visibilitychange", follow);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", follow);
    };
  });
</script>

<main>
  <header>
    <img src="/icon.png" alt="" width="28" height="28" />
    <h1>Snag</h1>
  </header>

  {#if message}
    <p class="message {message.tone}" role="status">{message.text}</p>
  {/if}

  {#if pairing}
    <p class="hint">Pairing…</p>
  {:else if !token}
    <p class="hint">
      To send links from this phone, open Snag on your computer, go to Settings, Phone, choose
      <b>Pair a phone</b>, and scan the QR code with this phone's camera.
    </p>
  {:else}
    <form onsubmit={add}>
      <input
        type="url"
        inputmode="url"
        aria-label="Link"
        placeholder="Paste a link"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        bind:value={link}
      />
      <button type="submit" class="send" disabled={sending || !link.trim()}>
        {sending ? "Sending…" : "Send to Snag"}
      </button>
    </form>

    <h2>Downloads</h2>
    {#if items === null}
      <p class="hint">Loading…</p>
    {:else if items.length === 0}
      <p class="hint">Nothing yet. Links you send appear here.</p>
    {:else}
      <ul aria-label="Downloads">
        {#each items as d (d.id)}
          <li aria-label={d.name}>
            <div class="row">
              <div class="text">
                <div class="name">{d.name}</div>
                <div class="meta numbers" class:bad={d.status === "error"}>{line(d)}</div>
              </div>
              {#if d.status === "downloading" || d.status === "queued"}
                <IconButton
                  icon="pause"
                  tip="Pause"
                  size={44}
                  busy={busy[d.id]}
                  onclick={() => act("pause", d.id)}
                />
              {:else if d.status === "paused" || d.status === "error"}
                <IconButton
                  icon={d.status === "error" ? "retry" : "play"}
                  tip={d.status === "error" ? "Try again" : "Resume"}
                  size={44}
                  busy={busy[d.id]}
                  onclick={() => act("resume", d.id)}
                />
              {:else if d.status === "done"}
                <IconButton
                  icon="x"
                  tip="Clear from the list"
                  kind="ghost"
                  size={44}
                  busy={busy[d.id]}
                  onclick={() => act("forget", d.id)}
                />
              {/if}
            </div>
            {#if d.status !== "done" && d.status !== "error"}
              <ProgressBar
                value={d.size > 0 ? d.done / d.size : 0}
                indeterminate={d.status === "downloading" && d.size <= 0}
                color={d.status === "downloading" ? "var(--accent)" : "var(--text-faint)"}
              />
            {/if}
          </li>
        {/each}
      </ul>
    {/if}

    {#if !homeScreen && platform !== "other"}
      <details class="tip">
        <summary>Put Snag on your home screen</summary>
        {#if platform === "ios"}
          {#if invited}
            <p>
              <b>Now tap Share, then Add to Home Screen</b>, and open Snag from there within 10
              minutes. It pairs on its own, so Snag's settings list it as another phone.
            </p>
          {:else}
            <p>
              The copy on the home screen keeps its own storage, apart from Safari's, so it needs
              pairing too. Get it ready, then tap Share, then Add to Home Screen.
            </p>
            <button type="button" class="secondary" disabled={inviting} onclick={invite}>
              {inviting ? "Getting ready…" : "Get it ready"}
            </button>
          {/if}
        {:else}
          <p>
            Open the browser's menu (<b>⋮</b>) and choose <b>Add to Home screen</b> or
            <b>Install app</b>. It stays paired.
          </p>
        {/if}
      </details>
    {/if}
  {/if}
</main>

<style>
  :global(html),
  :global(body) {
    margin: 0;
    background: var(--background);
    color: var(--text);
    font:
      16px / 1.4 "Inter Variable",
      Inter,
      system-ui,
      sans-serif;
    -webkit-text-size-adjust: 100%;
  }

  :global(*),
  :global(*::before),
  :global(*::after) {
    box-sizing: border-box;
  }

  main {
    max-width: 640px;
    margin: 0 auto;
    padding: max(16px, env(safe-area-inset-top)) 16px max(24px, env(safe-area-inset-bottom));
  }

  header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 16px;
  }

  header img {
    border-radius: 7px;
  }

  h1 {
    margin: 0;
    font-size: 22px;
    font-weight: 600;
  }

  h2 {
    margin: 24px 0 8px;
    color: var(--text-faint);
    font-size: 13px;
    font-weight: 600;
    letter-spacing: 0.6px;
    text-transform: uppercase;
  }

  .hint {
    color: var(--text-dim);
    line-height: 1.5;
  }

  .message {
    margin: 0 0 14px;
    padding: 10px 12px;
    border-radius: var(--radius);
    background: var(--good-soft);
    color: var(--text);
    overflow-wrap: anywhere;
  }

  .message.bad {
    background: var(--bad-soft);
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  input {
    width: 100%;
    height: 48px;
    padding: 0 14px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    outline: 0;
    background: var(--field);
    color: var(--text);
    font: inherit;
  }

  input:focus {
    border-color: var(--accent-strong);
  }

  .send {
    height: 48px;
    border: 0;
    border-radius: var(--radius);
    background: var(--accent-strong);
    color: white;
    font: inherit;
    font-weight: 600;
  }

  .send:disabled,
  .secondary:disabled {
    opacity: 0.5;
  }

  .tip {
    margin-top: 24px;
    padding: 12px 14px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    color: var(--text-dim);
    line-height: 1.5;
  }

  .tip summary {
    color: var(--text);
    font-weight: 600;
    cursor: pointer;
  }

  .tip p {
    margin: 10px 0;
  }

  .tip b {
    color: var(--text);
  }

  .secondary {
    width: 100%;
    height: 44px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--field);
    color: var(--text);
    font: inherit;
    font-weight: 600;
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    padding: 10px 0;
    border-bottom: 1px solid var(--border);
  }

  .row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 6px;
  }

  .text {
    flex: 1;
    min-width: 0;
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    color: var(--text-dim);
    font-size: 14px;
  }

  .meta.bad {
    color: var(--bad);
  }

  .numbers {
    font-variant-numeric: tabular-nums;
  }
</style>
