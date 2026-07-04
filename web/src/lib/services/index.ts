import type { ServiceEntry } from './types';
import * as ec2 from './ec2/mapper';
import EC2Form from './ec2/Form.svelte';

export const registry: Record<string, ServiceEntry<any>> = {
	ec2: {
		meta: ec2.meta,
		defaults: ec2.defaults,
		map: ec2.map,
		Form: EC2Form
	}
};

export type ServiceKey = keyof typeof registry;

export function listServices(): ServiceEntry<any>[] {
	return Object.values(registry);
}
