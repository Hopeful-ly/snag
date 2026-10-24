<script lang="ts">
  import { slide } from "svelte/transition";
  import Icon from "./Icon.svelte";
  import IconButton from "./IconButton.svelte";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  let link = $state("");
  let field: HTMLInputElement | undefined = $state();
  const busy = $derived(!!snag.busy["global:add"]);
  const importing = $derived(!!snag.busy["global:import"]);
  const several = $derived((link.match(/https?:\/\/\S+/g)?.length ?? 0) > 1);

  $effect(() => {
    if (!snag.addOpen) return;
    link = "";
    void snag.pasteLink().then((pasted) => {
      if (pasted && !link) link = pasted;
      field?.focus();
      field?.select();
    });
  });

  async function submit() {
    if (link.trim() && (await snag.addLink(link))) link = "";
  }
</script>

{#if snag.addOpen}
  <div class="bar" transition:slide={{ duration: 200 }}>
    <label class="field">
      <Icon name="link" size={17} />
      <input
        bind:this={field}
        bind:value={link}
        disabled={busy}
        placeholder="Paste a link: a file, a video page, a stream (.m3u8), or several"
        aria-label="Link"
        onkeydown={(e) => {
          if (e.key === "Enter") void submit();
          if (e.key === "Escape") {
            snag.addOpen = false;
            e.stopPropagation();
          }
        }}
      />
      <IconButton
        icon="paste"
        tip="Paste from clipboard"
        kind="ghost"
        size={32}
        iconSize={16}
        disabled={busy}
        onclick={async () => {
          link = (await snag.api.clipboardText().catch(() => "")).trim();
          field?.focus();
        }}
      />
      <IconButton
        icon="list"
        tip="Import links from a file…"
        kind="ghost"
        size={32}
        iconSize={16}
        busy={importing}
        disabled={busy || importing}
        onclick={() => snag.importLinks()}
      />
      <IconButton
        icon="download"
        label={several ? "Choose…" : "Download"}
        tip={several ? "Choose which of these links to download (Enter)" : "Add it (Enter)"}
        kind="primary"
        size={32}
        iconSize={16}
        {busy}
        disabled={!link.trim()}
        onclick={submit}
      />
    </label>
  </div>
{/if}

<style>
  .bar {
    padding: 0 20px 10px;
  }

  .field {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 44px;
    padding: 0 6px 0 14px;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--field);
    color: var(--text-faint);
    transition: border-color var(--fast);
  }

  .field:focus-within {
    border-color: var(--accent-strong);
  }

  input {
    flex: 1;
    min-width: 0;
    margin-left: 4px;
    border: 0;
    outline: 0;
    background: none;
    font-size: var(--text-md);
  }

  input::placeholder {
    color: var(--text-faint);
  }
</style>
