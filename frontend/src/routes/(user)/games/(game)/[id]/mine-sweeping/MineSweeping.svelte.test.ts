import { create } from '@bufbuild/protobuf';
import { readable } from 'svelte/store';
import { expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { FleetSchema, PlayerSchema } from '#lib/types/cs-proto.js';
import MineSweeping from './+page.svelte';

const mocks = vi.hoisted(() => ({ context: undefined as unknown, goto: vi.fn() }));
vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: () => mocks.context }));

it('counts current results and selects the exact fleet on the map', async () => {
	const fresh = create(FleetSchema, { mapObject: { num: 7, name: 'Mine Swap #7', tags: {
		'last-mine-sweep': 'Year 2498 result: 325 mines removed from 2 fields'
	} } });
	const stale = create(FleetSchema, { mapObject: { num: 9, name: 'Mine Swap #9', tags: {
		'last-mine-sweep': 'Year 2497 result: 999 mines removed from 1 fields'
	} } });
	const commandMapObject = vi.fn();
	const zoomToMapObject = vi.fn();
	mocks.context = {
		game: readable({ id: 21n, year: 2498 }),
		player: readable(create(PlayerSchema, {
			playerOrders: { automation: { fleetRoles: { 7: 'minesweeper', 9: 'minesweeper' } } }
		})),
		universe: readable({ getMyDesigns: () => [], getMyFleets: () => [stale, fresh] }),
		commandMapObject,
		zoomToMapObject
	};
	render(MineSweeping);
	await expect.element(page.getByText('325', { exact: true })).toBeVisible();
	await expect.element(page.getByText('325 mines', { exact: true })).toBeVisible();
	await expect.element(page.getByText('0 mines', { exact: true })).toBeVisible();
	await page.getByRole('button', { name: 'Go! Mine Swap #9' }).click();
	expect(commandMapObject).toHaveBeenCalledWith(stale);
	expect(zoomToMapObject).toHaveBeenCalledWith(stale);
	expect(mocks.goto).toHaveBeenCalledWith('/games/21');
});
