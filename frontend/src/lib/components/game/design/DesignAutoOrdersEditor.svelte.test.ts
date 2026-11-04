import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { create } from '@bufbuild/protobuf';
import { writable } from 'svelte/store';
import { beforeEach, it, expect, vi } from 'vitest';
import { ShipDesignSchema } from '#lib/types/cs-proto.js';
import DesignAutoOrdersEditor from './DesignAutoOrdersEditor.svelte';

const mocks = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: mocks.get }));
let save: ReturnType<typeof vi.fn>;
beforeEach(() => {
	save = vi.fn(async () => ({}));
	mocks.get.mockReturnValue({ readOnly: writable(false), updateDesignAutoOrders: save });
});

it('allows an in-use design to save Auto orders for future construction', async () => {
	const design = create(ShipDesignSchema, {
		num: 7,
		spec: { hasWeapons: true, numInstances: 25, fuelCapacity: 100 }
	});
	render(DesignAutoOrdersEditor, { design });
	await page.getByLabelText('Auto orders after construction').selectOptions('patrol');
	await page.getByRole('button', { name: 'Save Auto orders' }).click();
	expect(save).toHaveBeenCalledWith(7, 'patrol');
	await expect.element(page.getByText(/fleets keep their current orders and automation/)).toBeVisible();
});

it('shows inherited automation and can explicitly select Manual', async () => {
	render(DesignAutoOrdersEditor, {
		design: create(ShipDesignSchema, {
			num: 8,
			spec: { hasWeapons: true, defaultAutoRole: 'patrol' }
		})
	});
	await expect.element(page.getByLabelText('Auto orders after construction')).toHaveValue('patrol');
	await page.getByLabelText('Auto orders after construction').selectOptions('');
	await page.getByRole('button', { name: 'Save Auto orders' }).click();
	expect(save).toHaveBeenCalledWith(8, 'manual');
});

it('reports save failure and preserves the selected role for retry', async () => {
	save.mockRejectedValueOnce(new Error('Could not save'));
	render(DesignAutoOrdersEditor, {
		design: create(ShipDesignSchema, { num: 9, spec: { hasWeapons: true } })
	});
	await page.getByLabelText('Auto orders after construction').selectOptions('escort');
	await page.getByRole('button', { name: 'Save Auto orders' }).click();
	await expect.element(page.getByRole('alert')).toHaveTextContent('Could not save');
	await expect.element(page.getByLabelText('Auto orders after construction')).toHaveValue('escort');
});

it('does not offer fleet automation for a station', async () => {
	render(DesignAutoOrdersEditor, {
		design: create(ShipDesignSchema, { num: 10, spec: { starbase: true } })
	});
	await expect
		.element(page.getByLabelText('Auto orders after construction'))
		.not.toBeInTheDocument();
});
