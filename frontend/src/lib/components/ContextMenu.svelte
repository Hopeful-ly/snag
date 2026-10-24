<script lang="ts">
  import Menu from "./Menu.svelte";
  import { menuFor } from "../state/actions";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const target = $derived(snag.menu ? snag.downloads.get(snag.menu.id) : undefined);
</script>

{#if snag.menu && target}
  {@const id = target.id}
  <Menu
    label="Download actions"
    x={snag.menu.x}
    y={snag.menu.y}
    items={menuFor(target, snag.players, snag.queues)}
    onclose={() => (snag.menu = null)}
    onpick={(item) => void snag.act(id, item.action, item.player ?? item.queue)}
  />
{/if}
