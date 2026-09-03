<script lang="ts">
  let {
    kind = "info",
    text,
    actionLabel = "",
    onaction = undefined,
    ondismiss = undefined,
  }: {
    kind?: "info" | "success" | "warn" | "danger";
    text: string;
    actionLabel?: string;
    onaction?: () => void;
    ondismiss?: () => void;
  } = $props();
</script>

<div class="banner {kind}">
  <span class="banner-text">{text}</span>
  {#if actionLabel && onaction}
    <button class="btn" onclick={onaction}>{actionLabel}</button>
  {/if}
  {#if ondismiss}
    <button class="banner-x" aria-label="Dismiss" onclick={ondismiss}>✕</button>
  {/if}
</div>
