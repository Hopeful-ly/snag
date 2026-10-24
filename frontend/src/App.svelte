<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import AddBar from "./lib/components/AddBar.svelte";
  import ConnectSheet from "./lib/components/ConnectSheet.svelte";
  import ContextMenu from "./lib/components/ContextMenu.svelte";
  import QueueMenu from "./lib/components/QueueMenu.svelte";
  import CountdownSheet from "./lib/components/CountdownSheet.svelte";
  import DownloadList from "./lib/components/DownloadList.svelte";
  import SettingsPage from "./lib/components/SettingsPage.svelte";
  import SubscriptionsPage from "./lib/components/SubscriptionsPage.svelte";
  import QueueSheet from "./lib/components/QueueSheet.svelte";
  import WhereSheet from "./lib/components/WhereSheet.svelte";
  import CheckSheet from "./lib/components/CheckSheet.svelte";
  import DownloadDetails from "./lib/components/DownloadDetails.svelte";
  import SubscriptionMenu from "./lib/components/SubscriptionMenu.svelte";
  import SubscriptionSheet from "./lib/components/SubscriptionSheet.svelte";
  import QueueStrip from "./lib/components/QueueStrip.svelte";
  import Sidebar from "./lib/components/Sidebar.svelte";
  import Toast from "./lib/components/Toast.svelte";
  import Toolbar from "./lib/components/Toolbar.svelte";
  import Tooltip from "./lib/components/Tooltip.svelte";
  import type { Api } from "./lib/api/client";
  import { doubleClickOf } from "./lib/state/actions";
  import { provideSnag } from "./lib/state/context";
  import { Snag } from "./lib/state/snag.svelte";

  let { api }: { api: Api } = $props();

  // The api never changes over the window's life.
  const snag = new Snag(untrack(() => api));
  provideSnag(snag);
  void snag.start();
  onDestroy(() => snag.stop());

  let search: HTMLInputElement | undefined = $state();
  const pageOpen = $derived(snag.settingsOpen || snag.subscriptionsOpen);

  // In a text field only the Ctrl shortcuts apply.
  function onkeydown(e: KeyboardEvent) {
    const typing = e.target instanceof HTMLInputElement;
    const sheetOpen =
      snag.settingsOpen ||
      snag.subscriptionsOpen ||
      snag.connectOpen ||
      snag.menu ||
      snag.queueMenu ||
      snag.queueSheet ||
      snag.whereFor ||
      snag.checkFor ||
      snag.subscriptionSheet ||
      snag.subscriptionMenu ||
      snag.countdown;
    if (e.ctrlKey && ["n", "l", "v"].includes(e.key) && !typing && !sheetOpen) {
      snag.addOpen = true;
      e.preventDefault();
    } else if (e.ctrlKey && e.key === "f" && !sheetOpen) {
      search?.focus();
      e.preventDefault();
    } else if (typing || sheetOpen) {
      return;
    } else if (e.key === "Escape") {
      if (snag.detailsFor) snag.detailsFor = null;
      else if (snag.addOpen) snag.addOpen = false;
      else if (snag.search) snag.search = "";
      else snag.selected = "";
    } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      move(e.key === "ArrowDown" ? 1 : -1);
      e.preventDefault();
    } else {
      const d = snag.downloads.get(snag.selected);
      if (!d) return;
      if (e.key === "Delete") void snag.act(d.id, "remove");
      else if (e.key === "Enter") void snag.act(d.id, doubleClickOf(d));
      else if (e.key === " ") {
        if (d.status === "downloading" || d.status === "queued") void snag.act(d.id, "pause");
        else if (d.where) void snag.act(d.id, "where");
        else if (d.status === "paused" || d.status === "error") void snag.act(d.id, "resume");
        e.preventDefault();
      }
    }
  }

  function move(step: number) {
    const list = snag.visible;
    if (!list.length) return;
    const at = list.findIndex((d) => d.id === snag.selected);
    const next = list[Math.max(0, Math.min(list.length - 1, at + step))];
    if (!next) return;
    snag.selected = next.id;
    if (snag.detailsFor) snag.detailsFor = next.id;
    document
      .querySelector(`[role=option][aria-selected=true]`)
      ?.scrollIntoView({ block: "nearest" });
  }
</script>

<svelte:window {onkeydown} />

<div class="shell" class:with-details={!!snag.detailsFor && !pageOpen}>
  <Sidebar />
  <main>
    {#if snag.settingsOpen}
      <SettingsPage />
    {:else if snag.subscriptionsOpen}
      <SubscriptionsPage />
    {:else}
      <Toolbar bind:search />
      <AddBar />
      <QueueStrip />
      <DownloadList />
    {/if}
  </main>
  {#if !pageOpen}<DownloadDetails />{/if}
</div>

<ContextMenu />
<QueueMenu />
{#if snag.connectOpen}<ConnectSheet />{/if}
{#if snag.queueSheet}<QueueSheet />{/if}
{#if snag.whereFor}<WhereSheet />{/if}
{#if snag.checkFor}<CheckSheet />{/if}
{#if snag.subscriptionSheet}<SubscriptionSheet />{/if}
<SubscriptionMenu />
<CountdownSheet />
<Toast />
<Tooltip />

<style>
  .shell {
    display: grid;
    grid-template-columns: 220px 1fr;
    height: 100vh;
  }

  .shell.with-details {
    grid-template-columns: 220px 1fr 340px;
  }

  main {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
</style>
