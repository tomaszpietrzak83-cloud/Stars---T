import { create } from '@bufbuild/protobuf';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { me } from '#lib/services/Stores.js';
import { UserSession } from '#lib/types/User.js';
import { GameWithPlayersSchema, GameState, UserRole } from '#lib/types/cs-proto.js';
import Games from './Games.svelte';

const { getGames, getGame, addError } = vi.hoisted(() => ({
	getGames: vi.fn(), getGame: vi.fn(), addError: vi.fn()
}));
vi.mock('#lib/services/connect.js', () => ({ gameClient: { getGames, getGame } }));
vi.mock('#lib/services/Errors.js', () => ({ addError }));

beforeEach(() => {
	vi.clearAllMocks();
	me.set(Object.assign(new UserSession(), { id: 1n, role: UserRole.USER }));
});

const saved = create(GameWithPlayersSchema, {
	game: { id: 42n, hostId: 1n, name: 'Saved galaxy', year: 2450, state: GameState.WAITING_FOR_PLAYERS },
	players: [{ userId: 1n }]
});

it('shows saved games while public lobbies are still pending, without loading a game', async () => {
	getGames.mockImplementation(({ open }) => open ? new Promise(() => {}) : Promise.resolve({ games: [saved] }));
	render(Games);
	const link = page.getByRole('link', { name: 'Saved galaxy', exact: true });
	await expect.element(link).toBeVisible();
	await expect.element(link).toHaveAttribute('href', '/games/42');
	await link.hover();
	expect(getGame).not.toHaveBeenCalled();
	await expect.element(page.getByLabelText('Loading games')).not.toBeInTheDocument();
});

it('keeps saved games visible when public lobbies fail', async () => {
	getGames.mockImplementation(({ open }) => open ? Promise.reject(new Error('Lobby unavailable')) : Promise.resolve({ games: [saved] }));
	render(Games);
	await expect.element(page.getByRole('link', { name: 'Saved galaxy', exact: true })).toBeVisible();
	await expect.element(page.getByText('Unable to load games.')).not.toBeInTheDocument();
	expect(addError).toHaveBeenCalledOnce();
});
