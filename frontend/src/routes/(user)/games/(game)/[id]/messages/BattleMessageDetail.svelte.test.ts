import {create} from '@bufbuild/protobuf';
import {writable} from 'svelte/store';
import {beforeEach,expect,it,vi} from 'vitest';
import {page} from 'vitest/browser';
import {render} from 'vitest-browser-svelte';
import {BattleRecordSchema,PlayerMessageSchema,PlayerMessageType} from '#lib/types/cs-proto.js';
import BattleMessageDetail from './BattleMessageDetail.svelte';
let recording=true;
vi.mock('#lib/services/GameContext.js',()=>({getGameContext:()=>({game:writable({id:6n}),player:writable({num:1}),universe:writable({getBattle:()=>recording?create(BattleRecordSchema,{num:3}):undefined})})}));
vi.mock('#lib/types/Battle.js',()=>({getBattleRecordDetails:()=>({location:'Home',ours:2,theirs:1,ourDead:0,theirDead:1})}));
beforeEach(()=>{recording=true;});
it('links directly to the battle replay even when the notification has custom text',async()=>{
 render(BattleMessageDetail,{message:create(PlayerMessageSchema,{type:PlayerMessageType.BATTLE,battleNum:3,text:'A battle took place near Home.'})});
 await expect.element(page.getByRole('link',{name:'Watch battle',exact:true})).toHaveAttribute('href','/games/6/battles/3');
});
it('does not offer a replay when the recording is unavailable',async()=>{
 recording=false;
 render(BattleMessageDetail,{message:create(PlayerMessageSchema,{type:PlayerMessageType.BATTLE,battleNum:3,text:'Old battle report.'})});
 expect(page.getByRole('link',{name:'Watch battle',exact:true}).query()).toBeNull();
});
