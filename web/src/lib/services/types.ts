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

export interface ServiceEntry<Input = unknown> {
	meta: ServiceMeta;
	defaults: Input;
	map(input: Input, region: string): PricedComponent[];
	Form: Component;
}
