import type { Universe } from '#lib/services/Universe.js';
import { ReportAgeUnexplored } from '#lib/types/Consts.js';
import type { ScannerView } from './view';

const reach = 105; // light years
const core = 23;
type Colony = { x: number; y: number };

export function buildInfluenceLayer(universe: Universe) {
	const groups = new Map<number, Colony[]>();
	let left = Infinity;
	let top = Infinity;
	let right = -Infinity;
	let bottom = -Infinity;

	for (const intel of universe.planetIntels) {
		const current = universe.getPlanet(intel.mapObject?.num);
		const object = (current ?? intel).mapObject;
		const owner = object?.playerNum ?? 0;
		if (
			!object?.position ||
			owner <= 0 ||
			(owner !== universe.playerNum && object.reportAge === ReportAgeUnexplored)
		) {
			continue;
		}

		const x = Number(object.position.x);
		const y = Number(object.position.y);
		if (!Number.isFinite(x) || !Number.isFinite(y)) continue;
		const colonies = groups.get(owner) ?? [];
		colonies.push({ x, y });
		groups.set(owner, colonies);
		left = Math.min(left, x - reach);
		right = Math.max(right, x + reach);
		top = Math.min(top, y - reach);
		bottom = Math.max(bottom, y + reach);
	}
	if (!groups.size) return;

	// Sample in world coordinates once, so panning and zooming only move and scale the image.
	const step = Math.max(2, (right - left) / 1024, (bottom - top) / 1024);
	const width = Math.ceil((right - left) / step);
	const height = Math.ceil((bottom - top) / step);
	const canvas = document.createElement('canvas');
	canvas.width = width;
	canvas.height = height;
	const ctx = canvas.getContext('2d');
	if (!ctx) return;

	const strength = new Float32Array(width * height);
	const strongest = new Float32Array(strength.length);
	const runnerUp = new Float32Array(strength.length);
	const winner = new Uint16Array(strength.length);
	const palette: Uint8ClampedArray[] = [];
	for (const [owner, colonies] of groups) {
		ctx.fillStyle = '#ff0000';
		ctx.fillStyle = universe.getPlayerColor(owner);
		ctx.fillRect(0, 0, 1, 1);
		palette.push(ctx.getImageData(0, 0, 1, 1).data);
		strength.fill(0);

		for (const { x, y } of colonies) {
			const firstRow = Math.max(0, Math.floor((y - reach - top) / step));
			const lastRow = Math.min(height, Math.ceil((y + reach - top) / step));
			const firstCol = Math.max(0, Math.floor((x - reach - left) / step));
			const lastCol = Math.min(width, Math.ceil((x + reach - left) / step));
			for (let row = firstRow; row < lastRow; row++) {
				const dy = top + (row + 0.5) * step - y;
				for (let col = firstCol; col < lastCol; col++) {
					const dx = left + (col + 0.5) * step - x;
					const distanceSquared = dx * dx + dy * dy;
					const broad = Math.max(0, 1 - distanceSquared / (reach * reach));
					const local = Math.max(0, 1 - distanceSquared / (core * core));
					strength[row * width + col] += broad * broad + 2.5 * local * local;
				}
			}
		}

		for (let i = 0; i < strength.length; i++) {
			if (strength[i] > strongest[i]) {
				runnerUp[i] = strongest[i];
				strongest[i] = strength[i];
				winner[i] = palette.length - 1;
			} else {
				runnerUp[i] = Math.max(runnerUp[i], strength[i]);
			}
		}
	}

	const pixels = ctx.createImageData(width, height);
	for (let i = 0; i < strength.length; i++) {
		// A lead over the next player determines opacity, fading to a soft contested boundary.
		const lead = Math.max(0, strongest[i] - runnerUp[i] - 0.12);
		if (!lead) continue;
		const color = palette[winner[i]];
		const offset = i * 4;
		pixels.data[offset] = color[0];
		pixels.data[offset + 1] = color[1];
		pixels.data[offset + 2] = color[2];
		pixels.data[offset + 3] = (160 * lead) / (lead + 0.65);
	}
	ctx.putImageData(pixels, 0, 0);

	return {
		draw(target: CanvasRenderingContext2D, view: ScannerView) {
			const x = view.x(left);
			const y = view.y(top);
			target.drawImage(canvas, x, y, view.x(left + width * step) - x, view.y(top + height * step) - y);
		}
	};
}
