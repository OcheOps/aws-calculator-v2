<script lang="ts">
	import Input from './Input.svelte';
	import Select from './Select.svelte';
	import { debounce } from '$lib/utils/debounce';
	import { makeApi } from '$lib/api/client';
	import { humaniseKey, humaniseRegion } from '$lib/utils/labels';
	import type { Dimension, DimensionValue, Region, Service } from '$lib/api/types';

	type Props = {
		service: Service;
		dimensions: Dimension[];
		regions: Region[];
		onChange: (payload: { region: string; term: string; attributes: Record<string, string> }) => void;
	};

	let { service, dimensions, regions, onChange }: Props = $props();

	const api = makeApi();
	let region = $state('');
	// Pre-populate each dimension slot with an empty string so Svelte 5's
	// bind:value never sees `undefined` (throws props_invalid_value).
	const initialValues: Record<string, string> = {};
	for (const d of dimensions) {
		if (d.kind !== 'region') initialValues[d.key] = '';
	}
	let values = $state<Record<string, string>>(initialValues);
	let dimValues = $state<Record<string, DimensionValue[]>>({});
	let loading = $state<Record<string, boolean>>({});

	// Push required dimensions to the top; optional ones sit below a divider so
	// the form doesn't overwhelm on first render.
	const nonRegionDims = $derived(
		dimensions
			.filter((d) => d.kind !== 'region')
			.slice()
			.sort((a, b) => Number(b.is_required) - Number(a.is_required) || a.sort_order - b.sort_order)
	);
	const requiredKeys = $derived(new Set(nonRegionDims.filter((d) => d.is_required).map((d) => d.key)));
	const filledCount = $derived(
		nonRegionDims.filter((d) => (values[d.key] ?? '') !== '').length
	);
	// Fire /price once EITHER (a) every dimension the ingestor marked required
	// is set, or (b) at least one attribute is filled. Most AWS services have
	// no explicitly-required attribute in the API, and firing on a bare region
	// almost always fails the SKU match and surfaces a scary "Not Found".
	const readyToPrice = $derived(
		region !== '' &&
			(requiredKeys.size > 0
				? [...requiredKeys].every((k) => (values[k] ?? '') !== '')
				: filledCount > 0)
	);

	const emit = debounce(() => {
		if (!readyToPrice) return;
		// Send only the attributes the user actually filled — empty strings
		// match nothing against the JSONB containment operator.
		const attrs: Record<string, string> = {};
		for (const [k, v] of Object.entries(values)) if (v) attrs[k] = v;
		onChange({ region, term: 'OnDemand', attributes: attrs });
	}, 250);

	$effect(() => {
		void region;
		void JSON.stringify(values);
		emit();
	});

	async function loadValues(dim: string) {
		if (dimValues[dim] || loading[dim]) return;
		loading[dim] = true;
		try {
			dimValues[dim] = await api.dimensionValues(service.code, dim);
		} finally {
			loading[dim] = false;
		}
	}
</script>

<div class="space-y-4">
	<Select
		bind:value={region}
		label="Region *"
		placeholder="Choose a region…"
		options={(regions ?? []).map((r) => ({ value: r.code, label: humaniseRegion(r.code) }))}
	/>

	{#if region === ''}
		<p class="text-xs text-[--color-text-muted]">
			Pick a region first — the rest of the form comes to life once you do.
		</p>
	{:else}
		{#each nonRegionDims as dim (dim.key)}
			{#if dim.kind === 'numeric'}
				<Input
					type="number"
					label={humaniseKey(dim.label || dim.key) + (dim.is_required ? ' *' : '')}
					bind:value={values[dim.key]}
				/>
			{:else if dim.kind === 'freeform'}
				<Input
					label={humaniseKey(dim.label || dim.key) + (dim.is_required ? ' *' : '')}
					bind:value={values[dim.key]}
				/>
			{:else}
				<!-- kind === 'enum' -->
				<div
					onfocusin={() => loadValues(dim.key)}
					onmouseenter={() => loadValues(dim.key)}
					role="none"
				>
					<Select
						bind:value={values[dim.key]}
						label={humaniseKey(dim.label || dim.key) + (dim.is_required ? ' *' : '')}
						options={(dimValues[dim.key] ?? []).map((v) => ({
							value: v.value,
							label: v.label ?? v.value
						}))}
						placeholder={loading[dim.key] ? 'Loading…' : 'Select…'}
					/>
				</div>
			{/if}
		{/each}
	{/if}
</div>
