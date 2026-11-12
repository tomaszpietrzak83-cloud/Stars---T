import { create } from '@bufbuild/protobuf';
import { writable, get } from 'svelte/store';
import { beforeEach, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { PlayerSchema, PlayerOrdersSchema, LogisticsAutomationSchema } from '#lib/types/cs-proto.js';
import PriorityPlanet from './PriorityPlanet.svelte';
let context: ReturnType<typeof makeContext>;
vi.mock('#lib/services/GameContext.js',()=>({getGameContext:()=>context}));
function makeContext(){return {player:writable(create(PlayerSchema,{num:1,playerOrders:create(PlayerOrdersSchema,{automation:create(LogisticsAutomationSchema)})})),updatePlayerOrders:vi.fn(async()=>{})};}
beforeEach(()=>{context=makeContext();});
it('persists and clears the planet crosshair without changing other automation',async()=>{
 get(context.player).playerOrders!.automation!.fleetRoles[4]='scout';
 render(PriorityPlanet,{num:17});
 const button=page.getByRole('button',{name:'Priority planet',exact:true});
 await button.click();
 await expect.element(button).toHaveAttribute('aria-pressed','true');
 expect(get(context.player).playerOrders!.automation!.priorityPlanets).toEqual({17:true});
 expect(get(context.player).playerOrders!.automation!.fleetRoles).toEqual({4:'scout'});
 await button.click();
 await expect.element(button).toHaveAttribute('aria-pressed','false');
 expect(get(context.player).playerOrders!.automation!.priorityPlanets).toEqual({});
 expect(context.updatePlayerOrders).toHaveBeenCalledTimes(2);
});
it('rolls back the crosshair after a server rejection',async()=>{
 context.updatePlayerOrders.mockRejectedValueOnce(new Error('Save failed'));
 render(PriorityPlanet,{num:17});
 await page.getByRole('button',{name:'Priority planet',exact:true}).click();
 await expect.element(page.getByRole('alert')).toHaveTextContent('Save failed');
 expect(get(context.player).playerOrders!.automation!.priorityPlanets).toEqual({});
});
