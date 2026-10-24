<script lang="ts">
  import Icon from "./Icon.svelte";
  import IconButton from "./IconButton.svelte";
  import type { Subscription } from "../api/client";
  import { everyText, subscriptionSource, subscriptionStatus } from "../format";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  let link = $state("");
  let field: HTMLInputElement | undefined = $state();
  let search = $state("");

  // Ticks every minute so relative times stay current.
  let now = $state(Date.now());
  $effect(() => {
    const timer = setInterval(() => (now = Date.now()), 60_000);
    return () => clearInterval(timer);
  });
  $effect(() => {
    if (snag.subscribeFocus) {
      field?.focus();
      snag.subscribeFocus = false;
    }
  });
  void snag.refreshStartAtLogin(); // the system's settings may have changed it

  const all = $derived(snag.subscriptions);
  const searchable = $derived(all.length > 6);
  const shown = $derived.by(() => {
    const q = search.trim().toLowerCase();
    if (!q) return all;
    return all.filter(
      (s) => s.title.toLowerCase().includes(q) || s.source.toLowerCase().includes(q),
    );
  });
  const troubled = $derived(all.filter((s) => s.error && !s.paused).length);
  const paused = $derived(all.filter((s) => s.paused).length);
  const downloaded = $derived(all.reduce((sum, s) => sum + s.added, 0));
  const summary = $derived(
    [
      `${all.length} ${all.length === 1 ? "subscription" : "subscriptions"}`,
      downloaded ? `${downloaded} downloaded` : "",
      paused ? `${paused} paused` : "",
      troubled ? `${troubled} can't be read` : "",
    ]
      .filter(Boolean)
      .join("  ·  "),
  );
  const checkable = $derived(all.some((s) => !s.paused && !s.checking));
  const finding = $derived(!!snag.busy["global:subscriptions:find"]);
  const looksLikeLink = $derived(/^https?:\/\/\S+$/.test(link.trim()));
  const needsLogin = $derived(
    !!snag.startAtLogin?.available && !snag.startAtLogin.on && all.some((s) => !s.paused),
  );

  function close() {
    snag.subscriptionsOpen = false;
  }

  async function find() {
    if (!looksLikeLink || finding) return;
    if (await snag.findSubscription(link)) link = "";
  }

  function glyph(s: Subscription) {
    if (s.paused) return "pause" as const;
    if (s.error) return "failed" as const;
    if (s.podcast || s.qualities.find((q) => q.id === s.quality)?.audioOnly)
      return "music" as const;
    return "video" as const;
  }

  function how(s: Subscription): string {
    const quality = s.qualities.find((q) => q.id === s.quality)?.label;
    return [everyText(s.everyHours), quality, snag.short(s.directory)]
      .filter(Boolean)
      .join("  ·  ");
  }

  function menu(e: MouseEvent, id: string) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    snag.openSubscriptionMenu(id, r.right - 200, r.bottom + 4);
  }
</script>

<svelte:window
  onkeydown={(e) => {
    const typing = e.target instanceof HTMLInputElement;
    const sheetOpen = snag.subscriptionSheet || snag.subscriptionMenu || snag.connectOpen;
    if (e.key === "Escape" && !typing && !sheetOpen) {
      close();
      e.preventDefault();
    }
  }}
/>

