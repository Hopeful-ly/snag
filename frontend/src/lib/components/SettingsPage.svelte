<script lang="ts">
  import Icon from "./Icon.svelte";
  import IconButton from "./IconButton.svelte";
  import SettingRow from "./SettingRow.svelte";
  import Stepper from "./Stepper.svelte";
  import Toggle from "./Toggle.svelte";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const s = $derived(snag.settings);
  let picking = $state(false);
  let version = $state("");
  void snag.api.version().then((v) => (version = v));
  let author = $state({ name: "Roham1299", page: "https://github.com/Roham1299" });
  void snag.api.author().then(
    (a) => (author = a),
    () => {},
  );
  let update = $state<{ version: string } | null>(null);
  void snag.api.update().then(
    (u) => (update = u),
    () => {},
  );
  const busy = (key: string) => !!snag.busy[`global:settings:${key}`];
  void snag.refreshStartAtLogin(); // the system's settings may have changed it

  function close() {
    snag.settingsOpen = false;
  }

  async function changeFolder() {
    if (!s) return;
    picking = true;
    try {
      const picked = await snag.api.chooseFolder(s.downloadDir);
      if (picked) await snag.setSetting("downloadDir", picked);
    } catch {
      // Closing the dialog is no error.
    } finally {
      picking = false;
    }
  }
</script>

<svelte:window
  onkeydown={(e) => {
    const typing = e.target instanceof HTMLInputElement;
    if (e.key === "Escape" && !typing && !snag.connectOpen) {
      close();
      e.preventDefault();
    }
  }}
/>

