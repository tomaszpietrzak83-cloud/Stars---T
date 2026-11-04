import '../../../../css/app.css';
import type { Battle } from '#lib/types/Battle.js';
import { create } from '@bufbuild/protobuf';
import { BattleRecordTokenActionSchema, BattleRecordTokenActionType } from '#lib/types/cs-proto.js';
import { expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import BattleBoardDamage from './BattleBoardDamage.svelte';

it('renders per-attack losses in yellow with armor and shield damage', () => {
	const battle = {
		actions: [
			create(BattleRecordTokenActionSchema, {
				type: BattleRecordTokenActionType.BEAM_FIRE,
				tokenNum: 1,
				round: 1,
				slot: 1,
				to: { x: 2, y: 3 },
				damageDoneArmor: 500,
				damageDoneShields: 100,
				tokensDestroyed: 4
			})
		]
	} as Battle;
	const { container, unmount } = render(BattleBoardDamage, { battle, phase: 1, cell: 64 });
	const loss = container.querySelector<HTMLElement>('[data-kind="destroyed"]')!;
	expect(loss.textContent?.trim()).toBe('-4');
	expect(getComputedStyle(loss).color).toBe('rgb(255, 255, 0)');
	expect(loss.getAttribute('aria-label')).toBe('4 ships destroyed');
	expect(container.querySelectorAll('.battle-damage-mark')).toHaveLength(3);
	expect(container.querySelector('[data-kind="armor"]')?.textContent?.trim()).toBe('-500');
	expect(container.querySelector('[data-kind="shield"]')?.textContent?.trim()).toBe('-100');
	unmount();
	const empty = render(BattleBoardDamage, { battle, phase: 0, cell: 64 });
	expect(empty.container.querySelector('[data-kind="destroyed"]')).toBeNull();
	empty.unmount();
});

it('keeps the yellow count inside the bottom row when enlarged on phones', () => {
	const host = document.createElement('div');
	host.style.cssText = 'position:relative;width:690px;height:690px;overflow:hidden';
	document.body.appendChild(host);
	const battle = {
		actions: [
			create(BattleRecordTokenActionSchema, {
				type: BattleRecordTokenActionType.BEAM_FIRE,
				to: { x: 5, y: 9 },
				damageDoneArmor: 500,
				damageDoneShields: 100,
				tokensDestroyed: 4
			})
		]
	} as Battle;
	const view = render(
		BattleBoardDamage,
		{ battle, phase: 1, cell: 69, markScale: 2 },
		{ baseElement: host }
	);
	const mark = view.container.querySelector('[data-kind="destroyed"]')!;
	expect(mark.getBoundingClientRect().bottom).toBeLessThanOrEqual(
		host.getBoundingClientRect().bottom
	);
	view.unmount();
	host.remove();
});
