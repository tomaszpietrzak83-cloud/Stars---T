import { create } from '@bufbuild/protobuf';
import { expect, it } from 'vitest';
import { FleetSchema, ShipDesignSpecSchema } from './cs-proto.js';
import { automationRoles, automationStatus, dedicatedMinesweeper, designAutoRole } from './automationRoles.js';

it('shows interrupted sweeping as paused and keeps the measured result separate', () => {
	const fleet = create(FleetSchema, { mapObject: { tags: {
		'automatic-safety': JSON.stringify({ status: 'Mission paused: predicted hostile contact' }),
		'automatic-fuel-status': 'Approaching a field',
		'patrol-mine-sweep': 'Inside reported field',
		'last-mine-sweep': 'Year 2508 result: 0 mines removed from 0 fields'
	} } });
	expect(automationStatus(fleet)).toContain('Mission paused');
	expect(automationStatus(fleet)).not.toContain('Approaching');
	expect(automationStatus(fleet)).toContain('Year 2508 result: 0 mines removed');
});

const spec = () =>
	create(ShipDesignSpecSchema, {
		hasWeapons: true,
		mineSweep: 992,
		weaponSlots: [
			{ hullComponent: 'Gatling Gun', quantity: 1 },
			{ hullComponent: 'Gatling Gun', quantity: 1 }
		]
	});
it('classifies equipment, defaults to mine clearing and excludes combat roles', () => {
	const s = spec();
	expect(dedicatedMinesweeper(s)).toBe(true);
	expect(designAutoRole(s)).toBe('minesweeper');
	expect(
		automationRoles(create(FleetSchema, { spec: { shipDesignSpec: s } })).map((r) => r.value)
	).toEqual(['', 'minesweeper']);
	s.defaultAutoRole = 'manual';
	expect(designAutoRole(s)).toBe('');
});
it('does not classify mixed armament, bombs or an unarmed design as dedicated', () => {
	const mixed = spec();
	mixed.weaponSlots.push({
		$typeName: 'craig_stars.v1.ShipDesignSlot',
		hullComponent: 'X-Ray Laser',
		hullSlotIndex: 4,
		quantity: 1
	});
	expect(dedicatedMinesweeper(mixed)).toBe(false);
	const bombs = spec();
	bombs.bombs.push({
		$typeName: 'craig_stars.v1.Bomb',
		quantity: 1,
		killRate: 1,
		minKillRate: 0,
		structureDestroyRate: 0,
		unterraformRate: 0
	});
	expect(dedicatedMinesweeper(bombs)).toBe(false);
	expect(dedicatedMinesweeper(create(ShipDesignSpecSchema, { mineSweep: 50 }))).toBe(false);
});
