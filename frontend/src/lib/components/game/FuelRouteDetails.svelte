<script lang="ts">
 import {getGameContext} from '#lib/services/GameContext.js';
 import type {Fleet} from '#lib/types/cs-proto.js';
 let {fleet}:{fleet:Fleet}=$props();
 const {player,universe}=getGameContext();
 type Leg={to:{x:number,y:number},warp:number,refuelDock?:boolean,target?:{targetName?:string},donations?:{fleetNum:number,amount:number}[]};
 type Plan={leg:number,route:{turns:number,legs:Leg[]},status:string};
 function parse<T>(raw:string|undefined):T|undefined {try{return raw?JSON.parse(raw) as T:undefined;}catch{return undefined;}}
 let leader=$derived(($universe.fleets??[]).find(f=>f.mapObject?.playerNum===$player.num && f.mapObject.num===Number(fleet.mapObject?.tags['fuel-network-member']))??fleet);
 let plan=$derived(parse<Plan>(leader.mapObject?.tags['fuel-network-plan']));
 let installations=$derived(parse<{factories:number,mines:number,defenses:number,destination:number}>(fleet.mapObject?.tags['infrastructure-cargo']));
 let hold=$derived(parse<{leader:number,amount:number}>(fleet.mapObject?.tags['fuel-network-donor']));
 function tankerName(num:number){return ($universe.fleets??[]).find(f=>f.mapObject?.playerNum===$player.num && f.mapObject.num===num)?.mapObject?.name??`Tanker #${num}`;}
</script>
{#if plan?.route?.legs?.length}
 <details class="text-xs mt-2" aria-label="Reserved fuel itinerary">
  <summary>Reserved fuel route: leg {plan.leg+1} of {plan.route.legs.length}</summary>
  <p class="my-1">Planned travel: {plan.route.turns} turns. Fuel is reserved; changes trigger replanning.</p>
  <ol class="list-decimal pl-4">
   {#each plan.route.legs.slice(plan.leg) as leg,index (`${plan.leg+index}`)}
    <li>
     {#if leg.donations?.length}Refuel from {leg.donations.map(d=>`${tankerName(d.fleetNum)} (${d.amount})`).join(', ')}; {/if}
     {leg.target?.targetName || `(${leg.to.x}, ${leg.to.y})`} — {leg.warp===11?'stargate':`warp ${leg.warp}`}{leg.refuelDock?' — fuel dock':''}
    </li>
   {/each}
  </ol>
 </details>
{/if}
{#if hold}<p class="text-xs mt-2">{hold.amount} fuel reserved for fleet #{hold.leader}. Return fuel is protected.</p>{/if}

{#if installations}<p class="text-xs mt-2">Packed installations: {installations.factories} factories, {installations.mines} mines, {installations.defenses} defenses ({installations.factories*5+installations.mines*3+installations.defenses} kT). Delivery colony #{installations.destination}.</p>{/if}
