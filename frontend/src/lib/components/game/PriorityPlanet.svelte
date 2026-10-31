<script lang="ts">
 import { clone, create } from '@bufbuild/protobuf';
 import { getGameContext } from '#lib/services/GameContext.js';
 import { LogisticsAutomationSchema } from '#lib/types/cs-proto.js';
 let { num }: { num: number } = $props();
 const { player, updatePlayerOrders } = getGameContext();
 let busy = $state(false), error = $state('');
 let selected = $derived(!!$player.playerOrders.automation?.priorityPlanets[num]);
 async function toggle() {
  busy=true; error='';
  const before=$player.playerOrders.automation;
  const next=clone(LogisticsAutomationSchema, before ?? create(LogisticsAutomationSchema));
  if (selected) delete next.priorityPlanets[num]; else next.priorityPlanets[num]=true;
  $player.playerOrders.automation=next;
  try { await updatePlayerOrders(); }
  catch (cause) { $player.playerOrders.automation=before; error=cause instanceof Error?cause.message:'Could not save priority target.'; }
  finally { busy=false; }
 }
</script>
<button class="btn btn-sm my-1 {selected?'btn-primary':'btn-outline'}" aria-label="Priority planet" aria-pressed={selected} disabled={busy} onclick={toggle} title="Prioritize colonization, attack and reinforced landings">
 <svg aria-hidden="true" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="7"/><circle cx="12" cy="12" r="2"/><path d="M12 1v6m0 10v6M1 12h6m10 0h6"/></svg>
 {selected?'Priority target':'Set priority'}
</button>
{#if error}<p role="alert" class="text-error text-xs">{error}</p>{/if}
