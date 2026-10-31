import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';
import { FleetSchema } from './cs-proto';
import { CommandedFleet, getEta, getObservedSpeed, getStrategicSpeed } from './Fleet';
import type { Universe } from '#lib/services/Universe.js';

describe('strategic travel speed', () => {
	const universe = { getWormhole: () => undefined } as Pick<Universe, 'getWormhole'>;
	const fleet = (factor = 0) => new CommandedFleet(create(FleetSchema, {
		warpSpeed: 10,
		spec: { shipDesignSpec: { engine: { strategicSpeedFactor: factor } } },
		fleetOrders: { waypoints: [{ position: { x: 0, y: 0 } }, { position: { x: 6000, y: 0 }, warpSpeed: 10 }] }
	}));

	it.each([[0, 60], [1.5, 40], [2, 30], [3, 20]])('shows actual ETA at factor %s', (factor, years) => {
		const f = fleet(factor);
		expect(f.getYearsForLeg(universe, 1)).toBe(years);
		expect(getEta(f)).toBe(years);
		expect(getStrategicSpeed(f, 8)).toBe(64);
		expect(getStrategicSpeed(f, 11)).toBe(0);
	});

	it('uses observed velocity without exposing an enemy engine', () => {
		const intel = create(FleetSchema, { warpSpeed: 10, observedSpeed: 300 });
		expect(intel.spec).toBeUndefined();
		expect(getObservedSpeed(intel)).toBe(300);
		intel.warpSpeed = 11;
		expect(getObservedSpeed(intel)).toBe(0);
	});

	it('leaves stargate ETA as one year', () => {
		const f = fleet(3);
		f.fleetOrders.waypoints[1].warpSpeed = 11;
		expect(f.getYearsForLeg(universe, 1)).toBe(1);
		expect(getEta(f)).toBe(1);
	});
});
