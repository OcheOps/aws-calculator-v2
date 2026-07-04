<script lang="ts">
	import Input from '$lib/components/Input.svelte';
	import Card from '$lib/components/Card.svelte';
	import { debounce } from '$lib/utils/debounce';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';

	let { data } = $props();
	let search = $state(data.q);

	// Preserve the ?to=&edit= context across search + service navigations so
	// the estimate editor "resumes" once the user picks a service.
	const to = $derived(page.url.searchParams.get('to') ?? '');
	const edit = $derived(page.url.searchParams.get('edit') ?? '');

	function contextSuffix(existing: URLSearchParams): string {
		const out = new URLSearchParams(existing);
		if (to) out.set('to', to);
		if (edit) out.set('edit', edit);
		return out.toString();
	}

	function serviceHref(code: string): string {
		const params = new URLSearchParams();
		if (to) params.set('to', to);
		if (edit) params.set('edit', edit);
		const q = params.toString();
		return `/services/${code}${q ? '?' + q : ''}`;
	}

	const navigate = debounce((v: string) => {
		const params = new URLSearchParams(page.url.searchParams);
		if (v) params.set('q', v);
		else params.delete('q');
		goto('/services?' + contextSuffix(params), { replaceState: true, keepFocus: true });
	}, 250);

	$effect(() => {
		if (search !== data.q) navigate(search);
	});
</script>

<section class="space-y-6">
	<header class="flex items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">Advanced: all services</h1>
			<p class="mt-1 text-sm text-[--color-text-muted]">
				{data.services?.length ?? 0} AWS services ready to price. Use the
				<a href="/" class="underline">estimate editor</a> for the everyday flow.
			</p>
		</div>
		<div class="w-72">
			<Input placeholder="Search by name or code…" bind:value={search} />
		</div>
	</header>

	{#if data.error}
		<Card title="Backend unavailable">
			<p class="text-sm text-[--color-text-muted]">
				Could not load services: {data.error}.
			</p>
		</Card>
	{:else if data.services.length === 0 && data.q}
		<Card>
			<p class="text-sm text-[--color-text-muted]">
				No services match <span class="mono text-[--color-text]">"{data.q}"</span>.
			</p>
		</Card>
	{:else if data.services.length === 0}
		<Card title="Catalog not populated yet">
			<p class="text-sm text-[--color-text-muted]">
				Trigger the first ingest with:
			</p>
			<pre class="mono mt-3 rounded-md border border-[--color-border] bg-[--color-surface] p-3 text-[11px]"><code>curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" https://calc.example/api/v1/admin/ingest?provider=aws</code></pre>
		</Card>
	{:else}
		<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{#each data.services as s (s.id)}
				<a
					href={serviceHref(s.code)}
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
