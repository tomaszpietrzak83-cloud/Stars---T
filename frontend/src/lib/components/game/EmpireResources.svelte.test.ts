import { expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { create } from '@bufbuild/protobuf';
import { PlanetSchema } from '#lib/types/cs-proto.js';
import EmpireResources from './EmpireResources.svelte';

it('totals only owned production and research including production leftovers', async () => {
	const planets = [
		create(PlanetSchema, {
			mapObject: { playerNum: 1 },
			spec: {
				resourcesPerYear: 1000,
				resourcesPerYearResearch: 200,
				resourcesPerYearResearchEstimatedLeftover: 30
			}
		}),
		create(PlanetSchema, {
			mapObject: { playerNum: 1 },
			spec: {
				resourcesPerYear: 500,
				resourcesPerYearResearch: 0,
				resourcesPerYearResearchEstimatedLeftover: 50
			}
		}),
		create(PlanetSchema, {
			mapObject: { playerNum: 2 },
			spec: { resourcesPerYear: 9000, resourcesPerYearResearch: 4000 }
		})
	];
	render(EmpireResources, { planets, playerNum: 1, researchHref: '/games/1/research' });
	await expect.element(page.getByText('Resources 1,500')).toBeVisible();
	const research = page.getByRole('link', { name: 'Research 280' });
	await expect.element(research).toHaveAttribute('href', '/games/1/research');
	await expect.element(research).toHaveClass(/text-blue-600/);
});

it('shows zero for an empty empire', async () => {
	render(EmpireResources, { planets: [], playerNum: 1, researchHref: '/games/1/research' });
	await expect.element(page.getByText('Resources 0')).toBeVisible();
	await expect.element(page.getByRole('link', { name: 'Research 0' })).toBeVisible();
});
