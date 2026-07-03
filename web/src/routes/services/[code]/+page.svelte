<script lang="ts">
	import DimensionForm from '$lib/components/DimensionForm.svelte';
	import EstimateCart from '$lib/components/EstimateCart.svelte';
	import PriceBadge from '$lib/components/PriceBadge.svelte';
	import MatchQualityBadge from '$lib/components/MatchQualityBadge.svelte';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Input from '$lib/components/Input.svelte';
	import { currency } from '$lib/stores/currency.svelte';
	import { cart } from '$lib/stores/cart.svelte';
	import { makeApi } from '$lib/api/client';
	import type { PriceResponse } from '$lib/api/types';

	const api = makeApi();
	let { data } = $props();

	let livePrice = $state<PriceResponse | null>(null);
	let priceError = $state<string | null>(null);
	let lastPayload = $state<{
		region: string;
		term: string;
		attributes: Record<string, string>;
	} | null>(null);
	let quantity = $state('1');

	async function handleChange(payload: {
		region: string;
		term: string;
		attributes: Record<string, string>;
	}) {
		lastPayload = payload;
		try {
			livePrice = await api.price({
				service: data.service.code,
				region: payload.region,
				term: payload.term,
				attributes: payload.attributes,
				currency: currency.value
			});
			priceError = null;
		} catch (e) {
			livePrice = null;
			priceError = (e as Error).message;
		}
	}

	function addToEstimate() {
		if (!livePrice || !lastPayload) return;
		const monthlyMultiplier = /^hr?s?|hour/i.test(livePrice.unit) ? 730 : 1;
		const unitPrice = parseFloat(livePrice.price_per_unit);
		const qty = parseFloat(quantity) || 1;
		const monthly = (unitPrice * qty * monthlyMultiplier).toString();
		cart.add({
			service_id: data.service.id,
			service_code: data.service.code,
			region_code: lastPayload.region,
			sku: livePrice.sku,
			unit: livePrice.unit,
			unit_price: livePrice.price_per_unit,
			currency: livePrice.currency,
			quantity,
			attributes: lastPayload.attributes,
			monthly_cost: monthly
		});
	}

	// Refresh price when the display currency changes.
	$effect(() => {
		void currency.value;
		if (lastPayload) handleChange(lastPayload);
	});
</script>

<div class="grid gap-8 md:grid-cols-[1fr_340px]">
	<div class="space-y-6">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{data.service.name}</h1>
			<p class="mono text-xs text-[--color-text-muted]">{data.service.code}</p>
		</div>
		<Card>
			<DimensionForm
				service={data.service}
				dimensions={data.dimensions}
				regions={data.regions}
				onChange={handleChange}
			/>
		</Card>
	</div>

	<aside class="space-y-4">
		<Card title="Live price">
			{#if livePrice}
				<div class="flex items-baseline justify-between">
					<PriceBadge amount={livePrice.price_per_unit} currency={livePrice.currency} />
					<MatchQualityBadge
						quality={livePrice.match_quality}
						dropped={livePrice.dropped_attributes}
					/>
				</div>
				<p class="mt-2 text-xs text-[--color-text-muted]">per {livePrice.unit}</p>
				{#if livePrice.fx_rate}
					<p class="mt-1 text-[10px] text-[--color-text-muted]">
						FX rate {livePrice.fx_rate} · {livePrice.fx_as_of ?? ''}
					</p>
				{/if}
				<div class="mt-4">
					<Input label="Quantity" type="number" bind:value={quantity} />
				</div>
				<div class="mt-3 flex justify-end">
					<Button onclick={addToEstimate}>Add to estimate</Button>
				</div>
			{:else if priceError}
				<p class="text-sm text-red-500">{priceError}</p>
			{:else}
				<p class="text-sm text-[--color-text-muted]">Fill in required attributes to price.</p>
			{/if}
		</Card>
		<EstimateCart />
	</aside>
</div>
