import { browser } from '$app/environment';

export type CartLine = {
	service_id: number;
	service_code: string;
	region_code: string;
	sku: string;
	unit: string;
	unit_price: string;
	currency: string;
	quantity: string;
	attributes: Record<string, string>;
	monthly_cost: string;
};

export type CartSnapshot = {
	estimateId?: number;
	editToken?: string;
	lines: CartLine[];
};

const KEY = 'cart';

function load(): CartSnapshot {
	if (!browser) return { lines: [] };
	const raw = localStorage.getItem(KEY);
	if (!raw) return { lines: [] };
	try {
		return JSON.parse(raw) as CartSnapshot;
	} catch {
		return { lines: [] };
	}
}

let snap = $state<CartSnapshot>(load());

function persist() {
	if (browser) localStorage.setItem(KEY, JSON.stringify(snap));
}

export const cart = {
	get value() {
		return snap;
	},
	get lines() {
		return snap.lines;
	},
	get estimateId() {
		return snap.estimateId;
	},
	get editToken() {
		return snap.editToken;
	},
	add(line: CartLine) {
		snap = { ...snap, lines: [...snap.lines, line] };
		persist();
	},
	remove(idx: number) {
		snap = { ...snap, lines: snap.lines.filter((_, i) => i !== idx) };
		persist();
	},
	setEstimate(id: number, editToken?: string) {
		snap = { ...snap, estimateId: id, editToken };
		persist();
	},
	clear() {
		snap = { lines: [] };
		persist();
	}
};
