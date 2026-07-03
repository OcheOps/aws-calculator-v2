<script lang="ts">
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	let { data } = $props();
</script>

<section class="space-y-6">
	<header class="flex items-end justify-between">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">Estimates</h1>
			<p class="mt-1 text-sm text-[--color-text-muted]">
				{data.estimates.length} saved.
			</p>
		</div>
		<Button href="/services">Browse services</Button>
	</header>

	{#if data.error}
		<Card><p class="text-sm text-red-500">{data.error}</p></Card>
	{:else if data.estimates.length === 0}
		<Card>
			<p class="text-sm text-[--color-text-muted]">
				Nothing here yet. Head over to <a href="/services" class="underline">services</a> and build
				one.
			</p>
		</Card>
	{:else}
		<div class="divide-y divide-[--color-border] rounded-lg border border-[--color-border]">
			{#each data.estimates as e (e.id)}
				<a
					href="/estimates/{e.id}"
					class="block px-5 py-4 transition-colors hover:bg-[--color-surface-elev]"
				>
					<div class="flex items-baseline justify-between">
						<span class="font-medium">{e.name}</span>
						<span class="mono text-xs text-[--color-text-muted]">
							{new Date(e.updated_at).toLocaleDateString()} · {e.currency}
						</span>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</section>
