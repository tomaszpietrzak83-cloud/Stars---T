<script lang="ts">
 import UpgradeAll from '#lib/components/game/design/UpgradeAll.svelte';
	import { clone,create } from '@bufbuild/protobuf';
 import { LogisticsAutomationSchema } from '#lib/types/cs-proto.js';
 import {designFamilyMembers,latestFamilyDesign,hideSupersededDesign} from '#lib/types/designFamilies.js';
 import Breadcrumb from '#lib/components/game/Breadcrumb.svelte';
	import DesignCard from '#lib/components/game/DesignCard.svelte';
	import ItemTitle from '#lib/components/ItemTitle.svelte';
	import TableSearchInput from '#lib/components/table/TableSearchInput.svelte';
	import { QueueItemType, type ShipDesign } from '#lib/types/cs-proto.js';
	import { getGameContext } from '#lib/services/GameContext.js';

	const { game, player, universe, settings, deleteDesign, readOnly,updatePlayerOrders } = getGameContext();

	let familyError=$state('');
 function members(num:number){return designFamilyMembers($universe.getMyDesigns(),$player.playerOrders.automation?.designFamilies??{},num);}
 function latest(num:number){return latestFamilyDesign($universe.getMyDesigns(),$player.playerOrders.automation?.designFamilies??{},num);}
 function familyFleets(num:number){const target=latest(num);const previous=new Set(members(num).filter(d=>d.num!==target?.num).map(d=>d.num));return $universe.fleets.filter(f=>f.mapObject?.playerNum===$player.num && !f.starbase && !f.mapObject?.tags['design-maintenance'] && f.tokens.some(t=>previous.has(t.designNum)));}
 async function rebuildFamily(num:number){
  const before=$player.playerOrders.automation;
  const previousMaintenance={...$player.playerOrders.designMaintenance};
  const target=latest(num);
  if(target){for(const design of members(num)){if(design.num!==target.num)$player.playerOrders.designMaintenance[design.num]=target.num;}}
  const automation=clone(LogisticsAutomationSchema,before??create(LogisticsAutomationSchema));
  for(const fleet of familyFleets(num)) automation.rebuildFleets[fleet.mapObject!.num]=true;
  $player.playerOrders.automation=automation;familyError='';
  try{await updatePlayerOrders();}catch(cause){$player.playerOrders.automation=before;$player.playerOrders.designMaintenance=previousMaintenance;familyError=String(cause);}
 }
 // how many of each design are waiting in our production queues, by design num
	let queuedByDesign = $derived.by(() => {
		const queued: Record<number, number> = {};
		for (const planet of $universe.planets) {
			for (const item of planet.planetOrders?.productionQueue ?? []) {
				if (item.type === QueueItemType.SHIP_TOKEN || item.type === QueueItemType.STARBASE) {
					queued[item.designNum] = (queued[item.designNum] ?? 0) + item.quantity;
				}
			}
		}
		return queued;
	});

	// filterable designs
	let search = $state('');
	let filteredDesigns: ShipDesign[] = $derived(
		$universe
			.getMyDesigns()
   .filter(d => !hideSupersededDesign(d, $universe.getMyDesigns(), $player.playerOrders.automation?.designFamilies ?? {}, $player.playerOrders.designMaintenance, queuedByDesign[d.num] ?? 0))
			.filter(
				(i) =>
					i.name.toLowerCase().indexOf(search.toLowerCase()) != -1 ||
					i.hull.toLowerCase().indexOf(search.toLocaleLowerCase()) != -1
			)
			// unused designs have nothing built from them and nothing queued, so they're safe to delete
			.filter(
				(i) => !$settings.showUnusedDesignsOnly || (!i.spec?.numInstances && !queuedByDesign[i.num])
			)
			.sort((a, b) => a.name.localeCompare(b.name))
	);
</script>

<Breadcrumb>
	{#snippet crumbs()}
		<li>
			<div class="hidden sm:block">Ship Designs</div>
			<div class="sm:hidden">Designs</div>
		</li>
	{/snippet}
	{#snippet end()}
		<div class="flex justify-end mb-1">
			<div class="flex flex-row justify-between items-center gap-2 m-2">
				<label class="cs-form-label cursor-pointer shrink-0">
					<span class="cs-label-text mr-1">Unused Only</span>
					<input
						type="checkbox"
						class="toggle"
						class:toggle-accent={$settings.showUnusedDesignsOnly}
						bind:checked={$settings.showUnusedDesignsOnly}
						data-type="unused-designs-only"
					/>
				</label>
				<TableSearchInput bind:value={search} />
				{#if !$readOnly}
     <UpgradeAll />
					<div>
						<a class="cs-link btn btn-sm" href={`/games/${$game.id}/designer/create`}
							><span class="hidden sm:block">Create</span><span class="sm:hidden">+</span></a
						>
					</div>
				{/if}
			</div>
		</div>
	{/snippet}
</Breadcrumb>

{#if familyError}<p class="text-error" role="alert">{familyError}</p>{/if}
<div class="flex flex-wrap justify-evenly gap-2">
	{#each filteredDesigns.filter((d) => d.playerNum === $player.num && !d.spec?.starbase) as design (design.num)}
		<DesignCard
			{design}
			readOnly={$readOnly}
			numQueued={queuedByDesign[design.num] ?? 0}
			href={`/games/${$game.id}/designer/${design.num}`}
			copyhref={`/games/${$game.id}/designer/create/${design.hull}?copy=${design.num}`}
   upgradehref={`/games/${$game.id}/designer/create/${design.hull}?upgrade=${design.num}`}
   familyLabel={members(design.num).length>1?`Family: ${members(design.num).map(d=>d.name).join(' → ')}`:undefined}
   onRebuildFamily={members(design.num).length>1 && latest(design.num)?.num===design.num ? ()=>rebuildFamily(design.num):undefined}
   familyFleetCount={familyFleets(design.num).length}

			onDelete={async (design) => {
				if (design.num) {familyError=''; try {await deleteDesign(design.num);} catch(cause) {familyError=String(cause);}}
			}}
		/>
	{/each}
</div>

<ItemTitle>Starbases</ItemTitle>
<div class="flex flex-wrap justify-evenly gap-2">
	{#each filteredDesigns.filter((d) => d.playerNum === $player.num && d.spec?.starbase) as design (design.num)}
		<DesignCard
			{design}
			readOnly={$readOnly}
			numQueued={queuedByDesign[design.num] ?? 0}
			href={`/games/${$game.id}/designer/${design.num}`}
			copyhref={`/games/${$game.id}/designer/create/${design.hull}?copy=${design.num}`}
			onDelete={async (design) => {
				if (design.num) {familyError=''; try {await deleteDesign(design.num);} catch(cause) {familyError=String(cause);}}
			}}
		/>
	{/each}
</div>
