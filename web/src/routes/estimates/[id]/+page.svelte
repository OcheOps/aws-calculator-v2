<script lang="ts">
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import PriceBadge from '$lib/components/PriceBadge.svelte';
	import { makeApi } from '$lib/api/client';
	import type { PriceDelta } from '$lib/api/types';
	import { invalidateAll } from '$app/navigation';

	const api = makeApi();
	let { data } = $props();
	let deltas = $state<PriceDelta[] | null>(null);
	let sharing = $state(false);
	let shareUrl = $state<string | null>(null);
	let repricing = $state(false);

	const total = $derived(
		(data.estimate.items ?? []).reduce((a, it) => a + parseFloat(it.monthly_cost || '0'), 0)
	);

	async function reprice(apply: boolean) {
		repricing = true;
		try {
			deltas = await api.reprice(data.estimate.id, apply, data.editToken ?? undefined);
			if (apply) await invalidateAll();
		} finally {
			repricing = false;
		}
	}

	async function share() {
		sharing = true;
		try {
			const r = await api.share(data.estimate.id, data.editToken ?? undefined);
			shareUrl = r.share_url;
		} finally {
			sharing = false;
		}
	}

	function exportUrl(format: string) {
		const edit = data.editToken ? `&edit=${encodeURIComponent(data.editToken)}` : '';
		return `/estimates/${data.estimate.id}/export?format=${format}${edit}`;
	}
</script>

<section class="space-y-6">
	<header class="flex items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">{data.estimate.name}</h1>
			<p class="mt-1 text-sm text-[--color-text-muted]">
				{data.estimate.currency} · updated {new Date(data.estimate.updated_at).toLocaleString()}
			</p>
		</div>
		<div class="flex items-center gap-3">
			<Button variant="secondary" onclick={() => reprice(false)} disabled={repricing}>
				{repricing ? 'Repricing…' : 'Reprice'}
			</Button>
			<Button variant="secondary" onclick={share} disabled={sharing}>
				{sharing ? 'Sharing…' : 'Share'}
			</Button>
		</div>
	</header>

	{#if shareUrl}
		<Card title="Share link">
			<p class="mono break-all text-xs text-[--color-text-muted]">{shareUrl}</p>
		</Card>
	{/if}

	{#if deltas && deltas.length > 0}
		<Card title="Reprice preview">
			<ul class="space-y-2 text-sm">
				{#each deltas as d (d.item_id)}
					<li class="flex items-baseline justify-between">
						<span class="mono text-xs text-[--color-text-muted]">item #{d.item_id}</span>
						<span>
							<PriceBadge amount={d.old_monthly} currency={d.currency} muted />
							<span class="mx-2 text-[--color-text-muted]">→</span>
							<PriceBadge amount={d.new_monthly} currency={d.currency} />
						</span>
					</li>
				{/each}
			</ul>
			<div class="mt-4 flex justify-end">
				<Button onclick={() => reprice(true)}>Apply changes</Button>
			</div>
		</Card>
	{/if}

	<Card title="Items">
		{#if (data.estimate.items ?? []).length === 0}
			<p class="text-sm text-[--color-text-muted]">No items yet.</p>
		{:else}
			<table class="w-full text-sm">
				<thead class="text-xs uppercase tracking-wide text-[--color-text-muted]">
					<tr class="border-b border-[--color-border]">
						<th class="py-2 text-left">Service</th>
						<th class="text-left">Region</th>
						<th class="text-left">SKU</th>
						<th class="text-right">Qty</th>
						<th class="text-right">Monthly</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-[--color-border]">
					{#each data.estimate.items ?? [] as it (it.id)}
						<tr>
							<td class="py-2">{it.service_code ?? it.service_id}</td>
							<td>{it.region_code}</td>
							<td class="mono text-xs">{it.sku}</td>
							<td class="text-right">{it.quantity}</td>
							<td class="text-right">
								<PriceBadge amount={it.monthly_cost} currency={it.currency} />
							</td>
						</tr>
					{/each}
				</tbody>
				<tfoot class="border-t border-[--color-border] text-sm font-medium">
					<tr>
						<td colspan="4" class="py-3 text-right">Total</td>
						<td class="py-3 text-right">
							<PriceBadge amount={total} currency={data.estimate.currency} />
						</td>
					</tr>
				</tfoot>
			</table>
		{/if}
	</Card>

	<Card title="Export">
		<div class="flex gap-3">
			<a
				href={exportUrl('pdf')}
				class="rounded-md border border-[--color-border] px-3 py-1.5 text-sm hover:border-[--color-text]"
			>
				PDF
			</a>
			<a
				href={exportUrl('csv')}
				class="rounded-md border border-[--color-border] px-3 py-1.5 text-sm hover:border-[--color-text]"
			>
				CSV
			</a>
			<a
				href={exportUrl('json')}
				class="rounded-md border border-[--color-border] px-3 py-1.5 text-sm hover:border-[--color-text]"
			>
				JSON
			</a>
		</div>
		{#if data.editToken}
			<p class="mt-3 text-xs text-[--color-text-muted]">
				Anonymous estimate. Bookmark this page's URL — it contains your edit token.
			</p>
		{/if}
	</Card>
</section>
