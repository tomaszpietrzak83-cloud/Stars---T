<script lang="ts">
 import FuelRouteDetails from '#lib/components/game/FuelRouteDetails.svelte';
 import { clone, create } from '@bufbuild/protobuf';
 import { getGameContext } from '#lib/services/GameContext.js';
 import { LogisticsAutomationSchema, type Fleet } from '#lib/types/cs-proto.js';
 import { automationRoles, automationStatus } from '#lib/types/automationRoles.js';
 import CommandTile from './CommandTile.svelte';
 let { fleet }: {fleet: Fleet}=$props();
 const {player,universe,updatePlayerOrders}=getGameContext();
 let busy=$state(false),error=$state('');
 let automation=$derived($player.playerOrders.automation??create(LogisticsAutomationSchema));
 let options=$derived(automationRoles(fleet,$universe.getMyDesigns(),automation.fleetRoles[fleet.mapObject?.num??0]??''));
 let num=$derived(fleet.mapObject?.num??0);
 let petal=$derived.by(()=>{try{return JSON.parse(fleet.mapObject?.tags['scout-mission']??'null') as {plan?:number[],leg?:number,returnAt?:number}|null;}catch{return null;}});
 let role=$derived(automation.fleetRoles[num]??'');
 let plan=$derived(automation.fleetAssemblies[Number(fleet.mapObject?.tags['fleet-assembly'])]);
 let locked=$derived(!!fleet.mapObject?.tags['design-maintenance'] || (!!plan && plan.completedRevision!==plan.revision));
 async function assign(value:string) {
  const previous=$player.playerOrders.automation;
  const next=clone(LogisticsAutomationSchema,automation);
  if(value) next.fleetRoles[num]=value;else delete next.fleetRoles[num];
  $player.playerOrders.automation=next;busy=true;error='';
  try {await updatePlayerOrders();} catch(cause) {$player.playerOrders.automation=previous;error=cause instanceof Error?cause.message:'Could not save automatic orders';} finally {busy=false;}
 }
</script>
{#if fleet.mapObject?.playerNum===$player.num && !fleet.starbase}
 <CommandTile title="Auto Orders">
  <label class="text-sm">Automatic role
   <select class="select select-sm w-full" aria-label="Selected fleet automatic role" disabled={busy||locked} value={role} onchange={e=>assign(e.currentTarget.value)}>
    {#each options as option (option.value)}<option value={option.value}>{option.label}</option>{/each}
   </select>
  </label>
  <p class="text-xs mt-2">{role?automationStatus(fleet):'Manual waypoint orders. Select a role to enable automation.'}</p>
  <FuelRouteDetails {fleet} />
  {#if role==='scout' && petal?.plan?.length}<details class="text-xs mt-2"><summary>Planned petal itinerary</summary><ol class="list-decimal pl-4">{#each petal.plan.slice(petal.leg??0) as num,index (`${index}:${num}`)}<li>{$universe.getPlanet(num)?.mapObject?.name??`Planet #${num}`}{index+(petal.leg??0)>=(petal.returnAt??0)?' — return arm':''}</li>{/each}</ol></details>{/if}
  {#if role}<button class="btn btn-xs mt-2" disabled={busy||locked} onclick={()=>assign('')}>Disable automation</button>{/if}
  {#if locked}<p class="text-xs mt-2">Complete or cancel the active rebuild or fleet assembly first.</p>{/if}
  {#if error}<p class="text-error text-xs" role="alert">{error}</p>{/if}
 </CommandTile>
{/if}
