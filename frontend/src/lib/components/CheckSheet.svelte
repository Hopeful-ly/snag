<script lang="ts">
  import IconButton from "./IconButton.svelte";
  import Sheet from "./Sheet.svelte";
  import { checkText, looksLikeChecksum } from "../format";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const d = $derived(snag.checkFor ? snag.downloads.get(snag.checkFor) : undefined);
  const check = $derived(d?.check ?? null);

  let text = $state("");
  let pasted = false;

  const checking = $derived(!!d && (!!snag.busy[`${d.id}:check`] || check?.result === "checking"));
  const copying = $derived(!!d && !!snag.busy[`${d.id}:copyChecksum`]);
  const canCheck = $derived(
    !checking && (text.trim() !== "" ? looksLikeChecksum(text) : !!check?.want),
  );

  // The clipboard's checksum fills the box once, when the sheet opens.
  $effect(() => {
    if (!d || pasted) return;
    pasted = true;
    void snag.api.clipboardText().then(
      (clip) => {
        if (text === "" && looksLikeChecksum(clip)) text = clip.trim();
      },
      () => {},
    );
  });

  function close() {
    snag.checkFor = null;
  }

  async function run() {
    const target = d;
    if (!target || !canCheck) return;
    await snag.request(`${target.id}:check`, () => snag.api.check(target.id, text.trim()));
  }

  async function copy() {
    const target = d;
    if (!target) return;
    if (await snag.request(`${target.id}:copyChecksum`, () => snag.api.copyChecksum(target.id)))
      snag.showToast("SHA-256 copied", "good");
  }
</script>

{#if d}
  <Sheet title="Check checksum" icon="check" width={540} onclose={close}>
    <p class="lead">“{d.name}”</p>

    {#if check && check.result !== "looking"}
      <p
        class="state"
        class:good={check.result === "match"}
        class:bad={check.result === "mismatch" || check.result === "failed"}
        role="status"
      >
        {checkText(check, d.size)}
      </p>
      {#if check.result === "mismatch"}
        <p class="sums numbers">Expected {check.want}<br />The file's {check.got}</p>
      {/if}
    {:else}
      <p class="state" role="status">
        Snag found no checksum for this file. Paste one to check it.
      </p>
    {/if}

    <label class="caption" for="check-text">
      {check?.want ? "Or check against another checksum" : "Checksum"}
    </label>
    <input
      id="check-text"
      class="field numbers"
      placeholder="MD5, SHA-1, SHA-256 or SHA-512"
      spellcheck="false"
      bind:value={text}
      onkeydown={(e) => e.key === "Enter" && void run()}
    />
    {#if text.trim() !== "" && !looksLikeChecksum(text)}
      <p class="hint bad">That isn't a checksum Snag knows.</p>
    {/if}

    <div class="buttons">
      <IconButton
        icon="link"
        label="Copy SHA-256"
        tip="Copy the file's own SHA-256, to compare it yourself"
        busy={copying}
        disabled={copying}
        onclick={copy}
      />
      <span class="spacer"></span>
      {#if check?.result === "mismatch"}
        <IconButton
          icon="retry"
          label="Download again"
          busy={!!snag.busy[`${d.id}:redownload`]}
          onclick={() => {
            const id = d.id;
            close();
            void snag.act(id, "redownload");
          }}
        />
      {/if}
      <IconButton
        icon="check"
        label="Check"
        kind="primary"
        busy={checking}
        disabled={!canCheck}
        onclick={run}
      />
    </div>
  </Sheet>
{/if}

<style>
  .lead {
    margin: 0 0 10px;
    overflow: hidden;
    font-weight: 500;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .state {
    margin: 0 0 14px;
    color: var(--text-dim);
  }

  .state.good {
    color: var(--good);
  }

  .state.bad,
  .hint.bad {
    color: var(--bad);
  }

  .sums {
    margin: -6px 0 14px;
    color: var(--text-faint);
    font-size: var(--text-sm);
    word-break: break-all;
  }

  .caption {
    display: block;
    margin-bottom: 6px;
    color: var(--text-dim);
    font-size: var(--text-sm);
  }

  .field {
    box-sizing: border-box;
    width: 100%;
    height: 38px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 9px;
    outline: 0;
    background: var(--field);
    color: var(--text);
    font-size: var(--text-md);
    color-scheme: dark;
  }

  .field:focus {
    border-color: var(--accent-strong);
  }

  .hint {
    margin: 6px 0 0;
    font-size: var(--text-sm);
  }

  .buttons {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 18px;
  }

  .spacer {
    flex: 1;
  }
</style>
