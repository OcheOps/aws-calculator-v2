export function money(amount: number | string, currency: string): string {
	const n = typeof amount === 'string' ? parseFloat(amount) : amount;
	if (!isFinite(n)) return `— ${currency}`;
	return new Intl.NumberFormat(undefined, {
		style: 'currency',
		currency,
		minimumFractionDigits: 2,
		maximumFractionDigits: 4
	}).format(n);
}

export function num(amount: number | string): string {
	const n = typeof amount === 'string' ? parseFloat(amount) : amount;
	if (!isFinite(n)) return '—';
	return new Intl.NumberFormat(undefined, { maximumFractionDigits: 6 }).format(n);
}
