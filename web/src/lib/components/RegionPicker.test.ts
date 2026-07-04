import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import RegionPicker from './RegionPicker.svelte';

describe('RegionPicker', () => {
	it('groups regions by continent', () => {
		render(RegionPicker, {
			props: {
				value: 'us-east-1',
				regions: [{ code: 'us-east-1' }, { code: 'eu-west-1' }, { code: 'ap-south-1' }]
			}
		});
		expect(screen.getByRole('combobox')).toHaveValue('us-east-1');
		const select = screen.getByRole('combobox');
		expect(select.innerHTML).toContain('North America');
		expect(select.innerHTML).toContain('Europe');
		expect(select.innerHTML).toContain('Asia Pacific');
	});
});
