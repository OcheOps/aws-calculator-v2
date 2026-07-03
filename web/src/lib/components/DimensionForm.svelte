<script lang="ts">
	import Input from './Input.svelte';
	import Select from './Select.svelte';
	import { debounce } from '$lib/utils/debounce';
	import { makeApi } from '$lib/api/client';
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
	let values = $state<Record<string, string>>({});
	let dimValues = $state<Record<string, DimensionValue[]>>({});
	let loading = $state<Record<string, boolean>>({});

	const nonRegionDims = $derived(dimensions.filter((d) => d.kind !== 'region'));

	const emit = debounce(() => {
		if (!region) return;
		const required = dimensions.filter((d) => d.is_required && d.kind !== 'region').map((d) => d.key);
		for (const k of required) {
			if (!values[k]) return;
		}
		onChange({ region, term: 'OnDemand', attributes: { ...values } });
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
		label="Region"
		options={regions.map((r) => ({ value: r.code, label: r.code }))}
	/>
	{#each nonRegionDims as dim (dim.key)}
		{#if dim.kind === 'numeric'}
			<Input type="number" label={dim.label} bind:value={values[dim.key]} />
		{:else if dim.kind === 'freeform'}
			<Input label={dim.label} bind:value={values[dim.key]} />
		{:else}
			<!-- kind === 'enum' -->
			<div
				onfocusin={() => loadValues(dim.key)}
				onmouseenter={() => loadValues(dim.key)}
				role="none"
			>
				<Select
					bind:value={values[dim.key]}
					label={dim.label + (dim.is_required ? ' *' : '')}
					options={(dimValues[dim.key] ?? []).map((v) => ({ value: v.value, label: v.label }))}
					placeholder={loading[dim.key] ? 'Loading…' : 'Select…'}
				/>
			</div>
		{/if}
	{/each}
</div>
