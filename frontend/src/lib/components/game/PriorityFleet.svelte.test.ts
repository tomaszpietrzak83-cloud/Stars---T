import { create } from '@bufbuild/protobuf';
import { writable, get } from 'svelte/store';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { PlayerSchema, PlayerOrdersSchema, LogisticsAutomationSchema } from '#lib/types/cs-proto.js';
import PriorityFleet from './PriorityFleet.svelte';
let context: ReturnType<typeof makeContext>;
vi.mock('#lib/services/GameContext.js',()=>({getGameContext:()=>context}));
function makeContext(){return {player:writable(create(PlayerSchema,{num:1,playerOrders:create(PlayerOrdersSchema,{automation:create(LogisticsAutomationSchema)})})),updatePlayerOrders:vi.fn(async()=>{})};}
beforeEach(()=>{context=makeContext();});
it('persists and clears the fleet crosshair without changing other automation',async()=>{
 get(context.player).playerOrders!.automation!.fleetRoles[4]='scout';
 render(PriorityFleet,{num:17,playerNum:2});
 const button=page.getByRole('button',{name:'Priority fleet'});
 await button.click();
 await expect.element(button).toHaveAttribute('aria-pressed','true');
 expect(get(context.player).playerOrders!.automation!.priorityFleets).toEqual({'2:17':true});
 expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({4:'scout'});
 await button.click();
 await expect.element(button).toHaveAttribute('aria-pressed','false');
 expect(get(context.player).playerOrders!.automation!.priorityFleets).toEqual({});
 expect(context.updatePlayerOrders).toHaveBeenCalledTimes(2);
});
it('rolls back the crosshair after a server rejection',async()=>{
 context.updatePlayerOrders.mockRejectedValueOnce(new Error('Save failed'));
 render(PriorityFleet,{num:17,playerNum:2});
 await page.getByRole('button',{name:'Priority fleet'}).click();
 await expect.element(page.getByRole('alert')).toHaveTextContent('Save failed');
 expect(get(context.player).playerOrders!.automation!.priorityFleets).toEqual({});
});

it('distinguishes identical fleet numbers belonging to different players', async()=>{
 get(context.player).playerOrders!.automation!.priorityFleets['3:17']=true;
 render(PriorityFleet,{num:17,playerNum:2});
 await page.getByRole('button',{name:'Priority fleet'}).click();
 expect(get(context.player).playerOrders!.automation!.priorityFleets).toEqual({'2:17':true,'3:17':true});
});
