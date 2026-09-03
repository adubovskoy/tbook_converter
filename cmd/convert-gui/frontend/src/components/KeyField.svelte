<script lang="ts">
  let {
    value = $bindable(""),
    label = "API key",
    placeholder = "",
  }: { value?: string; label?: string; placeholder?: string } = $props();

  let show = $state(false);
</script>

<label class="field">
  <span class="field-label">{label}</span>
  <span class="field-input">
    <!-- No bind: — Svelte forbids two-way binding with a dynamic input type. -->
    <input
      type={show ? "text" : "password"}
      {value}
      {placeholder}
      autocomplete="off"
      spellcheck="false"
      oninput={(e) => (value = e.currentTarget.value)}
    />
    <button class="btn" type="button" onclick={() => (show = !show)}>{show ? "Hide" : "Show"}</button>
  </span>
</label>
