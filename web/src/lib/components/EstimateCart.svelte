<script lang="ts">
	import { cart } from '$lib/stores/cart.svelte';
	import { currency } from '$lib/stores/currency.svelte';
	import { session } from '$lib/stores/session.svelte';
	import PriceBadge from './PriceBadge.svelte';
	import Button from './Button.svelte';
	import Input from './Input.svelte';
	import Card from './Card.svelte';
	import { makeApi } from '$lib/api/client';
	import { humaniseRegion } from '$lib/utils/labels';
	import { goto } from '$app/navigation';

	const api = makeApi();
	let name = $state('My estimate');
	let saving = $state(false);
	let error = $state<string | null>(null);

	const total = $derived(
		cart.lines.reduce((acc, l) => acc + parseFloat(l.monthly_cost || '0'), 0)
	);

	async function save() {
		if (cart.lines.length === 0) return;
		saving = true;
		error = null;
		try {
			const created = await api.createEstimate({ name, currency: currency.value });
			for (const line of cart.lines) {
				await api.addItem(
					created.estimate.id,
					{
						service_id: line.service_id,
						region_code: line.region_code,
						sku: line.sku,
						quantity: line.quantity,
						attributes: line.attributes,
						unit_price: line.unit_price,
						unit: line.unit,
						currency: line.currency
					},
					created.edit_token
				);
			}
			cart.setEstimate(created.estimate.id, created.edit_token);
			cart.clear();
			const target = `/estimates/${created.estimate.id}`;
			const q = created.edit_token ? `?edit=${created.edit_token}` : '';
			goto(target + q);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			saving = false;
		}
	}
</script>

<Card title="Estimate">
	{#if cart.lines.length === 0}
		<div class="flex flex-col items-start gap-2 py-2">
			<div class="text-xl" aria-hidden="true">🧮</div>
			<p class="text-sm text-[--color-text-muted]">
				Nothing added yet. Pick a region and the required attributes on the left, then
				<span class="text-[--color-text]">Add to estimate</span>.
			</p>
		</div>
	{:else}
		<ul class="space-y-2 text-sm">
			{#each cart.lines as line, i (i)}
				<li class="flex items-baseline justify-between gap-2">
					<div class="min-w-0">
						<div class="truncate font-medium">{line.service_code}</div>
						<div class="truncate text-xs text-[--color-text-muted]">
							{humaniseRegion(line.region_code).split(' · ')[0]} · qty {line.quantity}
						</div>
					</div>
					<div class="flex items-center gap-2">
						<PriceBadge amount={line.monthly_cost} currency={line.currency} />
						<button
							onclick={() => cart.remove(i)}
							class="text-xs text-[--color-text-muted] hover:text-[--color-text]"
							aria-label="Remove"
						>
							×
						</button>
					</div>
				</li>
			{/each}
		</ul>

		<div class="mt-4 flex items-baseline justify-between border-t border-[--color-border] pt-3">
			<span class="text-xs uppercase tracking-wide text-[--color-text-muted]">Monthly</span>
			<PriceBadge amount={total} currency={currency.value} />
		</div>

		<div class="mt-4 space-y-3">
			<Input label="Name" bind:value={name} />
			{#if error}
				<p class="text-xs text-red-500">{error}</p>
			{/if}
			<Button onclick={save} disabled={saving}>
				{saving ? 'Saving…' : session.user ? 'Save estimate' : 'Save (anonymous)'}
			</Button>
			<button
				onclick={() => cart.clear()}
				class="w-full rounded-md text-xs text-[--color-text-muted] hover:text-[--color-text]"
			>
				Clear cart
			</button>
		</div>
	{/if}
</Card>
