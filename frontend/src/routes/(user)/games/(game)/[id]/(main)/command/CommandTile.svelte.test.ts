import { beforeEach, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { createRawSnippet } from 'svelte';
import CommandTile from './CommandTile.svelte';

beforeEach(() => localStorage.clear());

it('collapses content, keeps its header and restores the preference for another planet', async () => {
	const children = createRawSnippet(() => ({ render: () => '<p>Planet illustration</p>' }));
	const first = render(CommandTile, { title: 'Earth', sectionKey: 'planet-overview', children });
	await expect.element(page.getByText('Planet illustration')).toBeVisible();
	await page.getByRole('button', { name: 'Collapse Earth' }).click();
	await expect.element(page.getByText('Planet illustration')).not.toBeVisible();
	await expect.element(page.getByRole('button', { name: 'Expand Earth' })).toHaveAttribute('aria-expanded', 'false');
	await first.unmount();
	render(CommandTile, { title: 'Mars', sectionKey: 'planet-overview', children });
	await expect.element(page.getByText('Planet illustration')).not.toBeVisible();
	await page.getByRole('button', { name: 'Expand Mars' }).click();
	await expect.element(page.getByText('Planet illustration')).toBeVisible();
});

it('remembers each section independently', async () => {
	render(CommandTile, { title: 'Starbase', sectionKey: 'planet-starbase' });
	render(CommandTile, { title: 'Fleets In Orbit' });
	await page.getByRole('button', { name: 'Collapse Starbase' }).click();
	await expect.element(page.getByRole('button', { name: 'Collapse Fleets In Orbit' })).toHaveAttribute('aria-expanded', 'true');
	expect(localStorage.getItem('tomek-stars.command-section.planet-starbase')).toBe('collapsed');
});
