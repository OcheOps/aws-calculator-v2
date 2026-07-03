<script lang="ts">
	import { currency } from '$lib/stores/currency.svelte';
	const options = ['USD', 'EUR', 'GBP', 'NGN'];
</script>

<select
	value={currency.value}
	onchange={(e) => currency.set((e.currentTarget as HTMLSelectElement).value)}
	class="rounded-md border border-[--color-border] bg-transparent px-2 py-1 text-xs text-[--color-text-muted] hover:text-[--color-text]"
	aria-label="Display currency"
>
	{#each options as c}
		<option value={c}>{c}</option>
	{/each}
</select>
