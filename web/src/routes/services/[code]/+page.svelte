<script lang="ts">
	import DimensionForm from '$lib/components/DimensionForm.svelte';
	import PriceBadge from '$lib/components/PriceBadge.svelte';
	import MatchQualityBadge from '$lib/components/MatchQualityBadge.svelte';
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import Input from '$lib/components/Input.svelte';
	import { currency } from '$lib/stores/currency.svelte';
	import { makeApi } from '$lib/api/client';
	import { humaniseUnit } from '$lib/utils/labels';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
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

	async function addToEstimate() {
		if (!livePrice || !lastPayload) return;
		const to = page.url.searchParams.get('to');
		const edit = page.url.searchParams.get('edit') ?? undefined;
		const qty = parseFloat(quantity) || 1;
		if (to) {
			// Advanced mode: POST directly to the target estimate, then bounce back.
			try {
				await api.addItem(
					Number(to),
					{
						service_id: data.service.id,
						region_code: lastPayload.region,
						sku: livePrice.sku,
						quantity: String(qty),
						attributes: lastPayload.attributes,
						unit_price: livePrice.price_per_unit,
						unit: livePrice.unit,
						currency: livePrice.currency
					},
					edit
				);
				const q = edit ? `?edit=${encodeURIComponent(edit)}` : '';
				await goto(`/estimates/${to}${q}`);
				return;
			} catch (e) {
				alert(`Could not add: ${(e as Error).message}`);
				return;
			}
		}
		// Fallback (no ?to=): behaviour is intentionally a no-op in the new
		// design. The service-detail page is only meant to be reached from the
		// AddResourceSheet's Advanced link, which always sets ?to=.
		alert('Open this page from an estimate to add resources.');
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
				<p class="mt-2 text-xs text-[--color-text-muted]">{humaniseUnit(livePrice.unit)}</p>
				{#if livePrice.fx_rate}
					<p class="mt-1 text-[10px] text-[--color-text-muted]">
						FX rate {livePrice.fx_rate}{livePrice.fx_as_of
							? ` · as of ${new Date(livePrice.fx_as_of).toLocaleDateString()}`
							: ''}
					</p>
				{/if}
				<div class="mt-4">
					<Input label="Quantity" type="number" bind:value={quantity} />
				</div>
				<div class="mt-3 flex justify-end">
					<Button onclick={addToEstimate} disabled={!livePrice}>Add to estimate</Button>
				</div>
			{:else if priceError}
				<p class="text-sm text-red-500">Could not price: {priceError}</p>
				<p class="mt-2 text-xs text-[--color-text-muted]">
					Try widening one of the enum filters — some attribute combinations don't have a SKU.
				</p>
			{:else}
				<p class="text-sm text-[--color-text-muted]">
					Pick a region and at least one attribute (like instance type) to see a live price.
					More attributes narrows it further.
				</p>
			{/if}
		</Card>
	</aside>
</div>
