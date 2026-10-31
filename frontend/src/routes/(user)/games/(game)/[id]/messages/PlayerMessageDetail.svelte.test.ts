import {create} from '@bufbuild/protobuf';
import {writable} from 'svelte/store';
import {expect,it,vi} from 'vitest';
import {page} from 'vitest/browser';
import {render} from 'vitest-browser-svelte';
import {PlayerMessageSchema,PlayerMessageType,TechCategory} from '#lib/types/cs-proto.js';
import PlayerMessageDetail from './PlayerMessageDetail.svelte';
vi.mock('#lib/services/GameContext.js',()=>({getGameContext:()=>({game:writable({id:7n}),player:writable({num:1}),universe:writable({})})}));
vi.mock('#lib/services/Stores.js',()=>({tooltipComponent:writable(undefined),showTooltip:vi.fn(),techs:writable({getTech:()=>({tech:{category:TechCategory.ENGINE}})})}));
it('offers a hold preview for an invention without a navigation link',async()=>{
 render(PlayerMessageDetail,{message:create(PlayerMessageSchema,{type:PlayerMessageType.PLAYER_TECH_GAINED,spec:{techGained:'EEE Galactic Drive',field:'Propulsion'}})});
 await expect.element(page.getByRole('button',{name:'Preview EEE Galactic Drive'})).toHaveAttribute('title','Hold to preview');
 expect(page.getByRole('link').query()).toBeNull();
});
