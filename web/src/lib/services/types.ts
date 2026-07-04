import type { Component } from 'svelte';

export type PricedComponent = {
	service: string;
	region?: string;
	attributes: Record<string, string>;
	quantity: number;
	unitLabel: string;
	summary?: string;
};

export type ServiceMeta = {
	code: string;
	label: string;
	description: string;
	icon: string;
	category: 'Compute' | 'Storage' | 'Database' | 'Network' | 'Serverless';
};

export interface ServiceMapper<Input> {
	meta: ServiceMeta;
	defaults: Input;
	map(input: Input, region: string): PricedComponent[];
}

// `Form` is intentionally typed with `any` props so a heterogeneous registry
// (one entry per service, each with its own Input shape) can hold them all
// without a discriminated-union apparatus. Consumers dispatch by service key
// and know the expected prop shape from the corresponding mapper.
export interface ServiceEntry<Input = unknown> {
	meta: ServiceMeta;
	defaults: Input;
	map(input: Input, region: string): PricedComponent[];
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	Form: Component<any>;
}
