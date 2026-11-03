import '../../../../../../../css/app.css';
import { create } from '@bufbuild/protobuf';
import { PlanetSchema, MapObjectType } from '#lib/types/cs-proto.js';
import { Universe } from '#lib/services/Universe.js';
import { CommandedPlayer } from '#lib/types/Player.js';
import { PlanetViewState, PlayerSettings } from '#lib/types/PlayerSettings.js';
import { writable } from 'svelte/store';
import { expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ScannerCanvas from './ScannerCanvas.svelte';

const state = vi.hoisted(() => ({ context: undefined as unknown }));
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: () => state.context }));

it('renders sky-blue resource discs with proportional areas on the real map canvas', async () => {
	const universe = new Universe();
	universe.planets = [1000, 500, 250].map((resources, i) =>
		create(PlanetSchema, {
			mapObject: {
				type: MapObjectType.PLANET,
				num: i + 1,
				playerNum: 1,
				position: { x: 40 + i * 60, y: 50 }
			},
			spec: { resourcesPerYear: resources }
		})
	);
	universe.planetIntels = universe.planets;
	universe.planets[0].spec!.planetStarbaseSpec = {
		$typeName: 'craig_stars.v1.PlanetStarbaseSpec',
		hasStarbase: true,
		dockCapacity: 100
	} as never;
	universe.planets[0].cargo = {
		$typeName: 'craig_stars.v1.Cargo',
		ironium: 6000,
		boranium: 2500,
		germanium: 1
	} as never;
	universe.resetAllPlanets();
	universe.resetMapObjectsByPosition();
	const player = new CommandedPlayer();
	player.num = 1;
	const settings = new PlayerSettings();
	settings.planetViewState = PlanetViewState.Resources;
	settings.showScanners = settings.showAllyScanners = false;
	state.context = {
		player: writable(player),
		universe: writable(universe),
		settings: writable(settings),
		commandedFleet: writable(undefined),
		commandedPlanet: writable(undefined),
		commandedMapObject: writable(undefined),
		selectedMapObject: writable(undefined),
		selectedWaypoint: writable(undefined)
	};
	const { container } = render(ScannerCanvas, {
		transform: undefined,
		pixelsPerLightYear: { x: 2, y: 2 },
		minObjectZoom: 1,
		locate: undefined
	});
	const bounds = container.firstElementChild as HTMLElement;
	bounds.style.width = '400px';
	bounds.style.height = '200px';
	const canvas = container.querySelector<HTMLCanvasElement>('canvas')!;
	const bluePixels = () =>
		[80, 200, 320].map((x) => {
			const dpr = window.devicePixelRatio;
			const data = canvas
				.getContext('2d')!
				.getImageData((x - 23) * dpr, 77 * dpr, 46 * dpr, 46 * dpr).data;
			let count = 0;
			for (let i = 0; i < data.length; i += 4)
				if (data[i] === 135 && data[i + 1] === 206 && data[i + 2] === 250 && data[i + 3] === 255)
					count++;
			return count;
		});
	await expect.poll(() => bluePixels()[0]).toBeGreaterThan(300);
	const counts = bluePixels();
	expect(counts[1] / counts[0]).toBeCloseTo(0.5, 1);
	expect(counts[2] / counts[0]).toBeCloseTo(0.25, 1);
	const pixelsOf = (x: number, y: number, width: number, height: number, rgb: number[]) => {
		const dpr = window.devicePixelRatio;
		const pixels = canvas
			.getContext('2d')!
			.getImageData(x * dpr, y * dpr, width * dpr, height * dpr).data;
		let count = 0;
		for (let i = 0; i < pixels.length; i += 4)
			if (
				pixels[i] === rgb[0] &&
				pixels[i + 1] === rgb[1] &&
				pixels[i + 2] === rgb[2] &&
				pixels[i + 3] === 255
			)
				count++;
		return count;
	};
	expect(pixelsOf(68, 54, 24, 21, [253, 253, 0])).toBeGreaterThan(10);
	expect(pixelsOf(104, 76, 27, 44, [91, 140, 255])).toBeGreaterThan(50);
	expect(pixelsOf(104, 76, 27, 44, [84, 207, 120])).toBeGreaterThan(25);
	expect(pixelsOf(104, 76, 27, 44, [255, 216, 94])).toBeGreaterThan(0);
});
