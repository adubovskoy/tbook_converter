<script lang="ts">
  import { Events } from "@wailsio/runtime";
  import { PickBook } from "../lib/api";
  import { copy } from "../lib/copy";

  // Native OS drops arrive from Go: the window is created with EnableFileDrop
  // and relays WindowFilesDropped paths as a "files:dropped" event. Only
  // elements marked data-file-drop-target accept the drop.
  let { onpick }: { onpick: (path: string) => void } = $props();

  const accepted = /\.(epub|fb2|fb2\.zip|tbook)$/i;

  $effect(() => {
    return Events.On("files:dropped", (ev: { data: string[] | string[][] }) => {
      const paths = Array.isArray(ev.data) ? (ev.data.flat() as string[]) : [];
      const book = paths.find((p) => accepted.test(p));
      if (book) onpick(book);
    });
  });

  async function browse() {
    try {
      const p = await PickBook();
      if (p) onpick(p);
    } catch {
      // dialog dismissed
    }
  }
</script>

<button class="dropzone" data-file-drop-target onclick={browse}>
  <span class="dropzone-icon">📖</span>
  <span class="dropzone-title">{copy.convert.dropTitle}</span>
  <span class="dropzone-sub">{copy.convert.dropSub}</span>
</button>
