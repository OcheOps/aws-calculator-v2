import { browser } from '$app/environment';

let value = $state<'light' | 'dark'>('light');
if (browser) {
	value =
		(localStorage.getItem('theme') as 'light' | 'dark') ||
		(matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
}

export const theme = {
	get value() {
		return value;
	},
	toggle() {
		value = value === 'dark' ? 'light' : 'dark';
		if (browser) {
			localStorage.setItem('theme', value);
			document.documentElement.classList.toggle('dark', value === 'dark');
		}
	}
};
