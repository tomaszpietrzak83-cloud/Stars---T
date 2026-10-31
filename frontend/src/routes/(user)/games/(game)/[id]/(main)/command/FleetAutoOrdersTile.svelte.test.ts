import {create} from '@bufbuild/protobuf';
import {writable,get} from 'svelte/store';
import {beforeEach,expect,it,vi} from 'vitest';
import {page} from 'vitest/browser';
import {render} from 'vitest-browser-svelte';
import {FleetSchema,PlayerSchema,LogisticsAutomationSchema} from '#lib/types/cs-proto.js';
import FleetAutoOrdersTile from './FleetAutoOrdersTile.svelte';
let context:ReturnType<typeof makeContext>;
vi.mock('#lib/services/GameContext.js',()=>({getGameContext:()=>context}));
function makeContext(){return {player:writable(create(PlayerSchema,{num:1,playerOrders:{automation:create(LogisticsAutomationSchema)}})),universe:writable({getMyDesigns:()=>[]}),updatePlayerOrders:vi.fn(async()=>{})};}
const fleet=()=>create(FleetSchema,{mapObject:{num:1,playerNum:1,name:'Terraformer'},spec:{shipDesignSpec:{terraformRate:2,fuelCapacity:500}},tokens:[{designNum:1,quantity:1}]});
beforeEach(()=>{context=makeContext();});
it('assigns terraforming from the selected fleet tile',async()=>{
 render(FleetAutoOrdersTile,{fleet:fleet()});
 await page.getByRole('combobox',{name:'Selected fleet automatic role'}).selectOptions('terraformer');
 expect(get(context.player).playerOrders!.automation!.fleetRoles[1]).toBe('terraformer');
 expect(context.updatePlayerOrders).toHaveBeenCalledOnce();
 await page.getByRole('button',{name:'Disable automation'}).click();
 expect(get(context.player).playerOrders!.automation!.fleetRoles[1]).toBeUndefined();
});
it('rolls back a rejected role and displays the actual error',async()=>{
 context.updatePlayerOrders.mockRejectedValueOnce(new Error('Server rejected assignment'));
 render(FleetAutoOrdersTile,{fleet:fleet()});
 await page.getByRole('combobox',{name:'Selected fleet automatic role'}).selectOptions('terraformer');
 await expect.element(page.getByRole('alert')).toHaveTextContent('Server rejected assignment');
 expect(get(context.player).playerOrders!.automation!.fleetRoles[1]).toBeUndefined();
});
it('does not offer equipment-dependent roles to a scout',async()=>{
 const scout=fleet();scout.spec!.shipDesignSpec!.terraformRate=0;
 render(FleetAutoOrdersTile,{fleet:scout});
 expect(Array.from((page.getByRole('combobox').element() as HTMLSelectElement).options).some(o=>o.value==='terraformer')).toBe(false);
});

it('describes the selected fleet next action and warp in its Auto Orders tile',async()=>{
 const f=create(FleetSchema,{mapObject:{num:1,playerNum:1,name:'Scout',tags:{'automatic-fuel-status':'Returning to refuel'}},spec:{shipDesignSpec:{fuelCapacity:300}},fleetOrders:{waypoints:[{position:{x:100,y:0}},{position:{x:0,y:0},mapObjectTarget:{targetName:'Home'},warpSpeed:4}]}});
 context.player.update(p=>{p.playerOrders!.automation!.fleetRoles[1]='scout';return p;});
 render(FleetAutoOrdersTile,{fleet:f});
 await expect.element(page.getByText(/Returning to refuel.*Next: Home, warp 4/)).toBeVisible();
});
