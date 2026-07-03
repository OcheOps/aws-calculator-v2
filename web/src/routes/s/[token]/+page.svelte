<script lang="ts">
	import Card from '$lib/components/Card.svelte';
	import PriceBadge from '$lib/components/PriceBadge.svelte';
	import { session } from '$lib/stores/session.svelte';
	let { data } = $props();

	const total = $derived(
		(data.estimate.items ?? []).reduce((a, it) => a + parseFloat(it.monthly_cost || '0'), 0)
	);
</script>

<section class="space-y-6">
	<header>
		<h1 class="text-3xl font-semibold tracking-tight">{data.estimate.name}</h1>
		<p class="mt-1 text-sm text-[--color-text-muted]">
			Shared estimate · {data.estimate.currency}
		</p>
	</header>

	<Card title="Items">
		{#if (data.estimate.items ?? []).length === 0}
			<p class="text-sm text-[--color-text-muted]">No items in this estimate.</p>
		{:else}
			<table class="w-full text-sm">
				<thead class="text-xs uppercase tracking-wide text-[--color-text-muted]">
					<tr class="border-b border-[--color-border]">
						<th class="py-2 text-left">Service</th>
						<th class="text-left">Region</th>
						<th class="text-right">Qty</th>
						<th class="text-right">Monthly</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-[--color-border]">
					{#each data.estimate.items ?? [] as it (it.id)}
						<tr>
							<td class="py-2">{it.service_code ?? it.service_id}</td>
							<td>{it.region_code}</td>
							<td class="text-right">{it.quantity}</td>
							<td class="text-right">
								<PriceBadge amount={it.monthly_cost} currency={it.currency} />
							</td>
						</tr>
					{/each}
				</tbody>
				<tfoot class="border-t border-[--color-border] text-sm font-medium">
					<tr>
						<td colspan="3" class="py-3 text-right">Total</td>
						<td class="py-3 text-right">
							<PriceBadge amount={total} currency={data.estimate.currency} />
						</td>
					</tr>
				</tfoot>
			</table>
		{/if}
	</Card>

	{#if session.user}
		<Card>
			<p class="text-sm text-[--color-text-muted]">
				Clone-to-workspace coming soon. For now, copy items into a new estimate manually.
			</p>
		</Card>
	{/if}
</section>
