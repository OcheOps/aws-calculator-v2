import type { PricedComponent, ServiceMeta } from '../types';

export type EC2Input = {
	instanceType: string;
	operatingSystem: 'Linux' | 'Windows' | 'RHEL' | 'SUSE';
	tenancy: 'Shared' | 'Dedicated' | 'Host';
	count: number;
	hoursPerMonth: number;
};

export const meta: ServiceMeta = {
	code: 'ec2',
	label: 'EC2 (compute)',
	description: 'Virtual servers by the hour.',
	icon: '/icons/ec2.svg',
	category: 'Compute'
};

export const defaults: EC2Input = {
	instanceType: 'm5.large',
	operatingSystem: 'Linux',
	tenancy: 'Shared',
	count: 1,
	hoursPerMonth: 730
};

export function map(input: EC2Input, region: string): PricedComponent[] {
	if (input.count <= 0 || input.hoursPerMonth <= 0) return [];
	return [
		{
			service: 'AmazonEC2',
			region,
			attributes: {
				instanceType: input.instanceType,
				operatingSystem: input.operatingSystem,
				tenancy: input.tenancy,
				preInstalledSw: 'NA',
				capacitystatus: 'Used'
			},
			quantity: input.count * input.hoursPerMonth,
			unitLabel: 'hours',
			summary: `${input.count} × ${input.instanceType} in ${region}, ${input.operatingSystem}, ${input.hoursPerMonth} h/mo`
		}
	];
}
