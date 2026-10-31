import { expect, it } from 'vitest';
import { Size, Density } from '#lib/types/cs-proto.js';
import { recommendedEmpires, estimatedStars } from './mapPopulation';
import { getFirstAvailableColor } from './playerColors';

it('predicts planet counts with the generator integer density rounding',()=>{
	expect(estimatedStars(Size.SMALL,Density.NORMAL)).toBe(128);
	expect(estimatedStars(Size.VAST,Density.NORMAL)).toBe(1568);
	expect(estimatedStars(Size.IMMENSE,Density.NORMAL)).toBe(2048);
	expect(estimatedStars(Size.COLOSSAL,Density.PACKED)).toBe(5600);
	expect(estimatedStars(Size.TITANIC,Density.PACKED)).toBe(8750);
	expect(estimatedStars(Size.GALACTIC,Density.PACKED)).toBe(12600);
	expect(estimatedStars(Size.TINY_WIDE,Density.SPARSE)).toBe(23);
});

it('offers proportional empire populations on expanded maps', () => {
	expect(recommendedEmpires(Size.VAST)).toBe(32);
	expect(recommendedEmpires(Size.IMMENSE)).toBe(40);
	expect(recommendedEmpires(Size.COLOSSAL)).toBe(64);
});

it('assigns 64 valid distinct empire colors', () => {
	const used = new Set<string>();
	for (let i=0;i<64;i++) {
		const color=getFirstAvailableColor(used);
		expect(color).toMatch(/^#[0-9a-f]{6}$/i);
		expect(used.has(color)).toBe(false);
		used.add(color);
	}
});
