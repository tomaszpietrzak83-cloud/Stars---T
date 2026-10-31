<script lang="ts">
	import { TechField, type Race } from '#lib/types/cs-proto.js';
	import { eeeTraits, eeeTraitCost, selectEEETrait } from '#lib/types/EEE.js';
	let { race=$bindable(), readOnly=false }: { race: Race; readOnly?: boolean }=$props();
	let total=$derived(eeeTraits.reduce((cost,trait)=>cost+((race.eeeTraits&trait.flag)?eeeTraitCost(race,trait.flag):0),0));
	function toggle(flag:number,selected:boolean) { selectEEETrait(race,flag,selected); race={...race}; }
</script>

<section aria-label="EEE racial traits">
	<p class="text-sm my-2">EEE cost: {total} points. Positive costs spend points; negative costs return them. IFE (+10), ARM (+15) and UR (+10) interactions are included below. IFE reduces the Inefficient Propulsion refund by 10 points. Selecting an opposing trait deselects its counterpart.</p>
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
		{#each eeeTraits as trait (trait.flag)}
			{#if !readOnly || (race.eeeTraits&trait.flag)}
				<label class="border border-base-300 rounded p-2 flex gap-2 items-start">
					<input type="checkbox" class="checkbox checkbox-sm" aria-label={`EEE — ${trait.name}`} checked={!!(race.eeeTraits&trait.flag)} disabled={readOnly} onchange={(event)=>toggle(trait.flag,event.currentTarget.checked)} />
					<span><strong>EEE — {trait.name}</strong> ({eeeTraitCost(race,trait.flag)} pts)<br /><span class="text-sm">{trait.description}</span></span>
				</label>
			{/if}
		{/each}
	</div>
	{#if race.eeeTraits&(1<<11)}
		<label class="flex gap-2 my-3">EEE — Specialization field
			<select class="select select-bordered select-sm" aria-label="EEE — Specialization field" bind:value={race.eeeResearchField} disabled={readOnly}>
				<option value={TechField.ENERGY}>Energy</option><option value={TechField.WEAPONS}>Weapons</option><option value={TechField.PROPULSION}>Propulsion</option><option value={TechField.CONSTRUCTION}>Construction</option><option value={TechField.ELECTRONICS}>Electronics</option><option value={TechField.BIOTECHNOLOGY}>Biotechnology</option>
			</select>
		</label>
	{/if}
</section>
