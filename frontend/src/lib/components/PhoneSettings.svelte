<script lang="ts">
  import { onMount } from "svelte";
  import IconButton from "./IconButton.svelte";
  import SettingRow from "./SettingRow.svelte";
  import Toggle from "./Toggle.svelte";
  import { ago } from "../format";
  import { useSnag } from "../state/context";
  import type { Pairing, Phone } from "../api/client";

  const snag = useSnag();
  let phone = $state<Phone | null>(null);
  let pairing = $state<Pairing | null>(null);
  const busy = (key: string) => !!snag.busy[`global:phone:${key}`];

  function show(p: Phone) {
    const before = phone?.devices.length ?? 0;
    phone = p;
    if (pairing && p.devices.length > before) {
      pairing = null; // the code is used up
      snag.showToast(`Paired ${p.devices.at(-1)?.name ?? "a phone"}`, "good");
    }
    if (!p.address) pairing = null;
  }

  onMount(() => {
    void snag.api.phone().then(show, () => {});
    const stop = snag.api.onPhone(show);
    return () => {
      stop();
      if (pairing) void snag.api.stopPairing();
    };
  });

  $effect(() => {
    if (!pairing) return;
    const timer = setTimeout(() => (pairing = null), Math.max(0, pairing.until - Date.now()));
    return () => clearTimeout(timer);
  });

  async function turn(on: boolean) {
    if (!on) pairing = null;
    const ok = await snag.request("global:phone:on", async () => show(await snag.api.setPhone(on)));
    if (!ok) phone = await snag.api.phone().catch(() => phone);
  }

  async function pair() {
    await snag.request("global:phone:pair", async () => (pairing = await snag.api.pairPhone()));
  }

  async function closePairing() {
    pairing = null;
    await snag.request("", () => snag.api.stopPairing());
  }

  async function remove(id: string, name: string) {
    if (
      await snag.request(`global:phone:remove-${id}`, async () =>
        show(await snag.api.removePhone(id)),
      )
    )
      snag.showToast(`Removed ${name}: it can't reach Snag any more`, "good");
  }

  const shown = (address: string) => address.replace(/^https?:\/\//, "");
</script>

{#if phone}
  <h2>Phone</h2>
  <div class="card">
    <SettingRow
      title="Send links from my phone"
      detail="Over this computer's home network. Only for home and trusted networks: on shared Wi-Fi, others could see what's sent"
    >
      <Toggle
        checked={phone.on}
        busy={busy("on")}
        label="Send links from my phone"
        onchange={(v) => turn(v)}
      />
    </SettingRow>

    {#if phone.on && phone.address}
      <SettingRow title="Pair a phone" detail="Scan a QR code with the phone's camera">
        <IconButton
          icon="link"
          label="Pair a phone…"
          busy={busy("pair")}
          disabled={!!pairing}
          onclick={pair}
        />
      </SettingRow>
      {#if pairing}
        <div class="pairing" role="group" aria-label="Pairing QR code">
          <svg
            viewBox="-4 -4 {pairing.size + 8} {pairing.size + 8}"
            width="200"
            height="200"
            shape-rendering="crispEdges"
            role="img"
            aria-label="QR code"
          >
            <rect x="-4" y="-4" width={pairing.size + 8} height={pairing.size + 8} fill="#fff" />
            <path d={pairing.path} fill="#000" />
          </svg>
          <div class="steps">
            <p>Scan it with the phone's camera and open the page it finds.</p>
            <p class="faint">
              It works once, for one phone, for 10 minutes. The phone has to be on the same Wi-Fi.
            </p>
            <p class="faint">
              Or type <code>{shown(pairing.link)}</code> into the phone's browser.
            </p>
            <p class="faint">
              If this computer asks whether Snag may accept network connections, allow it (on
              Windows, for private networks only).
            </p>
            <IconButton icon="x" label="Close" kind="ghost" onclick={closePairing} />
          </div>
        </div>
      {/if}
    {:else if phone.on}
      <div class="note" role="note">
        Not listening: this computer isn't on the network sending from a phone was turned on in ({phone.network}).
        To use this network, turn it off and on again.
      </div>
    {/if}

    {#each phone.devices as d (d.id)}
      <SettingRow
        title={d.name}
        detail="Paired {ago(d.added)}{d.seen > d.added ? `, last used ${ago(d.seen)}` : ''}"
      >
        <IconButton
          icon="trash"
          label="Remove"
          kind="ghost"
          busy={busy(`remove-${d.id}`)}
          onclick={() => remove(d.id, d.name)}
        />
      </SettingRow>
    {/each}
  </div>
{/if}

<style>
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

  .pairing {
    display: flex;
    align-items: flex-start;
    gap: 18px;
    margin: 4px 0 14px;
  }

  svg {
    flex: none;
    border-radius: var(--radius);
  }

  .steps {
    color: var(--text-dim);
    font-size: var(--text-sm);
    line-height: 1.45;
  }

  .steps p {
    margin: 0 0 8px;
  }

  .faint {
    color: var(--text-faint);
  }

  code {
    font-family: ui-monospace, "JetBrains Mono", "Cascadia Mono", Consolas, monospace;
    font-size: 12px;
    color: var(--text);
    user-select: text;
  }

  .note {
    margin: -2px 0 12px;
    padding: 10px 12px;
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--warn) 10%, transparent);
    color: var(--text-dim);
    font-size: var(--text-sm);
    line-height: 1.45;
  }
</style>
