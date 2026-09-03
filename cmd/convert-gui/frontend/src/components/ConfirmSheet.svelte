<script lang="ts">
  let {
    title,
    body,
    confirmLabel,
    cancelLabel,
    danger = true,
    onconfirm,
    oncancel,
  }: {
    title: string;
    body: string;
    confirmLabel: string;
    cancelLabel: string;
    danger?: boolean;
    onconfirm: () => void;
    oncancel: () => void;
  } = $props();
</script>

<div class="sheet-backdrop">
  <div class="sheet" role="dialog" aria-label={title}>
    <div class="sheet-handle"></div>
    <div class="sheet-title">{title}</div>
    <div class="sheet-body">{body}</div>
    <div class="sheet-actions">
      <button class="btn" onclick={oncancel}>{cancelLabel}</button>
      <button class="btn {danger ? 'danger' : 'primary'}" onclick={onconfirm}>{confirmLabel}</button>
    </div>
  </div>
</div>
