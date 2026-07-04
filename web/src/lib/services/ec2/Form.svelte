<script lang="ts">
	import Combobox from '$lib/components/Combobox.svelte';
	import PresetSelect from '$lib/components/PresetSelect.svelte';
	import NumberInput from '$lib/components/NumberInput.svelte';
	import { defaults, map, type EC2Input } from './mapper';
	import type { PricedComponent } from '../types';

	type Props = {
		region: string;
		initial?: Partial<EC2Input>;
		onChange: (payload: { input: EC2Input; components: PricedComponent[] }) => void;
	};
	let { region, initial = {}, onChange }: Props = $props();

	let input = $state<EC2Input>({ ...defaults, ...initial });

	const instanceTypes = [
		{ value: 't3.micro', label: 't3.micro — 2 vCPU 1 GiB' },
		{ value: 't3.small', label: 't3.small — 2 vCPU 2 GiB' },
		{ value: 't3.medium', label: 't3.medium — 2 vCPU 4 GiB' },
		{ value: 't3.large', label: 't3.large — 2 vCPU 8 GiB' },
		{ value: 'm5.large', label: 'm5.large — 2 vCPU 8 GiB' },
		{ value: 'm5.xlarge', label: 'm5.xlarge — 4 vCPU 16 GiB' },
		{ value: 'm5.2xlarge', label: 'm5.2xlarge — 8 vCPU 32 GiB' },
		{ value: 'c5.large', label: 'c5.large — 2 vCPU 4 GiB' },
		{ value: 'c5.xlarge', label: 'c5.xlarge — 4 vCPU 8 GiB' },
		{ value: 'r5.large', label: 'r5.large — 2 vCPU 16 GiB' },
		{ value: 'r5.xlarge', label: 'r5.xlarge — 4 vCPU 32 GiB' }
	];
	const osOptions = [
		{ value: 'Linux', label: 'Linux' },
		{ value: 'Windows', label: 'Windows' },
		{ value: 'RHEL', label: 'RHEL' },
		{ value: 'SUSE', label: 'SUSE' }
	];
	const tenancyOptions = [
		{ value: 'Shared', label: 'Shared (default)' },
		{ value: 'Dedicated', label: 'Dedicated Instance' },
		{ value: 'Host', label: 'Dedicated Host' }
	];
	const usagePresets = [
		{ label: 'Full month (730 h)', value: 730 },
		{ label: 'Weekdays 8×5 (176 h)', value: 176 },
		{ label: 'Half month (365 h)', value: 365 }
	];

	$effect(() => {
		onChange({ input, components: map(input, region) });
	});
</script>

<div class="space-y-4">
	<Combobox
		label="Instance type"
		options={instanceTypes}
		bind:value={input.instanceType}
		placeholder="Search instance types…"
	/>
	<label class="block">
		<span class="mb-1 block text-xs text-[--color-text-muted]">Operating system</span>
		<select
			bind:value={input.operatingSystem}
			class="w-full rounded-md border border-[--color-border] bg-[--color-surface] px-3 py-2 text-sm text-[--color-text]"
		>
			{#each osOptions as o (o.value)}
				<option value={o.value}>{o.label}</option>
			{/each}
		</select>
	</label>
	<label class="block">
		<span class="mb-1 block text-xs text-[--color-text-muted]">Tenancy</span>
		<select
			bind:value={input.tenancy}
			class="w-full rounded-md border border-[--color-border] bg-[--color-surface] px-3 py-2 text-sm text-[--color-text]"
		>
			{#each tenancyOptions as o (o.value)}
				<option value={o.value}>{o.label}</option>
			{/each}
		</select>
	</label>
	<NumberInput label="How many?" unit="instances" min={1} step={1} bind:value={input.count} />
	<div>
		<PresetSelect
			label="Usage per month"
			presets={usagePresets}
			bind:value={input.hoursPerMonth}
		/>
	</div>
</div>
