import {create} from '@bufbuild/protobuf';
import {writable,get} from 'svelte/store';
import {beforeEach,expect,it,vi} from 'vitest';
import {page} from 'vitest/browser';
import {render} from 'vitest-browser-svelte';
import {FleetSchema,MapObjectSchema,PlayerSchema,PlayerOrdersSchema,LogisticsAutomationSchema} from '#lib/types/cs-proto.js';
import UpgradeAll from './UpgradeAll.svelte';

function fleet(num:number,designNum:number,playerNum=1,starbase=false){return create(FleetSchema,{mapObject:create(MapObjectSchema,{num,playerNum}),starbase,tokens:[{designNum,quantity:1}]});}
function makeContext(){return {
 player:writable(create(PlayerSchema,{num:1,playerOrders:create(PlayerOrdersSchema,{designMaintenance:{1:2,3:-1},automation:create(LogisticsAutomationSchema,{fleetRoles:{1:'patrol'}})})})),
 universe:writable({fleets:[fleet(1,1),fleet(2,2),fleet(3,3),fleet(4,1,2),fleet(5,1,1,true),fleet(6,1)]}),
 updatePlayerOrders:vi.fn(async()=>{})
};}
let context:ReturnType<typeof makeContext>;
vi.mock('#lib/services/GameContext.js',()=>({getGameContext:()=>context}));
beforeEach(()=>{context=makeContext();});
it('orders all eligible old fleets together and preserves automatic roles',async()=>{
 render(UpgradeAll);
 await page.getByRole('button',{name:'Upgrade all (2)',exact:true}).click();
 expect(get(context.player).playerOrders!.automation!.rebuildFleets).toEqual({1:true,6:true});
 expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({1:'patrol'});
 expect(context.updatePlayerOrders).toHaveBeenCalledTimes(1);
 await expect.element(page.getByRole('button',{name:'Upgrade all (0)',exact:true})).toBeDisabled();
});
it('restores previous orders if the server rejects the update',async()=>{
 context.updatePlayerOrders.mockRejectedValueOnce(new Error('Save failed'));
 render(UpgradeAll);
 await page.getByRole('button',{name:'Upgrade all (2)',exact:true}).click();
 await expect.element(page.getByRole('alert')).toHaveTextContent('Save failed');
 expect(get(context.player).playerOrders!.automation!.rebuildFleets).toEqual({});
 expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({1:'patrol'});
});