<section class="page" aria-label="Subscriptions">
  <header>
    <IconButton icon="back" tip="Back to downloads (Esc)" kind="ghost" onclick={close} />
    <div class="where">
      <h1>Subscriptions</h1>
      {#if all.length}<div class="summary">{summary}</div>{/if}
    </div>
    <span class="spacer"></span>
    {#if all.length}
      <IconButton
        icon="retry"
        label="Check all"
        tip="Look for what's new in every subscription that isn't paused"
        busy={!!snag.busy["global:subscriptions:check"]}
        disabled={!checkable}
        onclick={() => snag.checkAllSubscriptions()}
      />
    {/if}
  </header>

  <div class="scroll">
    <div class="column">
      <form
        class="follow"
        onsubmit={(e) => {
          e.preventDefault();
          void find();
        }}
      >
        <span class="tile"><Icon name="subscribe" size={18} /></span>
        <input
          bind:this={field}
          bind:value={link}
          class="field"
          aria-label="Address to subscribe to"
          placeholder="Paste a channel's, playlist's or podcast's address"
          disabled={finding}
        />
        <IconButton
          icon="plus"
          label={finding ? "Looking…" : "Subscribe…"}
          kind="primary"
          tip="Choose how its new ones download, and whether to get some now"
          busy={finding}
          disabled={!looksLikeLink}
          onclick={() => void find()}
        />
      </form>
      <p class="hint">
        In the browser, a channel's or playlist's Download card has <b>Subscribe</b> too. New videos,
        songs and episodes download on their own, each subscription into its own queue.
      </p>

      {#if needsLogin}
        <p class="note">
          Snag checks for new ones only while it's running. Turn on starting at login so it always
          is.
          <button type="button" class="inline" onclick={() => snag.setStartAtLogin(true)}
            >Turn it on</button
          >
        </p>
      {/if}

      {#if !all.length}
        <div class="empty">
          <Icon name="subscribe" size={30} />
          <p class="lead">Nothing followed yet</p>
          <p>
            Paste the address of a YouTube channel or playlist, a podcast, or a playlist on another
            site Snag knows. What's in it now is yours to pick; what comes later downloads by
            itself.
          </p>
        </div>
      {:else}
        <div class="heading">
          <h2>Following</h2>
          {#if searchable}
            <label class="search">
              <Icon name="search" size={14} />
              <input bind:value={search} placeholder="Find" aria-label="Find a subscription" />
            </label>
          {/if}
        </div>
        <ul class="card" aria-label="Following">
          {#each shown as s (s.id)}
            {@const bad = !!s.error && !s.paused}
            <li
              class="row"
              class:paused={s.paused}
              aria-label={s.title}
              oncontextmenu={(e) => {
                e.preventDefault();
                snag.openSubscriptionMenu(s.id, e.clientX, e.clientY);
              }}
            >
              <span class="tile" class:bad><Icon name={glyph(s)} size={18} /></span>
              <div class="texts">
                <div class="title">
                  <button
                    type="button"
                    class="name"
                    title="Edit {s.title}"
                    onclick={() => snag.actOnSubscription(s.id, "edit")}>{s.title}</button
                  >
                  <span class="kind" title={s.source}>{subscriptionSource(s)}</span>
                </div>
                <div class="how">{how(s)}</div>
                <div class="status" class:on={s.checking} class:bad>
                  {bad ? `Can't read it lately: ${s.error}` : subscriptionStatus(s, now)}
                </div>
              </div>
              <div class="actions">
                <IconButton
                  icon="list"
                  tip="Show what it downloaded"
                  kind="ghost"
                  onclick={() => snag.actOnSubscription(s.id, "downloads")}
                />
                <IconButton
                  icon="retry"
                  tip="Check now"
                  kind="ghost"
                  busy={s.checking || !!snag.busy[`subscription:${s.id}:check`]}
                  disabled={s.checking}
                  onclick={() => snag.actOnSubscription(s.id, "check")}
                />
                <IconButton
                  icon={s.paused ? "play" : "pause"}
                  tip={s.paused ? "Resume: look for new ones again" : "Pause: stop looking for now"}
                  kind="ghost"
                  busy={!!snag.busy[`subscription:${s.id}:save`]}
                  onclick={() => snag.actOnSubscription(s.id, s.paused ? "resume" : "pause")}
                />
                <IconButton
                  icon="settings"
                  tip="Edit…"
                  kind="ghost"
                  onclick={() => snag.actOnSubscription(s.id, "edit")}
                />
                <IconButton
                  icon="more"
                  tip="More"
                  kind="ghost"
                  onclick={(e: MouseEvent) => menu(e, s.id)}
                />
              </div>
            </li>
          {:else}
            <li class="none">No subscription matches “{search}”</li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
</section>

<style>
  .page {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }

  header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 18px 22px 12px;
    border-bottom: 1px solid var(--border);
  }

  h1 {
    margin: 0;
    font-size: 20px;
    font-weight: 600;
  }

  .summary {
    margin-top: 2px;
    color: var(--text-faint);
    font-size: var(--text-sm);
  }

  .spacer {
    flex: 1;
  }

  .scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }

  .column {
    max-width: 760px;
    margin: 0 auto;
    padding: 20px 28px 40px;
  }

  .follow {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 14px;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
  }

  .field {
    box-sizing: border-box;
    flex: 1;
    min-width: 0;
    height: 36px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    outline: 0;
    background: var(--field);
    color: var(--text);
    font-size: var(--text-base);
    transition: border-color var(--fast);
  }

  .field:focus {
    border-color: var(--accent-strong);
  }

  .hint {
    margin: 10px 2px 0;
    color: var(--text-faint);
    font-size: var(--text-sm);
    line-height: 1.45;
  }

  .hint b {
    color: var(--text-dim);
    font-weight: 600;
  }

  .note {
    margin: 14px 2px 0;
    color: var(--warn);
    font-size: var(--text-sm);
    line-height: 1.45;
  }

  .inline {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
    cursor: pointer;
  }

  .heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin: 26px 0 10px;
  }

  h2 {
    margin: 0;
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.6px;
    text-transform: uppercase;
  }

  .search {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 28px;
    padding: 0 10px;
    border: 1px solid var(--border);
    border-radius: 14px;
    background: var(--field);
    color: var(--text-faint);
  }

  .search input {
    width: 160px;
    border: 0;
    outline: 0;
    background: none;
    color: var(--text);
    font-size: var(--text-sm);
  }

  .card {
    margin: 0;
    padding: 0 6px 0 16px;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
    list-style: none;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 12px 0;
  }

  .row + .row {
    border-top: 1px solid var(--border);
  }

  .tile {
    display: grid;
    place-items: center;
    flex: none;
    width: 36px;
    height: 36px;
    border-radius: var(--radius);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .tile.bad {
    background: color-mix(in srgb, var(--bad) 16%, transparent);
    color: var(--bad);
  }

  .row.paused .tile {
    background: var(--raised);
    color: var(--text-faint);
  }

  .texts {
    flex: 1;
    min-width: 0;
  }

  .title {
    display: flex;
    align-items: baseline;
    gap: 10px;
    min-width: 0;
  }

  .name {
    overflow: hidden;
    padding: 0;
    border: 0;
    background: none;
    color: var(--text);
    font: inherit;
    font-size: var(--text-md);
    font-weight: 600;
    text-align: left;
    text-overflow: ellipsis;
    white-space: nowrap;
    cursor: pointer;
  }

  .name:hover {
    color: var(--accent);
  }

  .kind {
    flex: none;
    color: var(--text-faint);
    font-size: var(--text-xs);
    white-space: pre;
  }

  .how,
  .status {
    overflow: hidden;
    margin-top: 3px;
    color: var(--text-dim);
    font-size: var(--text-sm);
    text-overflow: ellipsis;
    white-space: pre;
  }

  .status {
    color: var(--text-faint);
  }

  .status.on {
    color: var(--accent);
  }

  .status.bad {
    color: var(--bad);
  }

  .actions {
    display: flex;
    flex: none;
    gap: 2px;
  }

  .none {
    padding: 16px 0;
    color: var(--text-faint);
    font-size: var(--text-sm);
  }

  .empty {
    display: grid;
    justify-items: center;
    gap: 6px;
    margin-top: 48px;
    padding: 0 40px;
    color: var(--text-faint);
    text-align: center;
  }

  .empty .lead {
    margin: 8px 0 0;
    color: var(--text);
    font-size: var(--text-md);
    font-weight: 600;
  }

  .empty p {
    max-width: 440px;
    margin: 0;
    font-size: var(--text-sm);
    line-height: 1.5;
  }
</style>
