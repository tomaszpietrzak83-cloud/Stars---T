import { create } from '@bufbuild/protobuf';
import { writable, get } from 'svelte/store';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { PlayerSchema, PlayerOrdersSchema, LogisticsAutomationSchema } from '#lib/types/cs-proto.js';
import PriorityTrader from './PriorityTrader.svelte';
let context: ReturnType<typeof makeContext>;
vi.mock('#lib/services/GameContext.js',()=>({getGameContext:()=>context}));
function makeContext(){return {player:writable(create(PlayerSchema,{num:1,playerOrders:create(PlayerOrdersSchema,{automation:create(LogisticsAutomationSchema)})})),updatePlayerOrders:vi.fn(async()=>{})};}
beforeEach(()=>{context=makeContext();});
it('persists and clears the trader crosshair without changing other automation',async()=>{
 get(context.player).playerOrders!.automation!.fleetRoles[4]='scout';
 render(PriorityTrader,{num:17});
 const button=page.getByRole('button',{name:'Priority trader'});
 await button.click();
 await expect.element(button).toHaveAttribute('aria-pressed','true');
 expect(get(context.player).playerOrders!.automation!.priorityTraders).toEqual({17:true});
 expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({4:'scout'});
 await button.click();
 await expect.element(button).toHaveAttribute('aria-pressed','false');
 expect(get(context.player).playerOrders!.automation!.priorityTraders).toEqual({});
 expect(context.updatePlayerOrders).toHaveBeenCalledTimes(2);
});
it('rolls back the crosshair after a server rejection',async()=>{
 context.updatePlayerOrders.mockRejectedValueOnce(new Error('Save failed'));
 render(PriorityTrader,{num:17});
 await page.getByRole('button',{name:'Priority trader'}).click();
 await expect.element(page.getByRole('alert')).toHaveTextContent('Save failed');
 expect(get(context.player).playerOrders!.automation!.priorityTraders).toEqual({});
});
