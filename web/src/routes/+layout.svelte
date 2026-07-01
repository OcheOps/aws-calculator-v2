<script lang="ts">
  import '../app.css';
  let { children } = $props();
</script>

<div class="mx-auto max-w-6xl px-6 py-10">
  <header class="mb-12 flex items-center justify-between">
    <a href="/" class="text-lg font-semibold tracking-tight">AWS Calculator</a>
    <nav class="text-sm text-neutral-400">
      <a href="/services" class="hover:text-neutral-100">Services</a>
    </nav>
  </header>
  {@render children()}
</div>
