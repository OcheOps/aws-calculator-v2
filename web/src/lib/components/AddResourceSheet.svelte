<script lang="ts">
	import SideSheet from './SideSheet.svelte';
	import ServicePicker from './ServicePicker.svelte';
	import PriceBadge from './PriceBadge.svelte';
	import Button from './Button.svelte';
	import { registry } from '$lib/services';
	import { makeApi } from '$lib/api/client';
	import type { PricedComponent } from '$lib/services/types';
	import type { AddCuratedItemBody, PricedComponentBody } from '$lib/api/types';

	type Props = {
		open: boolean;
		estimateId: number;
		editToken?: string;
		serviceId: number;
		region: string;
		currency: string;
		advancedHref: string;
		onClose: () => void;
		onAdded: () => void;
	};
	let {
		open,
		estimateId,
		editToken,
		serviceId,
		region,
		currency,
		advancedHref,
		onClose,
		onAdded
	}: Props = $props();

	const api = makeApi();

	// mode: 'picker' | 'form:<key>'
	let mode = $state<string>('picker');
	let input = $state<unknown>(null);
	let components = $state<PricedComponent[]>([]);
	let pricedComponents = $state<PricedComponentBody[]>([]);
	let priceError = $state<string | null>(null);
	let priceLoading = $state(false);
	let saving = $state(false);

	function reset() {
		mode = 'picker';
		input = null;
		components = [];
		pricedComponents = [];
		priceError = null;
		priceLoading = false;
	}

	$effect(() => {
		if (!open) reset();
	});

	function pick(serviceKey: string) {
		mode = `form:${serviceKey}`;
		const entry = registry[serviceKey];
		input = { ...entry.defaults };
		components = [];
		pricedComponents = [];
	}

	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	function schedulePrice(next: { input: unknown; components: PricedComponent[] }) {
		input = next.input;
		components = next.components;
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => priceComponents(next.components), 200);
	}

	async function priceComponents(comps: PricedComponent[]) {
		if (comps.length === 0) {
			pricedComponents = [];
			priceError = null;
			return;
		}
		priceLoading = true;
		priceError = null;
		try {
			const results = await Promise.all(
				comps.map((c) =>
					api.price({
						service: c.service,
						region: c.region ?? region,
						currency,
						attributes: c.attributes,
						term: 'OnDemand'
					})
				)
			);
			pricedComponents = comps.map((c, i) => {
				const p = results[i];
				const unitPrice = parseFloat(p.price_per_unit);
				const monthly = unitPrice * c.quantity;
				return {
					service_code: c.service,
					region_code: c.region ?? region,
					sku: p.sku,
					unit: p.unit,
					quantity: String(c.quantity),
					unit_price: p.price_per_unit,
					monthly_cost: monthly.toFixed(6),
					currency: p.currency,
					attributes: c.attributes
				};
			});
		} catch (e) {
			priceError = (e as Error).message;
			pricedComponents = [];
		} finally {
			priceLoading = false;
		}
	}

	const monthlyTotal = $derived(
		pricedComponents.reduce((acc, p) => acc + parseFloat(p.monthly_cost || '0'), 0)
	);

	const currentKey = $derived(mode.startsWith('form:') ? mode.slice(5) : '');
	const currentEntry = $derived(currentKey ? registry[currentKey] : null);

	async function save() {
		if (!currentEntry || pricedComponents.length === 0) return;
		saving = true;
		try {
			const body: AddCuratedItemBody = {
				line_type: 'curated',
				service_id: serviceId,
				service_key: currentKey,
				region_code: region,
				sku: `curated:${currentKey}:v1`,
				currency,
				user_input: input,
				components: pricedComponents
			};
			await api.addCuratedItem(estimateId, body, editToken);
			onAdded();
			onClose();
		} catch (e) {
			priceError = (e as Error).message;
		} finally {
			saving = false;
		}
	}

	const title = $derived(
		mode === 'picker' ? 'Add resource' : `Add ${currentEntry?.meta.label ?? ''}`
	);
</script>

<SideSheet {open} {title} {onClose}>
	{#if mode === 'picker'}
		<ServicePicker onPick={pick} {advancedHref} />
	{:else if currentEntry}
		{#key currentKey}
			{@const F = currentEntry.Form}
			<F {region} onChange={schedulePrice} />
		{/key}
	{/if}
	{#snippet footer()}
		{#if mode !== 'picker'}
			<div class="space-y-3">
				<div class="flex items-baseline justify-between">
					<span class="text-xs text-[--color-text-muted]">Monthly</span>
					{#if priceLoading}
						<span class="text-xs text-[--color-text-muted]">Loading…</span>
					{:else if priceError}
						<span class="text-xs text-red-500">{priceError}</span>
					{:else}
						<PriceBadge amount={monthlyTotal} {currency} />
					{/if}
				</div>
				<div class="flex justify-end gap-2">
					<button
						onclick={onClose}
						class="rounded-md px-3 py-1.5 text-xs text-[--color-text-muted] hover:text-[--color-text]"
					>
						Cancel
					</button>
					<Button
						onclick={save}
						disabled={saving || pricedComponents.length === 0 || priceLoading || !!priceError}
					>
						{saving ? 'Saving…' : 'Add to estimate'}
					</Button>
				</div>
			</div>
		{/if}
	{/snippet}
</SideSheet>
