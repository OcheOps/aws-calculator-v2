<script lang="ts" generics="T">
	type Preset = { label: string; value: T };
	type Props = {
		value?: T | undefined;
		presets: Preset[];
		label?: string;
		custom?: { label: string; onSelect: () => void };
	};
	let {
		value = $bindable(undefined as T | undefined),
		presets,
		label = '',
		custom
	}: Props = $props();
</script>

<div>
	{#if label}<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>{/if}
	<div class="flex flex-wrap gap-2">
		{#each presets as p (p.label)}
			<button
				type="button"
				onclick={() => (value = p.value)}
				class="rounded-md border px-3 py-1.5 text-xs {value === p.value
					? 'border-[--color-text] bg-[--color-surface-elev] text-[--color-text]'
					: 'border-[--color-border] text-[--color-text-muted] hover:text-[--color-text]'}"
			>
				{p.label}
			</button>
		{/each}
		{#if custom}
			<button
				type="button"
				onclick={() => custom.onSelect()}
				class="rounded-md border border-dashed border-[--color-border] px-3 py-1.5 text-xs text-[--color-text-muted] hover:text-[--color-text]"
			>
				{custom.label}
			</button>
		{/if}
	</div>
</div>
