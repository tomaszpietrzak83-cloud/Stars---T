<script lang="ts">
	import ItemTitle from '#lib/components/ItemTitle.svelte';
	import SectionHeader from '#lib/components/SectionHeader.svelte';
	import { CommandedPlayer, TechFields } from '#lib/types/Player.js';
	import { Beaker } from '@steeze-ui/heroicons';
	import { Icon } from '@steeze-ui/svelte-icon';

	import EnumSelect from '#lib/components/EnumSelect.svelte';
	import SpinnerNumberText from '#lib/components/SpinnerNumberText.svelte';
	import Factory from '#lib/components/icons/Factory.svelte';
	import Microscope from '#lib/components/icons/Microscope.svelte';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { enumToString } from '#lib/types/Enums.js';
	import { get } from '#lib/types/TechLevel.js';
	import { NextResearchField, PlayerResearchSpecSchema, TechField } from '#lib/types/cs-proto.js';
	import { create } from '@bufbuild/protobuf';
	import FutureTechs from './FutureTechs.svelte';

	const { cs, player, game } = getGameContext();

	type Props = {
		onUpdatePlayer?: () => Promise<void>;
	};
	let { onUpdatePlayer }: Props = $props();

 const {universe}=getGameContext();
 let researchColonies=$derived($universe.planets.filter(planet=>planet.mapObject?.playerNum===$player.num));
 let tax=$derived($player.playerOrders.automation?.researchTax);
 let leftoverColonies=$derived(researchColonies.filter(planet=>tax?.enabled ? (tax.basis==='resources'?(planet.spec?.resourcesPerYear??0):(planet.cargo?.colonists??0)*100)<tax.leftoverUntil : planet.planetOrders?.contributesOnlyLeftoverToResearch));

	let spec = $state(create(PlayerResearchSpecSchema));

	$effect(() => {
		$player.getResearchSpec(cs).then((s) => (spec = s));
	});

	const getLevel = (player: CommandedPlayer, field: TechField): number => {
		return get(player.techLevels, field);
	};

	let spent = $derived(get($player.techLevelsSpent, $player.playerOrders.researching) || 0);

	let leftToSpend = $derived(spec.currentResearchCost - spent);
	let yearsLeft = $derived(Math.ceil(leftToSpend / spec.resourcesPerYearResearchEstimated));
</script>

<ItemTitle>Research</ItemTitle>
{#if tax?.enabled}<div class="alert alert-info my-3 block"><strong>Automatic colony research tax is active.</strong> Small / medium / large colonies reserve {tax.smallPercent}% / {tax.mediumPercent}% / {tax.largePercent}% before production. These rates override the manual empire budget below. <a class="link" href={`/games/${$game.id}/automation`}>Configure automation</a></div>{/if}
{#if leftoverColonies.length}
 <div class="alert alert-warning my-3 block" role="status">
  <p><strong>{leftoverColonies.length} of {researchColonies.length} colonies contribute only production leftovers.</strong> The research percentage is not reserved on these planets. If production uses all resources, they contribute zero.</p>
  <p class="text-sm mt-1">{leftoverColonies.map(planet=>planet.mapObject?.name).join(', ')}. Disable “Contributes only leftover to research” in each planet's production settings to reserve the research budget before production.</p>
 </div>
{/if}
<div
	class="stats stats-vertical sm:stats-horizontal sm:flex shadow-sm border border-base-200 w-full"
>
	<div class="stat place-items-center sm:grow">
		<div class="stat-title">Researching</div>
		<div class="stat-figure"><Icon class="size-8 " src={Beaker} /></div>
		<div class="stat-value">
			{enumToString(TechField, $player.playerOrders.researching)}
			{getLevel($player, $player.playerOrders.researching) + 1}
		</div>
		<div class="stat-desc pt-1">
			{spent}/{spec.currentResearchCost} resources
			{#if yearsLeft < 100}
				, {yearsLeft.toFixed()}
				{Math.floor(yearsLeft) > 1 ? 'years' : 'year'}
			{/if}
		</div>
	</div>
	<div class="stat place-items-center sm:grow">
		<div class="stat-title">Resources Available</div>
		<div class="stat-figure"><Factory class="size-8  fill-primary" /></div>
		<div class="stat-value">
			{spec.resourcesPerYear}
		</div>
	</div>
</div>
<div
	class="stats stats-vertical sm:stats-horizontal sm:flex shadow-sm border border-base-200 w-full"
>
	<div class="stat place-items-center sm:grow">
		<div class="stat-title">Spent Last Year</div>
		<div class="stat-figure"><Microscope class="size-8  fill-primary" /></div>
		<div class="stat-value">
			{$player.researchSpentLastYear}
		</div>
	</div>
	<div class="grow stat place-items-center">
		<div class="stat-title">Estimated Spending Next Year</div>
		<div class="stat-figure"><Microscope class="size-8  fill-warning" /></div>
		<div class="stat-value">
			{spec.resourcesPerYearResearchEstimated}
		</div>
	</div>
</div>
<div class="grid grid-cols-1 md:grid-cols-2 justify-center gap-2">
	<div class="w-full">
		<SectionHeader>Tech Levels</SectionHeader>

		<SpinnerNumberText
			class="flex flex-row gap-1 place-content-center text-2xl mb-2"
			bind:value={$player.playerOrders.researchAmount}
			min={0}
			max={100}
			step={1}
			unit="%"
			onChange={onUpdatePlayer}
		>
			{#snippet begin()}
				Research Budget
			{/snippet}
			{#snippet end()}{/snippet}
		</SpinnerNumberText>

		<div class="grid grid-cols-2">
			<div class="text-center">
				Field of Study <div class="divider secondary w-[90%]"></div>
			</div>
			<div class="text-center">
				Current Level <div class="divider secondary w-[90%]"></div>
			</div>
			{#each TechFields as field (field)}
				<div class="cs-form-control">
					<label class="cs-form-label cursor-pointer">
						<span class="cs-label-text">{enumToString(TechField, field)}</span>
						<input
							type="radio"
							name="researching"
							value={field}
							class="radio radio-sm checked:bg-primary"
							bind:group={$player.playerOrders.researching}
							onchange={onUpdatePlayer}
						/>
					</label>
				</div>
				<div class="text-center">
					{getLevel($player, field)}
				</div>
			{/each}
		</div>
		<EnumSelect
			name="nextResearchField"
			enumType={NextResearchField}
			bind:value={$player.playerOrders.nextResearchField}
			onchange={onUpdatePlayer}
		/>
	</div>

	<div class="w-full">
		<SectionHeader>Expected Research Benefits</SectionHeader>
		<FutureTechs field={$player.playerOrders.researching} />
	</div>
</div>
