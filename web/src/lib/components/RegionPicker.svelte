<script lang="ts">
	import { humaniseRegion } from '$lib/utils/labels';
	import type { Region } from '$lib/api/types';

	type Props = {
		value?: string;
		regions: Region[];
		label?: string;
	};
	let { value = $bindable('us-east-1'), regions, label = 'Region' }: Props = $props();

	function continent(code: string): string {
		if (code.startsWith('us-') || code.startsWith('ca-')) return 'North America';
		if (code.startsWith('eu-')) return 'Europe';
		if (code.startsWith('ap-')) return 'Asia Pacific';
		if (code.startsWith('sa-')) return 'South America';
		if (code.startsWith('me-')) return 'Middle East';
		if (code.startsWith('af-')) return 'Africa';
		return 'Other';
	}

	const groups = $derived.by(() => {
		const buckets = new Map<string, Region[]>();
		for (const r of regions ?? []) {
			const c = continent(r.code);
			if (!buckets.has(c)) buckets.set(c, []);
			buckets.get(c)!.push(r);
		}
		return [...buckets.entries()].sort((a, b) => a[0].localeCompare(b[0]));
	});
</script>

<label class="block">
	<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>
	<select
		bind:value
		class="w-full rounded-md border border-[--color-border] bg-[--color-surface] px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
	>
		{#each groups as [continentName, rs] (continentName)}
			<optgroup label={continentName}>
				{#each rs as r (r.code)}
					<option value={r.code}>{humaniseRegion(r.code)}</option>
				{/each}
			</optgroup>
		{/each}
	</select>
</label>
