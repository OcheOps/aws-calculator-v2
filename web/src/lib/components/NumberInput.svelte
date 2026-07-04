<script lang="ts">
	type Props = {
		value?: number;
		label?: string;
		unit?: string;
		min?: number;
		max?: number;
		step?: number;
		placeholder?: string;
	};
	let {
		value = $bindable(0),
		label = '',
		unit = '',
		min,
		max,
		step = 1,
		placeholder = ''
	}: Props = $props();
</script>

<label class="block">
	{#if label}<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>{/if}
	<div
		class="flex items-center gap-2 rounded-md border border-[--color-border] bg-transparent px-3 py-2 text-sm text-[--color-text] focus-within:border-[--color-text]"
	>
		<input
			type="number"
			bind:value
			{min}
			{max}
			{step}
			{placeholder}
			class="w-full bg-transparent outline-none"
			aria-label={label}
		/>
		{#if unit}<span class="mono text-xs text-[--color-text-muted]">{unit}</span>{/if}
	</div>
</label>
