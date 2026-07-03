<script lang="ts">
	import Input from '$lib/components/Input.svelte';
	import Card from '$lib/components/Card.svelte';
	import { debounce } from '$lib/utils/debounce';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';

	let { data } = $props();
	let search = $state(data.q);

	const navigate = debounce((v: string) => {
		const params = new URLSearchParams(page.url.searchParams);
		if (v) params.set('q', v);
		else params.delete('q');
		goto('/services?' + params.toString(), { replaceState: true, keepFocus: true });
	}, 250);

	$effect(() => {
		if (search !== data.q) navigate(search);
	});
</script>

<section class="space-y-6">
	<header class="flex items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">Services</h1>
			<p class="mt-1 text-sm text-[--color-text-muted]">
				{data.services?.length ?? 0} AWS services ready to price.
			</p>
		</div>
		<div class="w-72">
			<Input placeholder="Search by name or code…" bind:value={search} />
		</div>
	</header>

	{#if data.error}
		<Card title="Backend unavailable">
			<p class="text-sm text-[--color-text-muted]">
				Could not load services: {data.error}. If ingestion is still populating the catalog,
				give it a few minutes.
			</p>
		</Card>
	{:else if data.services.length === 0}
		<Card>
			<p class="text-sm text-[--color-text-muted]">
				No services yet. If this is a fresh install, kick off ingestion via <code
					class="mono">POST /api/v1/admin/ingest</code
				>.
			</p>
		</Card>
	{:else}
		<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{#each data.services as s (s.id)}
				<a
					href="/services/{s.code}"
					class="group rounded-lg border border-[--color-border] bg-[--color-surface-elev] p-4 transition-colors hover:border-[--color-text]"
				>
					<div class="flex items-baseline justify-between">
						<span class="font-medium tracking-tight text-[--color-text]">{s.name}</span>
						<span class="mono text-[10px] text-[--color-text-muted]">{s.code}</span>
					</div>
					{#if s.family}
						<div class="mt-1 text-xs text-[--color-text-muted]">{s.family}</div>
					{/if}
				</a>
			{/each}
		</div>
	{/if}
</section>
