import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import NumberInput from './NumberInput.svelte';

describe('NumberInput', () => {
	it('renders label + unit', () => {
		render(NumberInput, { props: { label: 'Instances', unit: 'count', value: 3 } });
		expect(screen.getByText('Instances')).toBeInTheDocument();
		expect(screen.getByText('count')).toBeInTheDocument();
		expect(screen.getByRole('spinbutton', { name: 'Instances' })).toHaveValue(3);
	});
});
