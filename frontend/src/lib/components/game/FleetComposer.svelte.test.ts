import { create } from '@bufbuild/protobuf';
import { get, writable } from 'svelte/store';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { PlayerSchema, PlayerOrdersSchema, LogisticsAutomationSchema, FleetSchema, MapObjectSchema, ShipDesignSchema, PlanetSchema, PlanetSpecSchema, PlanetStarbaseSpecSchema } from '#lib/types/cs-proto.js';
import FleetComposer from './FleetComposer.svelte';

let context: ReturnType<typeof makeContext>;
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: () => context }));
function makeContext() {
	const fleet = create(FleetSchema, { mapObject: create(MapObjectSchema, { num: 1, playerNum: 1, name: 'Scout Wing' }), tokens: [{ designNum: 1, quantity: 3 }] });
	const design = create(ShipDesignSchema, { num: 1, playerNum: 1, name: 'Sword' });
	const planet = create(PlanetSchema, { mapObject: create(MapObjectSchema, { num: 7, playerNum: 1, name: 'Home' }), spec: create(PlanetSpecSchema, { planetStarbaseSpec: create(PlanetStarbaseSpecSchema, { hasStarbase: true }) }) });
	return { player: writable(create(PlayerSchema, { num: 1, playerOrders: create(PlayerOrdersSchema, { automation: create(LogisticsAutomationSchema) }) })), universe: writable({ fleets: [fleet], planets: [planet], getMyDesigns: () => [design] }), updatePlayerOrders: vi.fn(async () => {}) };
}
beforeEach(() => { context = makeContext(); });
it('orders a composition using existing ships and optional missing production', async () => {
	render(FleetComposer);
	await page.getByRole('button', { name: 'Create fleet composition', exact: true }).click();
	await page.getByLabelText('Fleet composition name').fill('Sword Squadron');
	await page.getByLabelText('Quantity Sword').fill('5');
	await page.getByRole('checkbox', { name: 'Scout Wing — 3 ships' }).click();
	await page.getByRole('button', { name: 'Save fleet composition' }).click();
	const plan = get(context.player).playerOrders!.automation!.fleetAssemblies[1];
	expect(plan.name).toBe('Sword Squadron'); expect(plan.designs).toEqual({ 1: 5 }); expect(plan.sourceFleets).toEqual({ 1: true }); expect(plan.rallyPlanetNum).toBe(7); expect(plan.buildMissing).toBe(true); expect(plan.revision).toBe(1);
	await page.getByRole('button', { name: 'Edit composition Sword Squadron' }).click();
	await page.getByLabelText('Quantity Sword').fill('7');
	await page.getByRole('button', { name: 'Save fleet composition' }).click();
	expect(get(context.player).playerOrders!.automation!.fleetAssemblies[1].revision).toBe(2);
});
it('restores the saved order if the server rejects an edit', async () => {
	render(FleetComposer);
	await page.getByRole('button', { name: 'Create fleet composition', exact: true }).click();
	await page.getByLabelText('Quantity Sword').fill('2');
	context.updatePlayerOrders.mockRejectedValueOnce(new Error('Station lost'));
	await page.getByRole('button', { name: 'Save fleet composition' }).click();
	await expect.element(page.getByRole('alert')).toHaveTextContent('Station lost');
	expect(get(context.player).playerOrders!.automation!.fleetAssemblies).toEqual({});
});
