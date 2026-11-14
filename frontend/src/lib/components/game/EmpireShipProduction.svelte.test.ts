import '../../../css/app.css';
import { create } from '@bufbuild/protobuf';
import { writable } from 'svelte/store';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import {
	PlayerSchema,
	ShipDesignSchema,
	ShipProductionResponseSchema,
	ShipyardProductionSchema
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
				yearsToComplete: quantity > 4 ? 2 : 1,
				reason: 'Ready'
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
	sessionStorage.removeItem('cs.shipProduction.sort.1.1');
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
	for (const stock of ['11', '22', '33']) {
		await expect.element(row.getByRole('cell', { name: stock, exact: true })).toBeVisible();
	}
});

it('uses compact headers and makes room for a long status on wide layouts', async () => {
	const reason = 'Existing production orders remain after this year and must finish before new ships can be assigned.';
	const preview = response(1);
	preview.yards[0].reason = reason;
	context.previewShipProduction.mockResolvedValue(preview);
	const container = document.createElement('div');
	container.style.width = '360px';
	document.body.append(container);
	try {
		const view = render(EmpireShipProduction, { target: container });
		const status = page.getByRole('cell', { name: reason, exact: true });
		await expect.element(status).toHaveStyle({ whiteSpace: 'nowrap' });
		const scroller = status.element().closest('table')!.parentElement!;
		expect(scroller.scrollWidth).toBeGreaterThan(scroller.clientWidth);
		for (const [label, short] of [
			['Queue resources', 'Res'],
			['Ironium (kT)', 'I'],
			['Boranium (kT)', 'B'],
			['Germanium (kT)', 'G']
		]) {
			const button = page.getByRole('button', { name: label, exact: true });
			await expect.element(button).toHaveTextContent(new RegExp(`^${short}`));
			expect(button.element().closest('th')!.offsetWidth).toBeLessThan(60);
		}

		container.style.width = '1440px';
		expect(container.firstElementChild!.getBoundingClientRect().width).toBe(1440);
		expect(scroller.scrollWidth).toBe(scroller.clientWidth);
		await view.unmount();
	} finally {
		container.remove();
	}
});

it.each([
	['Ironium (kT)', 'descending', 'Gamma', 'Alpha'],
	['Boranium (kT)', 'descending', 'Alpha', 'Gamma'],
	['Germanium (kT)', 'descending', 'Gamma', 'Alpha'],
	['Queue resources', 'descending', 'Gamma', 'Alpha'],
	['Status', 'ascending', 'Gamma', 'Alpha']
])('sorts %s in both directions and remembers the choice on return', async (label, direction, first, second) => {
	const preview = response(1);
	preview.yards.push(
		create(ShipyardProductionSchema, {
			planet: {
				mapObject: { num: 3, name: 'Gamma' },
				spec: { resourcesPerYearAvailable: 100 },
				cargo: { ironium: 100, boranium: 2, germanium: 40 }
			},
			reason: 'Existing orders'
		})
	);
	context.previewShipProduction.mockResolvedValue(preview);
	const view = render(EmpireShipProduction);
	const table = page.getByRole('table', { name: 'Shipyard allocation' });
	const yards = table.getByRole('link');
	const column = table.getByRole('columnheader', { name: label });
	const button = column.getByRole('button');
	await expect.element(yards.nth(0)).toHaveTextContent('Alpha');
	await expect.element(yards.nth(1)).toHaveTextContent('Beta');

	await button.click();
	await expect.element(column).toHaveAttribute('aria-sort', direction);
	await expect.element(yards.nth(0)).toHaveTextContent(first);
	await expect.element(yards.nth(1)).toHaveTextContent(second);
	await expect.element(yards.nth(2)).toHaveTextContent('Beta');

	await button.click();
	const reversed = direction === 'ascending' ? 'descending' : 'ascending';
	await expect.element(column).toHaveAttribute('aria-sort', reversed);
	await expect.element(yards.nth(0)).toHaveTextContent(second);
	await expect.element(yards.nth(1)).toHaveTextContent(first);
	await expect.element(yards.nth(2)).toHaveTextContent('Beta');
	expect(preview.yards.map((yard) => yard.planet?.mapObject?.name)).toEqual(['Alpha', 'Beta', 'Gamma']);
	expect(context.previewShipProduction).toHaveBeenCalledTimes(1);

	await view.unmount();
	render(EmpireShipProduction);
	await expect.element(column).toHaveAttribute('aria-sort', reversed);
	await expect.element(yards.nth(0)).toHaveTextContent(second);
});
