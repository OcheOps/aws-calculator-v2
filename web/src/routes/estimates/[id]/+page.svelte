<script lang="ts">
	import Card from '$lib/components/Card.svelte';
	import Button from '$lib/components/Button.svelte';
	import PriceBadge from '$lib/components/PriceBadge.svelte';
	import LineRow from '$lib/components/LineRow.svelte';
	import AddResourceSheet from '$lib/components/AddResourceSheet.svelte';
	import Input from '$lib/components/Input.svelte';
	import { makeApi } from '$lib/api/client';
	import { currency as currencyStore } from '$lib/stores/currency.svelte';
	import type { EstimateItem } from '$lib/api/types';
	import { invalidateAll, goto } from '$app/navigation';
	import { page } from '$app/state';

	let { data } = $props();

	const api = makeApi();
	let sheetOpen = $state(false);
	let sharing = $state(false);
	let shareUrl = $state<string | null>(null);
	let name = $state(data.estimate.name);
	let notes = $state(data.estimate.notes ?? '');
	let saving = $state(false);
	let saveError = $state<string | null>(null);

	// Sync name/notes with data on navigation.
	$effect(() => {
		name = data.estimate.name;
		notes = data.estimate.notes ?? '';
	});

	const grouped = $derived.by(() => {
		const buckets = new Map<string, EstimateItem[]>();
		for (const it of data.estimate.items ?? []) {
			const key = it.service_code ?? 'Unknown';
			if (!buckets.has(key)) buckets.set(key, []);
			buckets.get(key)!.push(it);
		}
		return [...buckets.entries()].sort((a, b) => a[0].localeCompare(b[0]));
	});

	const total = $derived(
		(data.estimate.items ?? []).reduce((a, it) => a + parseFloat(it.monthly_cost || '0'), 0)
	);

	const serviceIdForNewLines = $derived.by(() => {
		// Prefer an existing item's service_id (all curated EC2 lines share it);
		// fall back to the first item in the estimate.
		const first = (data.estimate.items ?? [])[0];
		return first?.service_id ?? 0;
	});

	const advancedHref = $derived(
		`/services?to=${data.estimate.id}` +
			(data.editToken ? `&edit=${encodeURIComponent(data.editToken)}` : '')
	);

	async function share() {
		sharing = true;
		try {
			const r = await api.share(data.estimate.id, data.editToken ?? undefined);
			shareUrl = r.share_url;
		} finally {
			sharing = false;
		}
	}

	async function saveMeta() {
		if (name === data.estimate.name && notes === (data.estimate.notes ?? '')) return;
		saving = true;
		saveError = null;
		try {
			// PATCH via fetch — client doesn't currently expose a PATCH helper.
			const res = await fetch(`/api/v1/estimates/${data.estimate.id}`, {
				method: 'PATCH',
				headers: {
					'Content-Type': 'application/json',
					...(data.editToken ? { 'X-Edit-Token': data.editToken } : {})
				},
				body: JSON.stringify({ name, notes })
			});
			if (!res.ok) throw new Error(`PATCH failed (${res.status})`);
			await invalidateAll();
		} catch (e) {
			saveError = (e as Error).message;
		} finally {
			saving = false;
		}
	}

	async function deleteItem(item: EstimateItem) {
		if (!confirm('Delete this line?')) return;
		await api.deleteItem(data.estimate.id, item.id, data.editToken ?? undefined);
		await invalidateAll();
	}

	function editItem(item: EstimateItem) {
		// v1: opening the sheet in edit mode is future work; delete + re-add for now.
		alert('Editing individual lines is coming soon — delete + re-add for now.');
	}

	function exportUrl(format: string) {
		const edit = data.editToken ? `&edit=${encodeURIComponent(data.editToken)}` : '';
		return `/estimates/${data.estimate.id}/export?format=${format}${edit}`;
	}
</script>

