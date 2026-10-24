<script lang="ts">
  import { icons, type IconName } from "../icons";

  let {
    name,
    size = 18,
    stroke = 2,
  }: { name: IconName; size?: number; stroke?: number } = $props();
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width={stroke}
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  <path d={icons[name]} />
</svg>
