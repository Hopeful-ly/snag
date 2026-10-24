<script lang="ts">
  let { size = 16 }: { size?: number } = $props();
</script>

<svg
  class="spinner"
  width={size}
  height={size}
  viewBox="0 0 24 24"
  aria-label="Working…"
  role="img"
>
  <path
    d="M12 3a9 9 0 1 1-9 9"
    fill="none"
    stroke="currentColor"
    stroke-width="2.6"
    stroke-linecap="round"
  />
</svg>

<style>
  .spinner {
    animation: turn 0.8s linear infinite;
  }

  @keyframes turn {
    to {
      transform: rotate(360deg);
    }
  }
</style>