<section class="page" aria-label="Settings">
  <header>
    <IconButton icon="back" tip="Back to downloads (Esc)" kind="ghost" onclick={close} />
    <h1>Settings</h1>
  </header>

  {#if s}
    <div class="scroll">
      <div class="column">
        <h2>Downloads</h2>
        <div class="card">
          <SettingRow title="Save downloads to" detail={snag.short(s.downloadDir)}>
            <IconButton
              icon="folder"
              label="Change…"
              busy={picking || busy("downloadDir")}
              onclick={changeFolder}
            />
          </SettingRow>
          <SettingRow
            title="Ask where to save each download"
            detail="Shows the Save dialog for every new download"
          >
            <Toggle
              checked={s.ask}
              busy={busy("ask")}
              label="Ask where to save each download"
              onchange={(v) => snag.setSetting("ask", v)}
            />
          </SettingRow>
          <SettingRow
            title="Connections per download"
            detail="More can be faster on fast lines; some servers limit them"
          >
            <Stepper
              value={s.connections}
              min={1}
              max={32}
              busy={busy("connections")}
              label="Connections per download"
              onchange={(v) => snag.setSetting("connections", v)}
            />
          </SettingRow>
          <SettingRow title="Downloads at once" detail="The rest wait their turn">
            <Stepper
              value={s.maxActive}
              min={1}
              max={10}
              busy={busy("maxActive")}
              label="Downloads at once"
              onchange={(v) => snag.setSetting("maxActive", v)}
            />
          </SettingRow>
          <SettingRow title="Notify when a download finishes">
            <Toggle
              checked={s.notify}
              busy={busy("notify")}
              label="Notify when a download finishes"
              onchange={(v) => snag.setSetting("notify", v)}
            />
          </SettingRow>
        </div>

        <h2>Naming playlist files</h2>
        <div class="card">
          {#each [{ key: "songs", label: "Songs" }, { key: "videos", label: "Videos" }, { key: "episodes", label: "Episodes" }] as const as n (n.key)}
            <label class="named">
              <span>{n.label}</span>
              <input
                class="field"
                aria-label={n.label}
                spellcheck="false"
                value={s.naming?.[n.key] ?? ""}
                disabled={busy("naming")}
                onchange={(e) =>
                  snag.setSetting("naming", { ...s.naming, [n.key]: e.currentTarget.value.trim() })}
              />
            </label>
          {/each}
          <p class="hint">
            Empty fields drop out, so {"{artist}"} - {"{title}"} is just the title when there is no artist.
          </p>
        </div>

        <h2>Snag</h2>
        <div class="card">
          {#if snag.startAtLogin?.available}
            <SettingRow
              title="Start Snag when I log in"
              detail="In the tray, so queues and subscriptions run on time and the browser never waits"
            >
              <Toggle
                checked={snag.startAtLogin.on}
                busy={busy("startAtLogin")}
                label="Start Snag when I log in"
                onchange={(v) => snag.setStartAtLogin(v)}
              />
            </SettingRow>
            {#if snag.startAtLogin.desktop && snag.startAtLogin.command}
              <!-- Hyprland, Sway and the like start only what their own config lists. -->
              <div class="manual" role="note">
                <p>
                  {snag.startAtLogin.desktop} doesn't start what other desktops do at login, so this setting
                  alone won't start Snag there. Add this to {snag.startAtLogin.desktop}'s own
                  startup programs (<code>exec-once</code> in its config, for example):
                </p>
                <div class="command">
                  <code>{snag.startAtLogin.command}</code>
                  <IconButton
                    icon="paste"
                    tip="Copy the command"
                    kind="ghost"
                    size={28}
                    iconSize={14}
                    onclick={async () => {
                      const command = snag.startAtLogin?.command ?? "";
                      if (await snag.request("", () => snag.api.copyText(command)))
                        snag.showToast("Command copied", "good");
                    }}
                  />
                </div>
              </div>
            {/if}
          {/if}
          <SettingRow
            title="Browser extension"
            detail={snag.setup?.connected
              ? "Connected"
              : snag.setup?.connectedBefore
                ? "Not connected right now"
                : "Not connected yet"}
          >
            <IconButton icon="link" label="Connect…" onclick={() => (snag.connectOpen = true)} />
          </SettingRow>
          <SettingRow
            title="TMDB key"
            detail="Optional. Lists a show's episodes on streaming sites whose pages don't; free at themoviedb.org"
          >
            <input
              class="field key"
              type="password"
              aria-label="TMDB key"
              placeholder="None"
              autocomplete="off"
              spellcheck="false"
              value={s.tmdbKey ?? ""}
              disabled={busy("tmdbKey")}
              onchange={(e) => snag.setSetting("tmdbKey", e.currentTarget.value.trim())}
            />
          </SettingRow>
        </div>

        <h2>About</h2>
        <div class="card about">
          <span class="logo"><Icon name="download" size={18} stroke={2.4} /></span>
          <div>
            <div class="name">Snag <span class="version numbers">{version}</span></div>
            {#if update}
              <div class="credits">
                Snag <span class="numbers">{update.version}</span> is out:
                <button type="button" class="link" onclick={() => snag.api.openUpdatePage()}
                  >get it</button
                >
              </div>
            {/if}
            <div class="credits">
              Made by
              <button
                type="button"
                class="link"
                title={author.page}
                onclick={() => snag.api.openAuthorPage()}>{author.name}</button
              >
            </div>
            <div class="credits faint">
              Free software under the GPL 3.0 or later, with no warranty. Source:
              github.com/Roham1299/snag.
            </div>
            <div class="credits faint">
              Uses yt-dlp (public domain) and FFmpeg (LGPL 2.1, ffmpeg.org).
            </div>
          </div>
        </div>
      </div>
    </div>
  {/if}
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

  .scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }

  .column {
    max-width: 760px;
    margin: 0 auto;
    padding: 8px 28px 40px;
  }

  h2 {
    margin: 26px 0 10px;
    color: var(--text-faint);
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.6px;
    text-transform: uppercase;
  }

  .card {
    padding: 4px 18px;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
  }

  .field {
    box-sizing: border-box;
    height: 34px;
    padding: 0 11px;
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

  .key {
    width: 220px;
  }

  .named {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 10px 0 0;
  }

  .named span {
    width: 80px;
    flex: none;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .named .field {
    flex: 1;
    min-width: 0;
  }

  .hint {
    margin: 10px 0 12px;
    color: var(--text-faint);
    font-size: var(--text-sm);
    line-height: 1.4;
  }

  .about {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 14px 18px;
  }

  .logo {
    display: grid;
    place-items: center;
    flex: none;
    width: 36px;
    height: 36px;
    border-radius: var(--radius);
    background: var(--accent-strong);
    color: white;
  }

  .name {
    font-weight: 600;
  }

  .version {
    margin-left: 6px;
    color: var(--text-faint);
    font-weight: 400;
  }

  .credits {
    margin-top: 3px;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .credits.faint {
    color: var(--text-faint);
  }

  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    cursor: pointer;
  }

  .link:hover {
    text-decoration: underline;
  }

  .manual {
    margin: -2px 0 12px;
    padding: 10px 12px;
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--warn) 10%, transparent);
    color: var(--text-dim);
    font-size: var(--text-sm);
    line-height: 1.45;
  }

  .manual p {
    margin: 0 0 8px;
  }

  .command {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 4px 4px 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--field);
  }

  .command code {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    color: var(--text);
  }

  code {
    font-family: ui-monospace, "JetBrains Mono", "Cascadia Mono", Consolas, monospace;
    font-size: 12px;
  }
</style>
