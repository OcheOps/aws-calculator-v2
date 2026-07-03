<script lang="ts">
	type Props = { quality: 'exact' | 'partial'; dropped?: string[] };
	let { quality, dropped }: Props = $props();
	const label =
		quality === 'exact'
			? 'Exact match'
			: `Closest match (dropped: ${(dropped ?? []).join(', ') || 'none'})`;
</script>

<span
	title={label}
	class="rounded-full border px-2 py-0.5 text-[10px] uppercase tracking-wide {quality === 'exact'
		? 'border-emerald-500/40 text-emerald-500'
		: 'border-amber-500/40 text-amber-500'}"
>
	{quality}
</span>
