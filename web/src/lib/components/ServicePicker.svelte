<script lang="ts">
	import { registry } from '$lib/services';

	type Props = {
		onPick: (serviceKey: string) => void;
		advancedHref: string;
	};
	let { onPick, advancedHref }: Props = $props();

	let query = $state('');

	const entries = $derived(
		Object.entries(registry)
			.filter(([, e]) => {
				const q = query.toLowerCase();
				if (!q) return true;
				return (
					e.meta.label.toLowerCase().includes(q) ||
					e.meta.description.toLowerCase().includes(q) ||
					e.meta.code.toLowerCase().includes(q)
				);
			})
			.sort((a, b) => a[1].meta.label.localeCompare(b[1].meta.label))
	);
</script>

<div class="space-y-4">
	<input
		type="text"
		bind:value={query}
		placeholder="Search services…"
		class="w-full rounded-md border border-[--color-border] bg-transparent px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
	/>
	<div class="grid gap-2 sm:grid-cols-2">
		{#each entries as [key, entry] (key)}
			<button
				type="button"
				onclick={() => onPick(key)}
				class="flex flex-col items-start gap-1 rounded-lg border border-[--color-border] bg-[--color-surface-elev] p-3 text-left transition-colors hover:border-[--color-text]"
			>
				<span class="text-sm font-medium text-[--color-text]">{entry.meta.label}</span>
				<span class="text-xs text-[--color-text-muted]">{entry.meta.description}</span>
			</button>
		{/each}
	</div>
	<a
		href={advancedHref}
		class="mt-2 block text-center text-xs text-[--color-text-muted] hover:text-[--color-text]"
	>
		Advanced: browse all 268 services →
	</a>
</div>
