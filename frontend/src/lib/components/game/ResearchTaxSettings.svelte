<script lang="ts">
	import HelpHint from '#lib/components/game/HelpHint.svelte';
 import {clone,create} from '@bufbuild/protobuf';
 import {ResearchTaxPolicySchema,type LogisticsAutomation} from '#lib/types/cs-proto.js';
 let {automation,busy=false,onSave}:{automation:LogisticsAutomation,busy?:boolean,onSave:(change:(value:LogisticsAutomation)=>void)=>Promise<void>}=$props();
 const defaults=()=>create(ResearchTaxPolicySchema,{basis:'population',leftoverUntil:50000,mediumThreshold:250000,largeThreshold:750000,smallPercent:10,mediumPercent:20,largePercent:30});
 let policy=$state(defaults());
 let maturePercent=$state(50);
 $effect(()=>{const next=clone(ResearchTaxPolicySchema,automation.researchTax??defaults());policy=next;maturePercent=next.matureResearchPercent??50;});
 function changeBasis(basis:string){policy.basis=basis;if(basis==='resources'){policy.leftoverUntil=50;policy.mediumThreshold=250;policy.largeThreshold=1000;}else{policy.leftoverUntil=50000;policy.mediumThreshold=250000;policy.largeThreshold=750000;}}
 let valid=$derived(policy.leftoverUntil>=0 && policy.mediumThreshold>=policy.leftoverUntil && policy.largeThreshold>policy.mediumThreshold && policy.largeThreshold<=100000000 && maturePercent>=0 && maturePercent<=100 && [policy.smallPercent,policy.mediumPercent,policy.largePercent].every(rate=>rate>=0&&rate<=100));
</script>
<fieldset class="border border-base-300 rounded-md p-3 my-3" aria-label="Automatic colony research tax">
 <legend class="font-semibold">Automatic colony research tax <HelpHint label="Automatic colony research tax" text="Set the research share by colony size. Colonies below the leftover threshold spend on their production queues first." example="A 15% tax reserves 150 of a 1,000-resource colony budget for research." /></legend>
 <label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" bind:checked={policy.enabled} disabled={busy} />Enable automatic colony research tax</label>
 <label class="block my-2">Colony size measured by<select class="select select-sm w-full" aria-label="Research tax size basis" value={policy.basis||'population'} onchange={e=>changeBasis(e.currentTarget.value)}><option value="population">Population (people)</option><option value="resources">Annual resource production</option></select></label>
 <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
  <label>Use only leftovers below<input class="input input-bordered w-full" aria-label="Research tax leftover threshold" type="number" min="0" max="100000000" bind:value={policy.leftoverUntil} /></label>
  <label>Medium colony from<input class="input input-bordered w-full" aria-label="Research tax medium threshold" type="number" min="0" max="100000000" bind:value={policy.mediumThreshold} /></label>
  <label>Large colony from<input class="input input-bordered w-full" aria-label="Research tax large threshold" type="number" min="1" max="100000000" bind:value={policy.largeThreshold} /></label>
  <label>Small colony research (%)<input class="input input-bordered w-full" aria-label="Small colony research (%)" type="number" min="0" max="100" bind:value={policy.smallPercent} /></label>
  <label>Medium colony research (%)<input class="input input-bordered w-full" aria-label="Medium colony research (%)" type="number" min="0" max="100" bind:value={policy.mediumPercent} /></label>
  <label>Large colony research (%)<input class="input input-bordered w-full" aria-label="Large colony research (%)" type="number" min="0" max="100" bind:value={policy.largePercent} /></label>
 </div>
 <p class="text-xs my-3">Below the first threshold, colonies develop using all production and contribute only leftovers. At or above it, the appropriate rate is reserved for research before production, overriding the manual empire budget. Colony size is checked each turn. Population is measured in people; resources means gross annual production. Disabling the policy returns rates to the manual empire budget and keeps the last planet leftover settings.</p>
 <div class="border-t pt-3 mt-3">
  <label class="flex gap-2"><input type="checkbox" class="checkbox" bind:checked={policy.disableMatureEconomy} disabled={busy} />Disable mature colony research / alchemy</label>
  <label class="block my-2">Research at full factory and mine capacity (%)<input class="input input-bordered w-full" type="number" min="0" max="100" bind:value={maturePercent} aria-label="Mature colony research (%)" disabled={busy||policy.disableMatureEconomy} /></label>
  <p class="text-sm">When both current operating limits are filled, {maturePercent}% goes to research and the remaining {100-maturePercent}% to alchemy. Ship or starbase orders restore the normal tax and suspend this automatic alchemy. This setting also works with the ordinary tax policy disabled.</p>
 </div>
 {#if !valid}<p class="text-error text-sm">Use ascending thresholds and research rates from 0 to 100%.</p>{/if}
 <button class="btn btn-primary btn-sm" disabled={busy||!valid} onclick={()=>onSave(value=>{policy.matureResearchPercent=maturePercent;value.researchTax=clone(ResearchTaxPolicySchema,policy);})}>Save colony research tax</button>
</fieldset>
