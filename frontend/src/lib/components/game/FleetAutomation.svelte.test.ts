import { create } from '@bufbuild/protobuf';
import { writable, get } from 'svelte/store';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import {
	FleetSchema,
	FleetSpecSchema,
	ShipDesignSpecSchema,
	MapObjectSchema,
	PlayerSchema,
	PlayerOrdersSchema,
	LogisticsAutomationSchema
} from '#lib/types/cs-proto.js';
import FleetAutomation from './FleetAutomation.svelte';

let context: ReturnType<typeof makeContext>;
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: () => context }));
function makeContext() {
	const fleet = create(FleetSchema, {
		mapObject: create(MapObjectSchema, { num: 1, playerNum: 1, name: 'Transport' }),
		spec: create(FleetSpecSchema, {
			shipDesignSpec: create(ShipDesignSpecSchema, {
				cargoCapacity: 200,
				fuelCapacity: 500,
				hasWeapons: true,
				miningRate: 10
			})
		}),
		tokens: [{ designNum: 1, quantity: 2 }]
	});
	return {
		player: writable(
			create(PlayerSchema, {
				num: 1,
				playerOrders: create(PlayerOrdersSchema, {
					designMaintenance: { 1: 2 },
					automation: create(LogisticsAutomationSchema)
				})
			})
		),
		universe: writable({ fleets: [fleet] }),
		updatePlayerOrders: vi.fn(async () => {})
	};
}
beforeEach(() => {
	context = makeContext();
});

it('selects and unselects a displayed range with Shift and saves bulk orders once', async () => {
	const original = get(context.universe).fleets[0];
	context.universe.set({
		fleets: [1, 2, 3, 4].map((num) =>
			create(FleetSchema, {
				...original,
				mapObject: create(MapObjectSchema, { num, playerNum: 1, name: `Fleet ${num}` })
			})
		)
	});
	render(FleetAutomation);
	await page.getByRole('checkbox', { name: 'Select Fleet 1', exact: true }).click();
	await page
		.getByRole('checkbox', { name: 'Select Fleet 3', exact: true })
		.click({ modifiers: ['Shift'] });
	for (const num of [1, 2, 3])
		await expect
			.element(page.getByRole('checkbox', { name: `Select Fleet ${num}`, exact: true }))
			.toBeChecked();
	await expect
		.element(page.getByRole('checkbox', { name: 'Select Fleet 4', exact: true }))
		.not.toBeChecked();
	await page
		.getByRole('combobox', { name: 'Auto orders for selected fleets', exact: true })
		.selectOptions('tanker');
	await page.getByRole('button', { name: 'Apply Auto orders (3)', exact: true }).click();
	expect(context.updatePlayerOrders).toHaveBeenCalledTimes(1);
	expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({
		1: 'tanker',
		2: 'tanker',
		3: 'tanker'
	});
});

it('excludes incompatible and refitting fleets from a bulk order', async () => {
	const original = get(context.universe).fleets[0];
	const miner = create(FleetSchema, {
		...original,
		mapObject: create(MapObjectSchema, { num: 2, playerNum: 1, name: 'Miner only' }),
		spec: create(FleetSpecSchema, { shipDesignSpec: { miningRate: 10 } })
	});
	const refit = create(FleetSchema, {
		...original,
		mapObject: create(MapObjectSchema, {
			num: 3,
			playerNum: 1,
			name: 'Refit',
			tags: { 'design-maintenance': '{}' }
		})
	});
	context.universe.set({ fleets: [original, miner, refit] });
	get(context.player).playerOrders!.automation!.fleetRoles = { 2: 'miner', 3: 'freighter' };
	render(FleetAutomation);
	await page.getByRole('button', { name: 'Select all fleets', exact: true }).click();
	await page
		.getByRole('combobox', { name: 'Auto orders for selected fleets', exact: true })
		.selectOptions('tanker');
	await page.getByRole('button', { name: 'Apply Auto orders (1)', exact: true }).click();
	expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({
		1: 'tanker',
		2: 'miner',
		3: 'freighter'
	});
});

it('rolls back a failed bulk change', async () => {
	context.updatePlayerOrders.mockRejectedValueOnce(new Error('Save failed'));
	get(context.player).playerOrders!.automation!.fleetRoles = { 1: 'freighter' };
	render(FleetAutomation);
	await page.getByRole('button', { name: 'Select all fleets', exact: true }).click();
	await page
		.getByRole('combobox', { name: 'Auto orders for selected fleets', exact: true })
		.selectOptions('tanker');
	await page.getByRole('button', { name: 'Apply Auto orders (1)', exact: true }).click();
	await expect.element(page.getByRole('alert')).toHaveTextContent('Save failed');
	expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({ 1: 'freighter' });
});

