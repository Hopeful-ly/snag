<script lang="ts">
  import Icon from "./Icon.svelte";
  import IconButton from "./IconButton.svelte";
  import Sheet from "./Sheet.svelte";
  import { useSnag } from "../state/context";
  import type { Browser } from "../api/client";

  // Browsers only load extensions from outside their stores from a folder, so Snag keeps one ready.
  const snag = useSnag();
  const extensionsPage = "chrome://extensions";
  const setup = $derived(snag.setup);
  const waiting = $derived(
    "Waiting for the extension…" +
      (setup?.connectedBefore
        ? " It connected before; if it was just updated, restart the browser or press the extension's reload button."
        : ""),
  );
  let registering = $state(false);
  let browsers = $state<Browser[]>([]);
  snag.api.browsers().then(
    (list) => (browsers = list),
    () => {},
  );

  async function openPage(b: Browser) {
    await snag.request(`open:${b.id}`, () => snag.api.openExtensionsPage(b.id));
  }

  async function copy(text: string, what: string) {
    if (await snag.request("", () => snag.api.copyText(text)))
      snag.showToast(`Copied ${what}`, "good");
  }

  async function registerAgain() {
    registering = true;
    await snag.request("", async () => {
      const browsers = await snag.api.registerBrowsers();
      snag.showToast(
        browsers.length ? `Set up for ${browsers.join(", ")}` : "No supported browser found",
        browsers.length ? "good" : "info",
      );
    });
    registering = false;
  }
</script>

<Sheet
  title="Connect your browser"
  icon="link"
  width={600}
  onclose={() => (snag.connectOpen = false)}
>
  <p class="intro">Snag's Download button lives in your browser. Add Snag's extension once:</p>

  <ol>
    <li>
      <div>Open your browser's extensions page:</div>
      {#if browsers.length}
        <div class="browsers">
          {#each browsers as b (b.id)}
            <IconButton
              icon="open"
              label={b.name}
              tip={`Open ${b.name} on its extensions page`}
              size={34}
              busy={snag.busy[`open:${b.id}`]}
              onclick={() => openPage(b)}
            />
          {/each}
        </div>
        <div class="note">Or paste this in your browser's address bar:</div>
      {:else}
        <div class="note">Paste this in your browser's address bar:</div>
      {/if}
      <div class="value">
        <code>{extensionsPage}</code>
        <IconButton
          icon="paste"
          tip="Copy"
          size={30}
          iconSize={15}
          onclick={() => copy(extensionsPage, "the address")}
        />
      </div>
      <div class="note">It works in Chrome, Edge, Brave, Vivaldi and Opera.</div>
    </li>
    <li>Turn on <strong>Developer mode</strong>, at the top of that page.</li>
    <li>
      <div>Drag Snag's extension folder onto that page:</div>
      <div class="browsers">
        <IconButton
          icon="folder"
          label="Show the folder"
          tip="Show the folder, to drag it onto the extensions page"
          size={34}
          onclick={() => snag.request("", () => snag.api.showExtensionFolder())}
        />
      </div>
      <div class="note">Or click <strong>Load unpacked</strong> there and choose it:</div>
      {#if setup}
        <div class="value">
          <code title={setup.extensionDir}>{snag.short(setup.extensionDir)}</code>
          <IconButton
            icon="paste"
            tip="Copy the folder's path"
            size={30}
            iconSize={15}
            onclick={() => copy(setup.extensionDir, "the folder's path")}
          />
        </div>
      {/if}
      <div class="note">
        Your browser may remind you about extensions in Developer mode; that's expected for ones
        from outside its store. Snag keeps the folder up to date.
      </div>
    </li>
  </ol>

  <p class="firefox">Firefox: the Snag add-on for Firefox is coming soon.</p>
  {#if snag.startAtLogin?.on}
    <p class="firefox">
      Snag starts with your computer, in the tray, so it's ready when the browser needs it. Turn
      that off in Settings.
    </p>
  {/if}

  <div class="status" class:connected={setup?.connected} role="status">
    {#if setup?.connected}
      <Icon name="done" size={18} />
      <span>Connected: the extension is talking to Snag.</span>
    {:else}
      <Icon name="clock" size={18} />
      <span>{waiting}</span>
    {/if}
  </div>

  <div class="buttons">
    <IconButton
      icon="retry"
      label="Set up browsers again"
      kind="ghost"
      size={36}
      tip="Point your browsers at this copy of Snag"
      busy={registering}
      onclick={registerAgain}
    />
    <IconButton
      icon="check"
      label="Done"
      kind="primary"
      size={36}
      onclick={() => (snag.connectOpen = false)}
    />
  </div>
</Sheet>

<style>
  .intro {
    margin: 0 0 12px;
    color: var(--text-dim);
  }

  ol {
    display: flex;
    flex-direction: column;
    gap: 14px;
    margin: 0;
    padding-left: 22px;
  }

  li::marker {
    color: var(--accent);
    font-weight: 600;
  }

  .browsers {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 8px;
  }

  .value {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 6px;
  }

  code {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    padding: 7px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--field);
    font:
      var(--text-sm) / 1.3 ui-monospace,
      "JetBrains Mono",
      monospace;
    white-space: nowrap;
    text-overflow: ellipsis;
    user-select: text;
  }

  .note {
    margin-top: 5px;
    color: var(--text-faint);
    font-size: var(--text-sm);
  }

  .firefox {
    margin: 16px 0 0;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .status {
    display: flex;
    align-items: center;
    gap: 9px;
    margin-top: 16px;
    padding: 10px 12px;
    border-radius: var(--radius-lg);
    background: var(--raised);
    color: var(--text-dim);
  }

  .status.connected {
    background: var(--good-soft);
    color: var(--good);
  }

  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }
</style>
