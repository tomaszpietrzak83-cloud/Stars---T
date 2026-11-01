import { create } from '@bufbuild/protobuf';
import { writable } from 'svelte/store';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { BattlePlanSchema, BattleTactic, BattleTarget, PlayerSchema, PlayerPlansSchema, ShipDesignSpecSchema } from '#lib/types/cs-proto.js';
import DesignBattleStrategy from './DesignBattleStrategy.svelte';

const state = vi.hoisted(() => ({ update: vi.fn(), create: vi.fn() }));
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: () => ({
 player: writable(create(PlayerSchema, { playerPlans: create(PlayerPlansSchema, { battlePlans: [create(BattlePlanSchema, { num: 3, name: 'Sniper', tactic: BattleTactic.MAXIMIZE_RANGE, primaryTarget: BattleTarget.ARMED_SHIPS, secondaryTarget: BattleTarget.ANY })] }) })),
 updateBattlePlan: state.update, createBattlePlan: state.create
}) }));
beforeEach(() => { state.update.mockReset().mockImplementation(async p => p); state.create.mockReset().mockImplementation(async p => ({...p,num:7})); });
it('opens strategy editing inline and returns without navigation', async () => {
 render(DesignBattleStrategy, { selection: 4, spec: create(ShipDesignSpecSchema, { hasWeapons: true }) });
 await page.getByRole('button', { name: 'Preview / edit strategy' }).click();
 await expect.element(page.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Sniper');
 await page.getByRole('textbox', { name: 'Name', exact: true }).fill('Long range');
 await page.getByRole('button', { name: 'Save strategy and return' }).click();
 expect(state.update).toHaveBeenCalledWith(expect.objectContaining({ name: 'Long range', tactic: BattleTactic.MAXIMIZE_RANGE }));
 await expect.element(page.getByRole('button', { name: 'Preview / edit strategy' })).toBeVisible();
 expect(page.getByRole('button', { name: 'Save strategy and return' }).query()).toBeNull();
});
it('previews automatic Chicken and allows returning without saving', async () => {
 render(DesignBattleStrategy, { spec: create(ShipDesignSpecSchema) });
 await expect.element(page.getByRole('option', { name: 'Automatic: Chicken' })).toBeInTheDocument();
 await page.getByRole('button', { name: 'Preview / edit strategy' }).click();
 await expect.element(page.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Chicken');
 await page.getByRole('button', { name: 'Back to ship design' }).click();
 expect(state.update).not.toHaveBeenCalled(); expect(state.create).not.toHaveBeenCalled();
});
