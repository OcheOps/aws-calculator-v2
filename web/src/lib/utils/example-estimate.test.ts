import { describe, expect, it } from 'vitest';
import { exampleEstimate } from './example-estimate';

describe('exampleEstimate', () => {
	it('renders as a real Estimate with several items', () => {
		const est = exampleEstimate('USD');
		expect(est.currency).toBe('USD');
		expect(est.items?.length ?? 0).toBeGreaterThanOrEqual(4);
		for (const it of est.items ?? []) {
			expect(parseFloat(it.monthly_cost)).toBeGreaterThan(0);
		}
	});

	it('respects the display currency in each row', () => {
		const est = exampleEstimate('NGN');
		expect(est.currency).toBe('NGN');
		for (const it of est.items ?? []) expect(it.currency).toBe('NGN');
	});
});
