import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import ServicePicker from './ServicePicker.svelte';

describe('ServicePicker', () => {
	it('shows every registered service as a tile', () => {
		render(ServicePicker, { props: { onPick: () => {}, advancedHref: '/services' } });
		expect(screen.getByRole('button', { name: /EC2/i })).toBeInTheDocument();
	});

	it('filters tiles when the user types', async () => {
		render(ServicePicker, { props: { onPick: () => {}, advancedHref: '/services' } });
		const input = screen.getByPlaceholderText(/search/i);
		await fireEvent.input(input, { target: { value: 'ec2' } });
		expect(screen.getByRole('button', { name: /EC2/i })).toBeInTheDocument();
	});

	it('calls onPick with the service key', async () => {
		const onPick = vi.fn();
		render(ServicePicker, { props: { onPick, advancedHref: '/services' } });
		await fireEvent.click(screen.getByRole('button', { name: /EC2/i }));
		expect(onPick).toHaveBeenCalledWith('ec2');
	});
});
