<script lang="ts">
	type Props = {
		value?: string;
		placeholder?: string;
		type?: string;
		label?: string;
		name?: string;
		required?: boolean;
	};
	let {
		value = $bindable(''),
		placeholder = '',
		type = 'text',
		label = '',
		name,
		required = false
	}: Props = $props();
</script>

<label class="block">
	{#if label}<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>{/if}
	<input
		{type}
		{name}
		{required}
		bind:value
		{placeholder}
		class="w-full rounded-md border border-[--color-border] bg-transparent px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
	/>
</label>
