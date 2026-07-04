import type { EstimateItem } from '$lib/api/types';

/**
 * humanSummary returns a plain-English one-liner for an estimate item. It
 * mirrors the Go export summary in `api/internal/export/summary.go` so PDF /
 * CSV / web all read the same. Curated lines use their service_key to select
 * a template; legacy or unknown-key curated lines fall back to
 * `service_code · region · key=value, key=value`.
 */
export function humanSummary(item: EstimateItem): string {
	if (item.line_type === 'curated') {
		const s = curatedSummary(item);
		if (s) return s;
	}
	return legacySummary(item);
}

function curatedSummary(item: EstimateItem): string {
	const input = item.user_input as Record<string, unknown> | undefined;
	if (!input) return '';
	switch (item.service_key) {
		case 'ec2':
			return `EC2 · ${input.count} × ${input.instanceType} in ${item.region_code} · ${input.hoursPerMonth} h/mo · ${input.operatingSystem}`;
	}
	// Curated but service_key isn't formatted yet — generic key=value dump.
	const pairs = Object.keys(input)
		.sort()
		.map((k) => `${k}=${input[k]}`);
	return `${item.service_key ?? item.service_code ?? ''} · ${item.region_code} · ${pairs.join(', ')}`;
}

function legacySummary(item: EstimateItem): string {
	const attrs = item.attributes ?? {};
	const pairs = Object.keys(attrs)
		.sort()
		.map((k) => `${k}=${attrs[k]}`);
	return `${item.service_code ?? ''} · ${item.region_code} · ${pairs.join(', ')}`;
}
