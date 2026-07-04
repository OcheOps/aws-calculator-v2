import { describe, expect, it } from 'vitest';
import { defaults, map } from './mapper';

describe('EC2 mapper', () => {
	it('returns one component with resolved attributes', () => {
		const comps = map(defaults, 'us-east-1');
		expect(comps).toHaveLength(1);
		expect(comps[0].service).toBe('AmazonEC2');
		expect(comps[0].region).toBe('us-east-1');
		expect(comps[0].attributes.instanceType).toBe('m5.large');
		expect(comps[0].attributes.capacitystatus).toBe('Used');
		expect(comps[0].quantity).toBe(730);
	});

	it('multiplies quantity by count', () => {
		const [c] = map({ ...defaults, count: 3 }, 'us-east-1');
		expect(c.quantity).toBe(3 * 730);
	});

	it('returns no components for degenerate input', () => {
		expect(map({ ...defaults, count: 0 }, 'us-east-1')).toHaveLength(0);
		expect(map({ ...defaults, hoursPerMonth: 0 }, 'us-east-1')).toHaveLength(0);
	});
});
