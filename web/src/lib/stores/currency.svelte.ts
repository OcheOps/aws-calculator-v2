import { browser } from '$app/environment';

let value = $state<string>('USD');
if (browser) value = localStorage.getItem('currency') ?? 'USD';

export const currency = {
	get value() {
		return value;
	},
	set(v: string) {
		value = v;
		if (browser) localStorage.setItem('currency', v);
	}
};