it('shows the containment assignment in the fleet list', async () => {
	get(context.universe).fleets[0].mapObject!.tags['patrol-containment'] = JSON.stringify({
		status: 'Containing slow formation FF2; cutting off support'
	});
	render(FleetAutomation);
	await expect
		.element(page.getByText('Containing slow formation FF2; cutting off support'))
		.toBeVisible();
});

it('assigns a tanker and shows it in the fuel network', async () => {
	render(FleetAutomation);
	await page.getByRole('combobox', { name: 'Role for Transport' }).selectOptions('tanker');
	expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({ 1: 'tanker' });
	await expect.element(page.getByText('Fuel network: 0 docks and 1 tankers')).toBeVisible();
});

it('issues one rebuild request for the selected fleets', async () => {
	render(FleetAutomation);
	await page.getByRole('checkbox', { name: 'Select Transport', exact: true }).click();
	await page.getByRole('button', { name: 'Rebuild selected (1)', exact: true }).click();
	expect(get(context.player).playerOrders!.automation!.rebuildFleets).toEqual({ 1: true });
});

it('saves the automatic remote mining role', async () => {
	render(FleetAutomation);
	await page.getByRole('combobox', { name: 'Role for Transport' }).selectOptions('miner');
	expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({ 1: 'miner' });
});

it('saves the unvisited-planet scout role', async () => {
	get(context.universe).fleets[0].spec!.shipDesignSpec!.scanner = true;
	render(FleetAutomation);
	await page.getByRole('combobox', { name: 'Role for Transport' }).selectOptions('scout');
	expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({ 1: 'scout' });
});

it('saves a transport role and a fleet-specific rebuild order', async () => {
	render(FleetAutomation);
	await page.getByRole('combobox', { name: 'Role for Transport' }).selectOptions('freighter');
	expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({ 1: 'freighter' });
	await page.getByRole('button', { name: 'Scrap and rebuild', exact: true }).click();
	expect(get(context.player).playerOrders!.automation!.rebuildFleets).toEqual({ 1: true });
});

it('enables victorious-battle salvage and automatic offloading', async () => {
	render(FleetAutomation);
	await page
		.getByRole('checkbox', {
			name: 'Collect debris after victorious battles, up to available cargo capacity'
		})
		.click();
	await page
		.getByRole('checkbox', {
			name: 'Send free automatic transports to collect minerals from combat fleets'
		})
		.click();
	expect(get(context.player).playerOrders!.automation!.battleSalvage).toBe(true);
	expect(get(context.player).playerOrders!.automation!.offloadCombatCargo).toBe(true);
});

it('saves passenger transport thresholds and escort preferences together', async () => {
	get(context.player).playerOrders!.automation!.disablePassengerTransport = true;
	render(FleetAutomation);
	await page
		.getByRole('checkbox', { name: 'Move passengers between source planets and growing colonies' })
		.click();
	await page.getByRole('checkbox', { name: 'Use manual population thresholds' }).click();
	await page.getByLabelText('Source population reserve (%)').fill('85');
	await page.getByLabelText('Destination population target (%)').fill('40');
	await page.getByLabelText('Threat corridor (ly)').fill('150');
	await page.getByLabelText('Required escort strength (%)').fill('125');
	await page.getByRole('button', { name: 'Save thresholds' }).click();
	const settings = get(context.player).playerOrders!.automation!;
	expect(settings.passengerTransport).toBe(true);
	expect(settings.sourcePopulationPercent).toBe(85);
	expect(settings.destinationPopulationPercent).toBe(40);
	expect(settings.escortMarginLy).toBe(150);
	expect(settings.escortStrengthPercent).toBe(125);
});

it('sorts fleet rows by displayed automatic role in both directions', async () => {
	const original = get(context.universe).fleets[0];
	context.universe.set({
		fleets: [
			original,
			create(FleetSchema, {
				...original,
				mapObject: create(MapObjectSchema, { num: 2, playerNum: 1, name: 'Scout fleet' })
			}),
			create(FleetSchema, {
				...original,
				mapObject: create(MapObjectSchema, { num: 3, playerNum: 1, name: 'Manual fleet' })
			})
		]
	});
	get(context.player).playerOrders!.automation!.fleetRoles = { 1: 'tanker', 2: 'scout' };
	render(FleetAutomation);
	const rows = () =>
		Array.from(
			document.querySelectorAll('section[aria-label="Fleet automation"] table:last-child tbody tr')
		)
			.filter((row) => row.querySelector('select[aria-label^="Role for"]'))
			.map((row) => row.children[1].textContent);
	await page.getByRole('button', { name: /Automatic role/ }).click();
	expect(rows()).toEqual(['Manual fleet', 'Scout fleet', 'Transport']);
	expect(document.querySelector('th[aria-sort]')?.getAttribute('aria-sort')).toBe('ascending');
	await page.getByRole('button', { name: /Automatic role/ }).click();
	expect(rows()).toEqual(['Transport', 'Scout fleet', 'Manual fleet']);
});