<div class="grid gap-8 md:grid-cols-[1fr_320px]">
	<section class="space-y-4">
		<div class="flex items-baseline justify-between">
			<h1 class="text-2xl font-semibold tracking-tight">{data.estimate.name}</h1>
			<Button onclick={() => (sheetOpen = true)}>Add resource</Button>
		</div>

		{#if (data.estimate.items ?? []).length === 0}
			<Card>
				<p class="text-sm text-[--color-text-muted]">
					Empty estimate. Click <span class="text-[--color-text]">Add resource</span> to start.
				</p>
			</Card>
		{:else}
			<Card>
				<table class="w-full">
					<thead>
						<tr class="border-b border-[--color-border] text-xs uppercase tracking-wide text-[--color-text-muted]">
							<th class="py-2 pr-2 text-left">Item</th>
							<th class="py-2 text-right">Monthly</th>
							<th class="py-2 pl-2 text-right"></th>
						</tr>
					</thead>
					{#each grouped as [svc, items] (svc)}
						<tbody>
							<tr class="border-t border-[--color-border] bg-[--color-surface-elev]">
								<td colspan="3" class="py-2 pr-2 text-xs font-medium uppercase tracking-wide text-[--color-text-muted]">
									{svc}
								</td>
							</tr>
							{#each items as it (it.id)}
								<LineRow
									item={it}
									currency={data.estimate.currency}
									onEdit={editItem}
									onDelete={deleteItem}
								/>
							{/each}
							{#if items.length > 1}
								{@const groupTotal = items.reduce((a, it) => a + parseFloat(it.monthly_cost || '0'), 0)}
								<tr class="border-t border-[--color-border]">
									<td class="py-2 pr-2 text-right text-xs uppercase tracking-wide text-[--color-text-muted]">
										Subtotal
									</td>
									<td class="py-2 text-right">
										<PriceBadge amount={groupTotal} currency={data.estimate.currency} muted />
									</td>
									<td></td>
								</tr>
							{/if}
						</tbody>
					{/each}
					<tfoot>
						<tr class="sticky bottom-0 border-t border-[--color-border] bg-[--color-surface]">
							<td class="py-3 pr-2 text-right text-sm font-medium">Grand total</td>
							<td class="py-3 text-right">
								<PriceBadge amount={total} currency={data.estimate.currency} />
							</td>
							<td></td>
						</tr>
					</tfoot>
				</table>
			</Card>
		{/if}
	</section>

	<aside class="space-y-4">
		<Card title="Estimate">
			<Input label="Name" bind:value={name} />
			<div class="mt-3">
				<label class="block text-xs text-[--color-text-muted]">Notes</label>
				<textarea
					bind:value={notes}
					rows="3"
					class="mt-1 w-full rounded-md border border-[--color-border] bg-transparent px-3 py-2 text-sm text-[--color-text] outline-none focus:border-[--color-text]"
				></textarea>
			</div>
			{#if saveError}<p class="mt-2 text-xs text-red-500">{saveError}</p>{/if}
			<div class="mt-3">
				<Button
					variant="secondary"
					onclick={saveMeta}
					disabled={saving ||
						(name === data.estimate.name && notes === (data.estimate.notes ?? ''))}
				>
					{saving ? 'Saving…' : 'Save changes'}
				</Button>
			</div>
		</Card>
		<Card title="Share & export">
			<div class="space-y-3">
				<Button variant="secondary" onclick={share} disabled={sharing}>
					{sharing ? 'Sharing…' : 'Share'}
				</Button>
				{#if shareUrl}
					<p class="mono break-all text-[10px] text-[--color-text-muted]">{shareUrl}</p>
				{/if}
				<div class="flex gap-2">
					<a
						href={exportUrl('pdf')}
						class="rounded-md border border-[--color-border] px-3 py-1.5 text-xs hover:border-[--color-text]"
					>
						PDF
					</a>
					<a
						href={exportUrl('csv')}
						class="rounded-md border border-[--color-border] px-3 py-1.5 text-xs hover:border-[--color-text]"
					>
						CSV
					</a>
					<a
						href={exportUrl('json')}
						class="rounded-md border border-[--color-border] px-3 py-1.5 text-xs hover:border-[--color-text]"
					>
						JSON
					</a>
				</div>
			</div>
		</Card>
	</aside>
</div>

<AddResourceSheet
	open={sheetOpen}
	estimateId={data.estimate.id}
	editToken={data.editToken ?? undefined}
	serviceId={serviceIdForNewLines}
	region={'us-east-1'}
	currency={currencyStore.value}
	{advancedHref}
	onClose={() => (sheetOpen = false)}
	onAdded={() => invalidateAll()}
/>
