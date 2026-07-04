<script lang="ts">
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import PriceBadge from '$lib/components/PriceBadge.svelte';
	import { humanSummary } from '$lib/utils/summary';
	import { exampleEstimate } from '$lib/utils/example-estimate';
	import { currency as currencyStore } from '$lib/stores/currency.svelte';
	import { makeApi } from '$lib/api/client';
	import { goto } from '$app/navigation';

	const api = makeApi();

	const est = $derived(exampleEstimate(currencyStore.value));
	let forking = $state(false);
	let forkError = $state<string | null>(null);

	async function fork() {
		forking = true;
		forkError = null;
		try {
			const created = await api.createEstimate({
				name: 'Untitled',
				currency: currencyStore.value
			});
			for (const it of est.items ?? []) {
				await api.addItem(
					created.estimate.id,
					{
						service_id: 0,
						region_code: it.region_code,
						sku: it.sku,
						quantity: it.quantity,
						attributes: it.attributes,
						unit_price: it.unit_price,
						unit: 'Hrs',
						currency: it.currency
					},
					created.edit_token
				);
			}
			const q = created.edit_token
				? `?edit=${encodeURIComponent(created.edit_token)}`
				: '';
			await goto(`/estimates/${created.estimate.id}${q}`);
		} catch (e) {
			forkError = (e as Error).message;
		} finally {
			forking = false;
		}
	}
</script>

<section class="space-y-12">
	<div class="max-w-2xl space-y-6">
		<h1 class="text-5xl font-semibold leading-tight tracking-tight">Price AWS. In your currency.</h1>
		<p class="text-lg text-[--color-text-muted]">
			Build an AWS estimate in three clicks per line, in USD, EUR, GBP, NGN, and more.
			Share with a link. Export a branded PDF.
		</p>
		<div class="flex gap-3">
			<Button onclick={fork} disabled={forking}>
				{forking ? 'Starting…' : 'Try it — start free estimate'}
			</Button>
		</div>
		{#if forkError}
			<p class="text-xs text-red-500">{forkError}</p>
		{/if}
	</div>

	<div>
		<h2 class="mb-3 text-sm font-medium uppercase tracking-wide text-[--color-text-muted]">
			Example: small SaaS backend
		</h2>
		<Card>
			<table class="w-full">
				<thead>
					<tr class="border-b border-[--color-border] text-xs uppercase tracking-wide text-[--color-text-muted]">
						<th class="py-2 pr-2 text-left">Item</th>
						<th class="py-2 text-right">Monthly</th>
					</tr>
				</thead>
				<tbody>
					{#each est.items ?? [] as it (it.id)}
						<tr class="border-t border-[--color-border]">
							<td class="py-2 pr-2 text-sm text-[--color-text]">{humanSummary(it)}</td>
							<td class="py-2 text-right">
								<PriceBadge amount={it.monthly_cost} currency={est.currency} />
							</td>
						</tr>
					{/each}
				</tbody>
				<tfoot>
					<tr class="border-t border-[--color-border]">
						<td class="py-3 pr-2 text-right text-sm font-medium">Grand total</td>
						<td class="py-3 text-right">
							<PriceBadge amount={parseFloat(est.total ?? '0')} currency={est.currency} />
						</td>
					</tr>
				</tfoot>
			</table>
		</Card>
	</div>
</section>
