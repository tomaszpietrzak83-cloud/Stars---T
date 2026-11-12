<script lang="ts">
	import { MaxGameNameLength } from '#lib/types/Consts.js';
	import CheckboxInput from '#lib/components/CheckboxInput.svelte';
	import EnumSelect from '#lib/components/EnumSelect.svelte';
	import TextInput from '#lib/components/TextInput.svelte';
	import {
		Density,
		GameStartMode,
		PlayerPositions,
		Size,
		type GameSettings
	} from '#lib/types/cs-proto.js';
	import { GameStartModeFullNames } from '#lib/types/Enums.js';
	import PrivateGameLink from './PrivateGameLink.svelte';
	import { estimatedStars, mapDimensions } from './mapPopulation';
	import { defaultRules } from '#lib/types/Rules.js';
	import { randomGameName } from './gameNames';

	type Props = {
		settings: GameSettings;
		showInviteLink?: boolean;
		allowRandomName?: boolean;
	};

	let { settings = $bindable(), showInviteLink = false, allowRandomName=false }: Props = $props();
	let stars=$derived(estimatedStars(settings.size,settings.density,defaultRules.universeGenerationRules?.sqLyPerPlanet??5000));
	let dimensions=$derived(mapDimensions(settings.size));
</script>

<div class="flex flex-row flex-wrap">
	{#if showInviteLink && !settings.public}
		<PrivateGameLink />
	{/if}
	<div class="flex w-full items-center gap-2"><TextInput name="name" bind:value={settings.name} maxlength={MaxGameNameLength} />{#if allowRandomName}<button type="button" class="btn btn-sm" onclick={()=>{settings.name=randomGameName(settings.name);}}>Randomize game name</button>{/if}</div>
	<EnumSelect name="size" enumType={Size} bind:value={settings.size} description="Map dimensions in light years. Larger maps mean longer journeys." />
	<EnumSelect name="density" enumType={Density} bind:value={settings.density} description="How many planets fit in the same area. Denser maps have shorter gaps between worlds." />
	<output class="w-full border border-base-300 bg-base-200 p-3 my-2" aria-live="polite" aria-label="Estimated star count">{#if stars!==undefined && dimensions}Approximately <strong>{stars.toLocaleString('en-US')} stars</strong> (planets) — area {dimensions[0]} × {dimensions[1]} light years.{/if}</output>
	<EnumSelect
		name="playerPositions"
		enumType={PlayerPositions}
		bind:value={settings.playerPositions}
		description="Starting distances between empires: Close, Moderate or Distant. Your starting location is selected separately below."
	/>
	<CheckboxInput name="public" bind:checked={settings.public} description="List this game publicly so other players can find it." />
	<CheckboxInput name="randomEvents" bind:checked={settings.randomEvents} description="Enable random galactic events, including special encounters and planetary surprises." />
	<CheckboxInput name="publicPlayerScores" bind:checked={settings.publicPlayerScores} description="Allow players to see other empires' scores." />
	<CheckboxInput
		title="Beginner: Max Minerals"
		name="maxMinerals"
		bind:checked={settings.maxMinerals}
		description="Most planets start at concentration 100 for each mineral, so their mines can produce equal amounts of ironium, boranium and germanium."
	/>
	<CheckboxInput
		name="computerPlayersFormAlliances"
		bind:checked={settings.computerPlayersFormAlliances}
		description="Allow computer empires to cooperate as allies."
	/>
	<CheckboxInput
		title="Galaxy Clumping"
		name="galaxyClumping"
		bind:checked={settings.galaxyClumping}
		description="Group nearby stars into clusters, leaving larger gaps between groups. This changes their distribution, not the intended number of planets."
	/>
	<EnumSelect
		name="startMode"
		enumType={GameStartMode}
		bind:value={settings.startMode}
		typeTitle={(t) => GameStartModeFullNames[t]}
		showEmpty={true}
		description="Normal starts a young empire. Accelerated BBS adds starting population and minerals. Max Start begins with developed technology and abundant supplies."
		tooltip="Accelerated BBS Play gives homeworlds 25% more surface minerals, extra starting population based on growth rate, and slightly higher mineral concentrations. Max Start creates a game with maxed tech levels, minerals, etc."
	/>
</div>
