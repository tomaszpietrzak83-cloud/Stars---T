import {create} from '@bufbuild/protobuf';
import {writable} from 'svelte/store';
import {expect,it,vi} from 'vitest';
import {page} from 'vitest/browser';
import {render} from 'vitest-browser-svelte';
import {BattleRecordSchema,PlayerMessageSchema,PlayerMessageType,TechCategory} from '#lib/types/cs-proto.js';
import PlayerMessageDetail from './PlayerMessageDetail.svelte';
const navigation = vi.hoisted(() => ({ goto: vi.fn(), gotoBattle: vi.fn() }));
vi.mock('$app/navigation', () => ({ goto: navigation.goto }));
vi.mock('#lib/services/GameContext.js', () => ({
	getGameContext: () => ({
		game: writable({ id: 7n }),
		player: writable({ num: 1, getAllies: () => [1] }),
		universe: writable({
			battleRecords: [7, 9].map((num) =>
				create(BattleRecordSchema, { num, stats: { shipsDestroyedByPlayer: { 2: 1 } } })
			),
			getBattleLocation: () => 'Space'
		}),
		gotoBattle: navigation.gotoBattle
	})
}));
vi.mock('#lib/services/Stores.js',()=>({tooltipComponent:writable(undefined),showTooltip:vi.fn(),techs:writable({getTech:()=>({tech:{category:TechCategory.ENGINE}})})}));
it('offers a hold preview for an invention without a navigation link',async()=>{
 render(PlayerMessageDetail,{message:create(PlayerMessageSchema,{type:PlayerMessageType.PLAYER_TECH_GAINED,spec:{techGained:'EEE Galactic Drive',field:'Propulsion'}})});
 await expect.element(page.getByRole('button',{name:'Preview EEE Galactic Drive'})).toHaveAttribute('title','Hold to preview');
 expect(page.getByRole('link').query()).toBeNull();
});

it('offers a separate map action and replay link for each noteworthy battle', async () => {
	render(PlayerMessageDetail, {
		message: create(PlayerMessageSchema, { type: PlayerMessageType.BATTLE_REPORTS })
	});

	await expect
		.element(page.getByRole('link', { name: /^Battle 7:/ }))
		.toHaveAttribute('href', '/games/7/battles/7');
	await page.getByRole('button', { name: 'Go! Show battle 9 on the map' }).click();
	expect(navigation.goto).toHaveBeenCalledWith('/games/7');
	expect(navigation.gotoBattle).toHaveBeenCalledWith(9, false);
});
