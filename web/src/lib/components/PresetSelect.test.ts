import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import PresetSelect from './PresetSelect.svelte';

describe('PresetSelect', () => {
	it('selects a preset on click', async () => {
		render(PresetSelect, {
			props: {
				presets: [
					{ label: '30 days', value: 720 },
					{ label: 'Full month', value: 730 }
				],
				value: undefined
			}
		});
		const full = screen.getByText('Full month');
		await fireEvent.click(full);
		expect(full.className).toContain('border-[--color-text]');
	});
});
