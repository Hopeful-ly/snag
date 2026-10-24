<script lang="ts">
  import Menu from "./Menu.svelte";
  import { queueMenuFor } from "../state/actions";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const place = $derived(
    snag.queueMenu ? snag.queues.findIndex((q) => q.id === snag.queueMenu?.id) : -1,
  );
</script>

{#if snag.queueMenu && place >= 0}
  {@const q = snag.queues[place]!}
  <Menu
    label="{q.name} actions"
    x={snag.queueMenu.x}
    y={snag.queueMenu.y}
    items={queueMenuFor(q, place, snag.queues.length)}
    onclose={() => (snag.queueMenu = null)}
    onpick={(item) => void snag.actOnQueue(q.id, item.action)}
  />
{/if}
