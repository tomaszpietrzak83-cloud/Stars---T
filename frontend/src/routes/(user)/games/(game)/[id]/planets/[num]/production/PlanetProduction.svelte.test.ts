import { page as route } from '$app/state';
import { create } from '@bufbuild/protobuf';
import { ProductionQueueItemSchema, QueueItemType } from '#lib/types/cs-proto.js';
import { CommandedPlanet } from '#lib/types/Planet.js';
import { CommandedPlayer } from '#lib/types/Player.js';
import { writable } from 'svelte/store';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import PlanetProduction from './+page.svelte';

const mocks = vi.hoisted(() => ({ context: undefined as unknown, goto: vi.fn() }));
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: () => mocks.context }));
vi.mock('$app/navigation', () => ({ goto: mocks.goto }));

const updatePlanetOrders = vi.fn<(planet: CommandedPlanet) => Promise<void>>();

const otherPlanet = new CommandedPlanet();

afterEach(() => vi.restoreAllMocks());

beforeEach(() => {
	route.params.num = '2';
	mocks.goto.mockReset();
	updatePlanetOrders.mockReset();
	const planet = new CommandedPlanet();
	planet.mapObject.num = 2;
	planet.mapObject.playerNum = 1;
	planet.mapObject.name = 'Beta';
	planet.cargo.ironium = 11;
	planet.cargo.boranium = 22;
	planet.cargo.germanium = 33;
	otherPlanet.mapObject.num = 1;
	otherPlanet.mapObject.playerNum = 1;
	otherPlanet.mapObject.name = 'Alpha';
	planet.factories = 12;
	planet.spec.maxFactories = 24;
	planet.mines = 6;
	planet.spec.maxMines = 18;
	vi.spyOn(CommandedPlanet.prototype, 'getAvailableProductionQueueItems').mockImplementation(() => [
		create(ProductionQueueItemSchema, {
			type: QueueItemType.FACTORY,
			quantity: 1,
			queueItemCompletionEstimate: { yearsToBuildOne: 1 }
		})
	]);
	vi.spyOn(CommandedPlanet.prototype, 'getAvailableProductionQueueShipDesigns').mockReturnValue([]);
	vi.spyOn(CommandedPlanet.prototype, 'getAvailableProductionQueueStarbaseDesigns').mockReturnValue([]);
	const player = new CommandedPlayer();
	player.num = 1;
	vi.spyOn(player, 'getItemCost').mockResolvedValue({ resources: 10 });
	mocks.context = {
		game: writable({ id: 1n }),
		player: writable(player),
		universe: writable({ getPlanet: (num: number) => [otherPlanet, planet][num - 1], designs: [] }),
		commandedPlanet: writable(otherPlanet),
		readOnly: writable(false),
		updatePlanetOrders,
		cs: {
			wasmService: {
				getMaxBuildable: async () => ({ result: 200 }),
				estimateProduction: async ({ planet }: { planet: CommandedPlanet }) => ({ planet })
			}
		}
	};
});

it('shows current and maximum installations and saves before returning to Ship Production', async () => {
	let finishSaving: () => void = () => {};
	updatePlanetOrders.mockImplementation(() => new Promise<void>((resolve) => {
		finishSaving = resolve;
	}));
	render(PlanetProduction);
	const status = page.getByRole('complementary', { name: 'Planet status' });
	await expect.element(status.getByText('12 / 24')).toBeVisible();
	await expect.element(status.getByText('6 / 18')).toBeVisible();
	await expect.element(status.getByText('11 kT')).toBeVisible();
	await expect.element(status.getByText('22 kT')).toBeVisible();
	await expect.element(status.getByText('33 kT')).toBeVisible();
	await page.getByRole('button', { name: 'Ok', exact: true }).click();
	expect(updatePlanetOrders).toHaveBeenCalledOnce();
	expect(mocks.goto).not.toHaveBeenCalled();
	finishSaving();
	await vi.waitFor(() => expect(mocks.goto).toHaveBeenCalledWith('/games/1/ship-production'));
});

it('stays on the queue if saving fails', async () => {
	updatePlanetOrders.mockRejectedValue(new Error('Unable to save queue'));
	render(PlanetProduction);
	await page.getByRole('button', { name: 'Ok', exact: true }).click();
	await expect.element(page.getByRole('alert')).toHaveTextContent('Unable to save queue');
	expect(mocks.goto).not.toHaveBeenCalled();
});

it('adds 100 factories only to the planet in the URL, even with another planet selected', async () => {
	updatePlanetOrders.mockResolvedValue();
	render(PlanetProduction);
	await expect.element(page.getByRole('heading', { name: 'Beta' })).toBeVisible();
	await expect.element(page.getByRole('button', { name: 'Next', exact: true })).not.toBeInTheDocument();
	await page.getByRole('button', { name: 'x100', exact: true }).click();
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	await page.getByRole('button', { name: 'Ok', exact: true }).click();

	expect(updatePlanetOrders).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({
		mapObject: expect.objectContaining({ num: 2 }),
		planetOrders: expect.objectContaining({
			productionQueue: [expect.objectContaining({ type: QueueItemType.FACTORY, quantity: 100 })]
		})
	}));
	expect(otherPlanet.planetOrders.productionQueue).toEqual([]);
});
