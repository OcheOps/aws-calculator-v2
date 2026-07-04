import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import Combobox from './Combobox.svelte';

describe('Combobox', () => {
	const options = [
		{ value: 'm5.large', label: 'm5.large — 2 vCPU 8 GiB' },
		{ value: 'm5.xlarge', label: 'm5.xlarge — 4 vCPU 16 GiB' },
		{ value: 'c5.large' }
	];

	it('filters options on typing', async () => {
		render(Combobox, { props: { options, value: '' } });
		const input = screen.getByRole('combobox');
		await fireEvent.focus(input);
		await fireEvent.input(input, { target: { value: 'm5' } });
		expect(screen.getAllByRole('option')).toHaveLength(2);
	});

	it('picks with keyboard', async () => {
		render(Combobox, { props: { options, value: '' } });
		const input = screen.getByRole('combobox');
		await fireEvent.focus(input);
		await fireEvent.keyDown(input, { key: 'ArrowDown' });
		await fireEvent.keyDown(input, { key: 'Enter' });
		expect((input as HTMLInputElement).value.length).toBeGreaterThan(0);
	});
});
