<script lang="ts">
	import { theme } from '$lib/stores/theme.svelte';
</script>

<button
	onclick={() => theme.toggle()}
	class="rounded-md border border-[--color-border] px-2 py-1 text-xs text-[--color-text-muted] hover:text-[--color-text]"
	aria-label="Toggle theme"
>
	{theme.value === 'dark' ? 'Light' : 'Dark'}
</button>
