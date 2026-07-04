import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import SideSheet from './SideSheet.svelte';

describe('SideSheet', () => {
	it('renders children and title when open', () => {
		render(SideSheet, {
			props: {
				open: true,
				title: 'Add EC2',
				onClose: () => {}
			}
		});
		expect(screen.getByText('Add EC2')).toBeInTheDocument();
		expect(screen.getByRole('dialog')).toBeInTheDocument();
	});

	it('is hidden when open=false', () => {
		render(SideSheet, { props: { open: false, title: 'Add EC2', onClose: () => {} } });
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('calls onClose on backdrop click', async () => {
		const onClose = vi.fn();
		render(SideSheet, { props: { open: true, title: 't', onClose } });
		await fireEvent.click(screen.getByTestId('sidesheet-backdrop'));
		expect(onClose).toHaveBeenCalledOnce();
	});
});
