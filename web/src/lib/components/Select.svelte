<script lang="ts">
	type Option = { value: string; label?: string };
	type Props = {
		value?: string;
		options?: Option[];
		label?: string;
		placeholder?: string;
	};
	let {
		value = $bindable(''),
		options = [],
		label = '',
		placeholder = 'Select…'
	}: Props = $props();
</script>

<label class="block">
	{#if label}<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>{/if}
	<select
		bind:value
		class="w-full rounded-md border border-[--color-border] bg-[--color-surface] px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
	>
		<option value="" disabled>{placeholder}</option>
		{#each options as o (o.value)}
			<option value={o.value}>{o.label ?? o.value}</option>
		{/each}
	</select>
</label>
