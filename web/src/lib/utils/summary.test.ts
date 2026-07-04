import { describe, expect, it } from 'vitest';
import { humanSummary } from './summary';
import type { EstimateItem } from '$lib/api/types';

const base: EstimateItem = {
	id: 1,
	estimate_id: 1,
	service_id: 141,
	region_code: 'us-east-1',
	sku: 'curated:ec2:v1',
	quantity: '730',
	attributes: {},
	unit_price: '0.096',
	currency: 'USD',
	monthly_cost: '70.08',
	line_type: 'curated'
};

describe('humanSummary', () => {
	it('formats a curated EC2 line', () => {
		const item: EstimateItem = {
			...base,
			service_key: 'ec2',
			service_code: 'AmazonEC2',
			user_input: {
				count: 2,
				instanceType: 'm5.large',
				hoursPerMonth: 730,
				operatingSystem: 'Linux'
			}
		};
		const s = humanSummary(item);
		expect(s).toContain('EC2');
		expect(s).toContain('m5.large');
		expect(s).toContain('Linux');
		expect(s).toContain('us-east-1');
	});

	it('falls back to attribute list for legacy lines', () => {
		const item: EstimateItem = {
			...base,
			line_type: 'legacy',
			service_code: 'AmazonS3',
			sku: 'S3-STORAGE',
			attributes: { storageClass: 'General Purpose', volumeType: 'Standard' }
		};
		const s = humanSummary(item);
		expect(s).toContain('AmazonS3');
		expect(s).toContain('storageClass=General Purpose');
	});

	it('curated line without service_key falls back to attribute list', () => {
		const item: EstimateItem = {
			...base,
			line_type: 'curated',
			service_code: 'AmazonEC2',
			attributes: { instanceType: 'm5.large' }
		};
		const s = humanSummary(item);
		expect(s).toContain('AmazonEC2');
	});
});
