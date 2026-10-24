<script lang="ts">
  import Icon from "./Icon.svelte";
  import IconButton from "./IconButton.svelte";
  import ProgressBar from "./ProgressBar.svelte";
  import type { Said } from "../api/client";
  import {
    checkText,
    detailsStatus,
    duration,
    host,
    percent,
    quietFor,
    size,
    stagePercent,
  } from "../format";
  import { primaryOf } from "../state/actions";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const d = $derived(snag.detailsFor ? snag.downloads.get(snag.detailsFor) : undefined);
  const queue = $derived(d ? snag.queueNamed(d.queue) : undefined);
  const primary = $derived(d ? primaryOf(d, queue?.running ?? true) : null);
  const busyAction = $derived(d ? snag.busyActionOf(d.id) : "");
  // While finishing, show the finishing step's progress; the download's own stays at its end.
  const pct = $derived(d ? (d.stage ? stagePercent(d) : percent(d)) : -1);

  let now = $state(Date.now());
  let said = $state<Said[]>([]);
  let log: HTMLElement | undefined = $state();
  let stuck = true; // the log follows its newest line unless scrolled up

  $effect(() => {
    const id = d?.id;
    const running = d?.status === "downloading" || d?.status === "pausing";
    if (!id) return;
    let alive = true;
    const look = () =>
      snag.api.activity(id).then(
        (lines) => {
          if (alive) said = lines;
        },
        () => {},
      );
    void look();
    const timer = setInterval(() => {
      now = Date.now();
      if (running) void look();
    }, 1000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  });

  $effect(() => {
    void said.length;
    if (log && stuck) log.scrollTop = log.scrollHeight;
  });

  const quiet = $derived(d ? quietFor(d, now) : 0);
  const status = $derived(d ? detailsStatus(d, quiet) : null);

  function clock(ms: number): string {
    return new Date(ms).toLocaleTimeString(undefined, { hour12: false });
  }

  function when(ms: number): string {
    return ms
      ? new Date(ms).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" })
      : "";
  }

  async function copyLog() {
    const text = said.map((l) => `${clock(l.at)}  ${l.text}`).join("\n");
    if (await snag.request("", () => snag.api.copyText(text)))
      snag.showToast("Copied what it said", "good");
  }
</script>

{#if d && status}
  <aside class="details" aria-label="Details of {d.name}">
    <header>
      <h2 title={d.path || d.url}>{d.name}</h2>
      <IconButton
        icon="x"
        tip="Close (Esc)"
        kind="ghost"
        size={30}
        iconSize={16}
        onclick={() => (snag.detailsFor = null)}
      />
    </header>

    <section class="state {status.tone}" role="status">
      <div class="headline">
        <span>{status.title}</span>
        {#if pct >= 0 && d.status !== "done"}<span class="pct numbers">{pct}%</span>{/if}
      </div>
      {#if d.status !== "done" && d.status !== "pending"}
        <ProgressBar
          value={pct >= 0 ? pct / 100 : 0}
          indeterminate={d.status === "downloading" && pct < 0}
          color={d.status === "error" ? "var(--bad)" : "var(--accent)"}
        />
      {/if}
      {#if status.why}<p class="why">{status.why}</p>{/if}
    </section>

    <dl class="facts numbers">
      <dt>Size</dt>
      <dd>
        {#if d.size}
          {size(d.done) || "0 B"} of {d.estimated ? "about " : ""}{size(d.size)}
        {:else}
          {size(d.done) || "Not known yet"}
        {/if}
        {#if d.estimated && d.status !== "done"}<span class="faint">
            (a guess until it's done)</span
          >{/if}
      </dd>
      {#if d.status === "downloading" && !d.stage}
        <dt>Speed</dt>
        <dd>{d.speed ? `${size(d.speed)}/s` : "Nothing coming right now"}</dd>
        {#if d.eta && d.eta > 0}
          <dt>Time left</dt>
          <dd>{duration(d.eta)}</dd>
        {/if}
      {/if}
      {#if d.pieces}
        <dt>Pieces</dt>
        <dd>{d.piecesDone} of {d.pieces}</dd>
      {/if}
      {#if d.connections}
        <dt>Connections</dt>
        <dd>{d.connections}</dd>
      {/if}
      {#if d.heard && d.status === "downloading"}
        <dt>Last heard</dt>
        <dd>{quiet < 2000 ? "just now" : `${duration(quiet / 1000)} ago`}</dd>
      {/if}
      {#if d.label}
        <dt>Quality</dt>
        <dd>{d.label}</dd>
      {/if}
      <dt>Kind</dt>
      <dd>{d.kind === "stream" ? "Video site or stream, through yt-dlp" : "File"}</dd>
      {#if queue && snag.queues.length > 1}
        <dt>Queue</dt>
        <dd>{queue.name}</dd>
      {/if}
      <dt>From</dt>
      <dd class="wrap" title={d.url}>{host(d.page || d.url) || d.url}</dd>
      <dt>Saved to</dt>
      <dd class="wrap">{snag.short(d.path || d.directory)}</dd>
      <dt>Added</dt>
      <dd>{when(d.created)}</dd>
      {#if d.finished}
        <dt>Finished</dt>
        <dd>{when(d.finished)}</dd>
      {/if}
      {#if d.check && d.check.result !== "looking"}
        <dt>Checksum</dt>
        <dd>{checkText(d.check, d.size)}</dd>
      {/if}
    </dl>

    <div class="actions">
      {#if primary}
        <IconButton
          icon={primary.icon}
          label={primary.tip}
          busy={busyAction === primary.action}
          disabled={busyAction !== ""}
          onclick={() => snag.act(d.id, primary.action)}
        />
      {/if}
      {#if d.status !== "pending"}
        <IconButton
          icon="folder"
          label="Show in folder"
          busy={busyAction === "reveal"}
          disabled={busyAction !== ""}
          onclick={() => snag.act(d.id, "reveal")}
        />
      {/if}
      <IconButton icon="link" tip="Copy link" onclick={() => snag.act(d.id, "copy")} />
    </div>

    <div class="log-head">
      <h3>What it said</h3>
      {#if said.length}
        <IconButton
          icon="paste"
          tip="Copy all of it"
          kind="ghost"
          size={26}
          iconSize={14}
          onclick={copyLog}
        />
      {/if}
    </div>
    <div
      class="log"
      bind:this={log}
      onscroll={() => {
        if (log) stuck = log.scrollTop + log.clientHeight >= log.scrollHeight - 8;
      }}
    >
      {#each said as line, i (i)}
        <div class="line"><span class="at">{clock(line.at)}</span> {line.text}</div>
      {:else}
        <div class="line faint">
          <Icon name="clock" size={13} /> Nothing yet: it says what it's doing once it runs.
        </div>
      {/each}
    </div>
  </aside>
{/if}

<style>
  .details {
    display: flex;
    flex-direction: column;
    gap: 14px;
    min-height: 0;
    padding: 16px 16px 14px;
    border-left: 1px solid var(--border);
    background: var(--sidebar);
    overflow-y: auto;
  }

  header {
    display: flex;
    align-items: flex-start;
    gap: 8px;
  }

  h2 {
    flex: 1;
    margin: 4px 0 0;
    font-size: var(--text-md);
    font-weight: 600;
    word-break: break-word;
  }

  .state {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px;
    border-radius: var(--radius-lg);
    background: var(--surface);
  }

  .headline {
    display: flex;
    justify-content: space-between;
    font-weight: 600;
  }

  .state.good .headline {
    color: var(--good);
  }

  .state.warn .headline {
    color: var(--warn);
  }

  .state.bad .headline {
    color: var(--bad);
  }

  .state.busy .headline {
    color: var(--accent);
  }

  .why {
    margin: 0;
    color: var(--text-dim);
    font-size: var(--text-sm);
    line-height: 1.45;
  }

  .facts {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 6px 14px;
    margin: 0;
    font-size: var(--text-sm);
  }

  dt {
    color: var(--text-faint);
  }

  dd {
    min-width: 0;
    margin: 0;
    color: var(--text);
  }

  .wrap {
    overflow-wrap: anywhere;
  }

  .faint {
    color: var(--text-faint);
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .log-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  h3 {
    margin: 0;
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.6px;
    text-transform: uppercase;
  }

  .log {
    flex: 1 0 140px;
    min-height: 140px;
    overflow-y: auto;
    padding: 8px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--field);
    font-family: ui-monospace, "JetBrains Mono", "Cascadia Mono", Consolas, monospace;
    font-size: 11.5px;
    line-height: 1.5;
    user-select: text;
  }

  .line {
    overflow-wrap: anywhere;
    color: var(--text-dim);
  }

  .at {
    color: var(--text-faint);
  }
</style>
