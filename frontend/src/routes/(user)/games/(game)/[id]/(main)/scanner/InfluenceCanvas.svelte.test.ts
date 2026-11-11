import { Universe } from '#lib/services/Universe.js';
import { ReportAgeUnexplored } from '#lib/types/Consts.js';
import type { Planet } from '#lib/types/cs-proto.js';
import { describe, expect, it, vi } from 'vitest';
import { buildInfluenceLayer } from './canvas/influence';
import { ScannerView } from './canvas/view';

function renderInfluence(colonies: [number, number, number, number?][]) {
	const universe = new Universe();
	universe.planetIntels = colonies.map(([x, y, playerNum, reportAge], index) => ({
		mapObject: { num: index + 1, playerNum, position: { x, y }, reportAge }
	}) as Planet);
	vi.spyOn(universe, 'getPlayerColor').mockImplementation((owner) =>
		owner === 1 ? '#ff0000' : '#0000ff'
	);
	const canvas = document.createElement('canvas');
	canvas.width = canvas.height = 400;
	const ctx = canvas.getContext('2d')!;
	buildInfluenceLayer(universe)?.draw(ctx, new ScannerView(1, 1, 1, 200, 200, 400, 400, 1, 1));
	return (x: number, y: number) => ctx.getImageData(x + 200, y + 200, 1, 1).data;
}

describe('territorial influence', () => {
	it('joins and strengthens overlapping colonies of the same owner', () => {
		const single = renderInfluence([[-65, 0, 1]]);
		const pair = renderInfluence([[-65, 0, 1], [65, 0, 1]]);
		expect(single(0, 0)[3]).toBeGreaterThan(0);
		expect(pair(0, 0)[3]).toBeGreaterThan(single(0, 0)[3]);
	});

	it('compresses the area of a colony surrounded by another owner', () => {
		const isolated = renderInfluence([[0, 0, 2]]);
		const surrounded = renderInfluence([[0, 0, 2], [-40, 0, 1], [40, 0, 1], [0, 40, 1]]);
		expect(isolated(0, -25)[2]).toBe(255);
		expect(surrounded(0, -25)[0]).toBe(255);
		expect(surrounded(0, 0)[2]).toBe(255);
		expect(surrounded(0, 0)[3]).toBeLessThan(isolated(0, 0)[3]);
	});

	it('ignores unclaimed planets and unexplored reports', () => {
		const sample = renderInfluence([[0, 0, 0], [30, 0, 2, ReportAgeUnexplored]]);
		expect(sample(0, 0)[3]).toBe(0);
		expect(sample(30, 0)[3]).toBe(0);
	});
});
