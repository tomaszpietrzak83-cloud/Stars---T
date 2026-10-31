import { create } from '@bufbuild/protobuf';
import { writable, get } from 'svelte/store';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { PlayerSchema, PlayerOrdersSchema, ShipDesignSchema } from '#lib/types/cs-proto.js';
import DesignMaintenance from './DesignMaintenance.svelte';

let context: ReturnType<typeof makeContext>;
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: () => context }));

const old = create(ShipDesignSchema, { num: 1, playerNum: 1, name: 'Sword1', hull: 'Cruiser' });
const replacement = create(ShipDesignSchema, { num: 2, playerNum: 1, name: 'Sword2', hull: 'Cruiser' });
const other = create(ShipDesignSchema, { num: 3, playerNum: 1, name: 'Battleship', hull: 'Battleship' });
function makeContext() {
	return {
		player: writable(create(PlayerSchema, { num: 1, playerOrders: create(PlayerOrdersSchema) })),
		universe: writable({ getMyDesigns: () => [old, replacement, other], fleets: [] }),
		updatePlayerOrders: vi.fn(async () => {})
	};
}
beforeEach(() => { context = makeContext(); });

it('offers only same-hull replacements and saves the selected successor', async () => {
	render(DesignMaintenance, { design: old });
	await expect.element(page.getByRole('option', { name: 'Modernize to Sword2' })).toBeInTheDocument();
	await expect.element(page.getByRole('option', { name: 'Modernize to Battleship' })).not.toBeInTheDocument();
	await page.getByRole('combobox').selectOptions('2');
	await page.getByRole('button', { name: 'Apply', exact: true }).click();
	expect(context.updatePlayerOrders).toHaveBeenCalledOnce();
	expect(get(context.player).playerOrders!.designMaintenance).toEqual({ 1: 2 });
});

it('uses a distinct obsolete action for scrapping', async () => {
	render(DesignMaintenance, { design: old });
	await page.getByRole('combobox').selectOptions('-1');
	await page.getByRole('button', { name: 'Apply', exact: true }).click();
	expect(get(context.player).playerOrders!.designMaintenance).toEqual({ 1: -1 });
});

it('restores the saved choice and shows an error when saving fails', async () => {
	context.updatePlayerOrders.mockRejectedValueOnce(new Error('Connection failed'));
	render(DesignMaintenance, { design: old });
	await page.getByRole('combobox').selectOptions('2');
	await page.getByRole('button', { name: 'Apply', exact: true }).click();
	await expect.element(page.getByRole('alert')).toHaveTextContent('Connection failed');
	expect(get(context.player).playerOrders!.designMaintenance).toEqual({});
});
