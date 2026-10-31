import { create } from '@bufbuild/protobuf';
import { expect, it } from 'vitest';
import { ShipDesignSchema, TechSchema, TechCategory } from './types/cs-proto';
import { getHullIcon, getTechIcon } from './techicon';

it('gives every new hull and engine its own icon instead of a legacy alias', () => {
	for (const name of ['Pathfinder', 'Bulk Carrier', 'Fleet Tender', 'Expedition Cruiser', 'Linebreaker', 'Expedition Bomber', 'Prospector']) {
		const hull = `EEE ${name}`;
		for (let set = 0; set < 4; set++) {
			const expected = `hull-eee-${name.toLowerCase().replaceAll(' ', '-')}-${set}`;
			expect(getHullIcon(create(ShipDesignSchema, { hull, hullSetNumber: set }))).toBe(expected);
			expect(getTechIcon({ tech: create(TechSchema, { name: hull, category: TechCategory.SHIP_HULL }) }, set)).toBe(expected);
		}
	}
	for (const name of ['Horizon Drive', 'Deep Space Drive', 'Galactic Drive']) {
		expect(getTechIcon({ tech: create(TechSchema, { name: `EEE ${name}` }) }, 0)).toBe(`eee-${name.toLowerCase().replaceAll(' ', '-')}`);
	}
});
