<script lang="ts">
 import {clone,create} from '@bufbuild/protobuf';
 import {getGameContext} from '#lib/services/GameContext.js';
 import {LogisticsAutomationSchema} from '#lib/types/cs-proto.js';
 const {player,universe,updatePlayerOrders}=getGameContext();
 let busy=$state(false),error=$state(''),message=$state('');
 let eligible=$derived($universe.fleets.filter(fleet=>{
  const map=fleet.mapObject;
  if(!map || map.playerNum!==$player.num || fleet.starbase || map.tags['design-maintenance'])return false;
  const orders=$player.playerOrders;
  if(orders.automation?.rebuildFleets[map.num])return false;
  return fleet.tokens.some(token=>(orders.designMaintenance[token.designNum]??0)>0);
 }));
 async function upgradeAll(){
  const before=$player.playerOrders.automation;
  const next=clone(LogisticsAutomationSchema,before??create(LogisticsAutomationSchema));
  const count=eligible.length;
  for(const fleet of eligible)next.rebuildFleets[fleet.mapObject!.num]=true;
  $player.playerOrders.automation=next;busy=true;error='';message='';
  try{await updatePlayerOrders();message=`Upgrade ordered for ${count} fleets. Automatic roles are preserved.`;}
  catch(cause){$player.playerOrders.automation=before;error=cause instanceof Error?cause.message:String(cause);}
  finally{busy=false;}
 }
</script>
<div>
 <button class="btn btn-primary btn-sm" disabled={busy || !eligible.length} onclick={upgradeAll} title="Scrap and rebuild every fleet with a designated successor project. Existing automatic roles resume after rebuilding.">Upgrade all ({eligible.length})</button>
 {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}
 {#if message}<p class="text-sm" role="status">{message}</p>{/if}
</div>