it('offers automatic patrol to an armed fleet', async () => {
	render(FleetAutomation);
	await page.getByRole('combobox', { name: 'Role for Transport' }).selectOptions('patrol');
	expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({ 1: 'patrol' });
});
it('keeps future terraforming and opportunistic installation transport as separate permissions', async () => {
	render(FleetAutomation);
	await page.getByRole('checkbox', { name: 'Serve worlds requiring terraforming' }).click();
	await page
		.getByRole('checkbox', { name: 'Carry surplus installations on existing routes' })
		.click();
	expect(get(context.player).playerOrders!.automation!.serveTerraformingWorlds).toBe(true);
	expect(get(context.player).playerOrders!.automation!.infrastructureTransport).toBe(true);
});

it('checks all eligible rebuilds without issuing the order until requested', async () => {
	render(FleetAutomation);
	await page.getByRole('button', { name: 'Check all for scrap and rebuild', exact: true }).click();
	await expect
		.element(page.getByRole('button', { name: 'Rebuild selected (1)', exact: true }))
		.toBeEnabled();
	expect(context.updatePlayerOrders).not.toHaveBeenCalled();
	await page.getByRole('button', { name: 'Rebuild selected (1)', exact: true }).click();
	expect(get(context.player).playerOrders!.automation!.rebuildFleets).toEqual({ 1: true });
});

it('sorts rebuilding fleets before other fleets', async () => {
	const first = get(context.universe).fleets[0];
	const rebuilding = create(FleetSchema, {
		...first,
		mapObject: create(MapObjectSchema, {
			num: 2,
			playerNum: 1,
			name: 'Rebuilding fleet',
			tags: { 'design-maintenance': '{}' }
		})
	});
	context.universe.set({ fleets: [first, rebuilding] });
	render(FleetAutomation);
	await page.getByRole('button', { name: /^Rebuilding/ }).click();
	const rows = Array.from(
		document.querySelectorAll('section[aria-label="Fleet automation"] tbody tr')
	).filter((row) => row.querySelector('select[aria-label^="Role for"]'));
	expect(rows[0]?.textContent).toContain('Rebuilding fleet');
});

it('does not offer Scout to an armed fleet without scanners', async () => {
	render(FleetAutomation);
	expect(
		page
			.getByRole('combobox', { name: 'Role for Transport' })
			.getByRole('option', { name: 'Scout: exploration and incoming fleet reconnaissance' })
			.query()
	).toBeNull();
});

it('saves editable transport policy and retains other automation settings', async () => {
	render(FleetAutomation);
	await page.getByLabelText('Lower-habitability cutoff (%)').fill('60');
	await page.getByLabelText('Lower-habitability population target and reserve (%)').fill('45');
	await page.getByLabelText('Captured colony population target (%)').fill('85');
	await page.getByLabelText('Defenses to retain at source (units)').fill('70');
	await page.getByLabelText('Factory and mine operating reserve (%)').fill('90');
	await page.getByLabelText('Minimum weakening landing (people)').fill('300000');
	await page.getByLabelText('Patrol ships for landing assembly').fill('4');
	await page
		.getByRole('checkbox', {
			name: 'Allow large landings that weaken defenders without capturing the planet'
		})
		.click();
	await page.getByRole('button', { name: 'Save transport policy', exact: true }).click();
	const policy = get(context.player).playerOrders!.automation!.transportPolicy!;
	expect(policy.colonyHabitabilityCutoff).toBe(60);
	expect(policy.colonyPopulationTarget).toBe(45);
	expect(policy.capturedPopulationTarget).toBe(85);
	expect(policy.defenseReserve).toBe(70);
	expect(policy.installationReservePercent).toBe(90);
	expect(policy.attritionMinimumPeople).toBe(300000);
	expect(policy.landingEscortShips).toBe(4);
	expect(policy.disableAttrition).toBe(true);
	await page.getByRole('button', { name: 'Reset transport policy to defaults' }).click();
	await page.getByRole('button', { name: 'Save transport policy', exact: true }).click();
	expect(get(context.player).playerOrders!.automation!.transportPolicy!.defenseReserve).toBe(80);
	expect(get(context.player).playerOrders!.automation!.transportPolicy!.disableAttrition).toBe(
		false
	);
});

it('rejects invalid transport policy values before saving', async () => {
	render(FleetAutomation);
	await page.getByLabelText('Minimum weakening landing (people)').fill('12345');
	await expect
		.element(page.getByRole('button', { name: 'Save transport policy', exact: true }))
		.toBeDisabled();
});
