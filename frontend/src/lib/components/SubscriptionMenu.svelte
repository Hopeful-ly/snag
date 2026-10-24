<script lang="ts">
  import Menu from "./Menu.svelte";
  import { subscriptionMenuFor } from "../state/actions";
  import { useSnag } from "../state/context";

  const snag = useSnag();
  const s = $derived(
    snag.subscriptionMenu
      ? snag.subscriptions.find((x) => x.id === snag.subscriptionMenu?.id)
      : undefined,
  );
</script>

{#if snag.subscriptionMenu && s}
  <Menu
    label="{s.title} actions"
    x={snag.subscriptionMenu.x}
    y={snag.subscriptionMenu.y}
    items={subscriptionMenuFor(s)}
    onclose={() => (snag.subscriptionMenu = null)}
    onpick={(item) => void snag.actOnSubscription(s.id, item.action)}
  />
{/if}
