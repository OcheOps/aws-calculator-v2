import type { Estimate, EstimateItem } from '$lib/api/types';

/**
 * exampleEstimate returns a plausible "small SaaS backend" estimate the
 * landing page renders below the fold. Not persisted; the numbers are
 * rounded illustrative figures rather than live pricing.
 *
 * When the user first edits anything, the landing page forks this into a
 * real anonymous estimate via POST /estimates + N × POST /items.
 */
export function exampleEstimate(currency: string): Estimate {
	const now = new Date().toISOString();
	const items: EstimateItem[] = [
		mkItem({
			id: -1,
			service_code: 'AmazonEC2',
			service_key: 'ec2',
			line_type: 'curated',
			region_code: 'us-east-1',
			user_input: {
				count: 2,
				instanceType: 't3.medium',
				operatingSystem: 'Linux',
				hoursPerMonth: 730
			},
			monthly_cost: '60.74',
			currency
		}),
		mkItem({
			id: -2,
			service_code: 'AmazonRDS',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { instanceType: 'db.t3.micro', engine: 'PostgreSQL' },
			monthly_cost: '13.87',
			currency
		}),
		mkItem({
			id: -3,
			service_code: 'AmazonS3',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { storageClass: 'Standard', volumeType: 'General Purpose' },
			monthly_cost: '2.30',
			currency
		}),
		mkItem({
			id: -4,
			service_code: 'AmazonCloudFront',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { region: 'US', tier: '1 TB out' },
			monthly_cost: '85.00',
			currency
		}),
		mkItem({
			id: -5,
			service_code: 'ElasticLoadBalancing',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { type: 'Application Load Balancer' },
			monthly_cost: '16.20',
			currency
		}),
		mkItem({
			id: -6,
			service_code: 'AmazonRoute53',
			line_type: 'legacy',
			region_code: 'us-east-1',
			attributes: { zoneType: 'Hosted zone' },
			monthly_cost: '0.50',
			currency
		})
	];

	const total = items.reduce((a, it) => a + parseFloat(it.monthly_cost), 0);

	return {
		id: 0,
		name: 'Example: small SaaS backend',
		currency,
		notes: '',
		items,
		total: total.toFixed(2),
		created_at: now,
		updated_at: now
	};
}

type ItemInit = Partial<EstimateItem> &
	Pick<EstimateItem, 'id' | 'service_code' | 'region_code' | 'monthly_cost' | 'currency' | 'line_type'>;

function mkItem(init: ItemInit): EstimateItem {
	return {
		estimate_id: 0,
		service_id: 0,
		sku: init.sku ?? 'example',
		quantity: init.quantity ?? '1',
		attributes: init.attributes ?? {},
		unit_price: init.unit_price ?? init.monthly_cost,
		...init
	};
}
