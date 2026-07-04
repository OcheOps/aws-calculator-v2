<script lang="ts">
	type Option = { value: string; label?: string; group?: string };
	type Props = {
		value?: string;
		options: Option[];
		label?: string;
		placeholder?: string;
	};
	let { value = $bindable(''), options, label = '', placeholder = 'Search…' }: Props = $props();
	let query = $state('');
	let open = $state(false);
	let activeIdx = $state(0);

	const filtered = $derived.by(() => {
		const q = query.toLowerCase();
		if (!q) return options.slice(0, 100);
		return options
			.filter(
				(o) =>
					(o.label ?? o.value).toLowerCase().includes(q) ||
					o.value.toLowerCase().includes(q)
			)
			.slice(0, 100);
	});

	function choose(idx: number) {
		const o = filtered[idx];
		if (!o) return;
		value = o.value;
		query = o.label ?? o.value;
		open = false;
	}

	function onKey(e: KeyboardEvent) {
		if (!open) return;
		if (e.key === 'ArrowDown') {
			activeIdx = Math.min(filtered.length - 1, activeIdx + 1);
			e.preventDefault();
		} else if (e.key === 'ArrowUp') {
			activeIdx = Math.max(0, activeIdx - 1);
			e.preventDefault();
		} else if (e.key === 'Enter') {
			choose(activeIdx);
			e.preventDefault();
		} else if (e.key === 'Escape') {
			open = false;
		}
	}
</script>

<label class="relative block">
	{#if label}<span class="mb-1 block text-xs text-[--color-text-muted]">{label}</span>{/if}
	<input
		type="text"
		bind:value={query}
		onfocus={() => (open = true)}
		onblur={() => setTimeout(() => (open = false), 100)}
		onkeydown={onKey}
		{placeholder}
		class="w-full rounded-md border border-[--color-border] bg-transparent px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
		role="combobox"
		aria-expanded={open}
		aria-controls="combobox-list"
	/>
	{#if open && filtered.length > 0}
		<ul
			id="combobox-list"
			role="listbox"
			class="absolute z-10 mt-1 max-h-64 w-full overflow-auto rounded-md border border-[--color-border] bg-[--color-surface-elev] text-sm"
		>
			{#each filtered as o, i (o.value)}
				<li
					role="option"
					aria-selected={i === activeIdx}
					onmousedown={() => choose(i)}
					class="cursor-pointer px-3 py-2 {i === activeIdx
						? 'bg-[--color-surface] text-[--color-text]'
						: 'text-[--color-text-muted] hover:text-[--color-text]'}"
				>
					<div>{o.label ?? o.value}</div>
					{#if o.group}
						<div class="mono text-[10px] text-[--color-text-muted]">{o.group}</div>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</label>
