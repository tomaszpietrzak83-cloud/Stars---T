import '../../../css/app.css';
import { create } from '@bufbuild/protobuf';
import { writable } from 'svelte/store';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import {
	PlayerSchema,
	ShipDesignSchema,
	ShipProductionResponseSchema
} from '#lib/types/cs-proto.js';
import EmpireShipProduction from './EmpireShipProduction.svelte';
const state = vi.hoisted(() => ({ context: undefined as unknown }));
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: () => state.context }));
function response(quantity: number) {
	return create(ShipProductionResponseSchema, {
		assignedQuantity: quantity,
		requestedQuantity: quantity,
		thisYearQuantity: Math.min(4, quantity),
		sourceHash: 'source-hash',
		year: 2400,
		yards: [
			{
				planet: {
					mapObject: { num: 1, name: 'Alpha' },
					spec: { resourcesPerYear: 99, resourcesPerYearAvailable: 72 },
					cargo: { ironium: 11, boranium: 22, germanium: 33 }
				},
				capacityThisYear: 2,
				thisYearQuantity: Math.min(2, quantity),
				laterQuantity: Math.max(0, quantity - 4),
				yearsToComplete: quantity > 4 ? 2 : 1
			},
			{
				planet: { mapObject: { num: 2, name: 'Beta' } },
				capacityThisYear: 2,
				thisYearQuantity: Math.max(0, Math.min(2, quantity - 2)),
				yearsToComplete: quantity > 2 ? 1 : 0
			}
		]
	});
}
let context: ReturnType<typeof makeContext>;
function makeContext() {
	const design = create(ShipDesignSchema, {
		num: 1,
		name: 'Destroyer',
		spec: { hasWeapons: true, fuelCapacity: 100, defaultAutoRole: 'patrol' }
	});
	return {
		game: writable({ id: 1n, year: 2400 }),
		player: writable(create(PlayerSchema, { num: 1, playerOrders: {} })),
		universe: writable({ getMyDesigns: () => [design] }),
		readOnly: writable(false),
		previewShipProduction: vi.fn(
			async (_num: number, quantity: number, _role: string, fill = false) =>
				response(fill ? 4 : quantity)
		),
		queueShipProduction: vi.fn(
			async (_num: number, quantity: number, _role: string, _hash: string) => response(quantity)
		)
	};
}
beforeEach(() => {
	context = makeContext();
	state.context = context;
});
it('shows blue only after all bases current-year capacity is filled and applies chosen Auto orders', async () => {
	render(EmpireShipProduction);
	await page.getByRole('spinbutton', { name: 'Total ship quantity' }).fill('4');
	await expect.element(page.getByText('4 this year', { exact: true })).toBeVisible();
	await expect.element(page.getByText('0 later', { exact: true })).toBeVisible();
	await page.getByRole('button', { name: '+1', exact: true }).click();
	await expect.element(page.getByText('1 later', { exact: true })).toBeVisible();
	await expect.element(page.getByText('4 this year', { exact: true })).toBeVisible();
	await page.getByRole('combobox', { name: 'Production Auto orders' }).selectOptions('tanker');
	await expect
		.element(page.getByRole('button', { name: 'Queue ships', exact: true }))
		.toBeEnabled();
	await page.getByRole('button', { name: 'Queue ships', exact: true }).click();
	expect(context.queueShipProduction).toHaveBeenCalledWith(1, 5, 'tanker', 'source-hash');
	await expect
		.element(page.getByText('Queued 5 ships: 4 this year, 1 later.', { exact: true }))
		.toBeVisible();
});
it('fills this year and makes the next increment visibly blue', async () => {
	render(EmpireShipProduction);
	await expect
		.element(page.getByRole('button', { name: "Fill this year's capacity", exact: true }))
		.toBeEnabled();
	await page.getByRole('button', { name: "Fill this year's capacity", exact: true }).click();
	await expect
		.element(page.getByRole('spinbutton', { name: 'Total ship quantity' }))
		.toHaveValue(4);
	await expect.element(page.getByText('0 later', { exact: true })).toBeVisible();
	await page.getByRole('button', { name: '+1', exact: true }).click();
	await expect.element(page.getByText('1 later', { exact: true })).toBeVisible();
});
it('keeps the preview available but prevents writes in read-only mode', async () => {
	context.readOnly.set(true);
	render(EmpireShipProduction);
	await expect.element(page.getByText('1 this year', { exact: true })).toBeVisible();
	await expect
		.element(page.getByRole('button', { name: 'Queue ships', exact: true }))
		.toBeDisabled();
	expect(context.queueShipProduction).not.toHaveBeenCalled();
});

it('links to the planet queue and shows its production budget and mineral stock', async () => {
	render(EmpireShipProduction);
	const row = page.getByRole('row').filter({ has: page.getByRole('link', { name: 'Alpha' }) });
	await expect.element(row.getByRole('link', { name: 'Alpha' }))
		.toHaveAttribute('href', '/games/1/planets/1/production');
	await expect.element(row.getByRole('cell', { name: '72', exact: true })).toBeVisible();
	await expect.element(row.getByRole('cell', { name: '11 / 22 / 33', exact: true })).toBeVisible();
});
