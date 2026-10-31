import {create} from '@bufbuild/protobuf';
import {writable} from 'svelte/store';
import {expect,it,vi} from 'vitest';
import {page} from 'vitest/browser';
import {render} from 'vitest-browser-svelte';
import {FleetSchema,MapObjectSchema} from '#lib/types/cs-proto.js';
import FuelRouteDetails from './FuelRouteDetails.svelte';
const plan={leg:0,route:{turns:4,legs:[{to:{x:20,y:0},warp:5},{to:{x:80,y:0},warp:7,refuelDock:true,target:{targetName:'Home'},donations:[{fleetNum:9,amount:40}]}]},status:'Reserved'};
const leader=create(FleetSchema,{mapObject:create(MapObjectSchema,{num:1,playerNum:1,tags:{'fuel-network-plan':JSON.stringify(plan)}})});
vi.mock('#lib/services/GameContext.js',()=>({getGameContext:()=>({player:writable({num:1}),universe:writable({fleets:[leader,create(FleetSchema,{mapObject:create(MapObjectSchema,{num:9,playerNum:1,name:'Network tender'})})]})})}));
it('shows a convoy member the shared reserved itinerary and actual tanker names',async()=>{
 render(FuelRouteDetails,{fleet:create(FleetSchema,{mapObject:create(MapObjectSchema,{num:2,playerNum:1,tags:{'fuel-network-member':'1'}})})});
 await page.getByText('Reserved fuel route: leg 1 of 2',{exact:true}).click();
 await expect.element(page.getByText('Planned travel: 4 turns.',{exact:false})).toBeVisible();
 await expect.element(page.getByText(/Refuel from Network tender \(40\); Home/)).toBeVisible();
});
it('shows the finite stock committed by a donor',async()=>{
 render(FuelRouteDetails,{fleet:create(FleetSchema,{mapObject:create(MapObjectSchema,{num:9,playerNum:1,tags:{'fuel-network-donor':JSON.stringify({leader:1,amount:40})}})})});
 await expect.element(page.getByText('40 fuel reserved for fleet #1. Return fuel is protected.',{exact:true})).toBeVisible();
});
