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
	<EnumSelect name="size" enumType={Size} bind:value={settings.size} />
	<EnumSelect name="density" enumType={Density} bind:value={settings.density} />
	<output class="w-full border border-base-300 bg-base-200 p-3 my-2" aria-live="polite" aria-label="Estimated star count">{#if stars!==undefined && dimensions}Approximately <strong>{stars.toLocaleString('en-US')} stars</strong> (planets) — area {dimensions[0]} × {dimensions[1]} light years.{/if}</output>
	<EnumSelect
		name="playerPositions"
		enumType={PlayerPositions}
		bind:value={settings.playerPositions}
	/>
	<CheckboxInput name="public" bind:checked={settings.public} />
	<CheckboxInput name="randomEvents" bind:checked={settings.randomEvents} />
	<CheckboxInput name="publicPlayerScores" bind:checked={settings.publicPlayerScores} />
	<CheckboxInput
		title="Beginner: Max Minerals"
		name="maxMinerals"
		bind:checked={settings.maxMinerals}
	/>
	<CheckboxInput
		name="computerPlayersFormAlliances"
		bind:checked={settings.computerPlayersFormAlliances}
	/>
	<CheckboxInput
		title="Galaxy Clumping"
		name="galaxyClumping"
		bind:checked={settings.galaxyClumping}
	/>
	<EnumSelect
		name="startMode"
		enumType={GameStartMode}
		bind:value={settings.startMode}
		typeTitle={(t) => GameStartModeFullNames[t]}
		showEmpty={true}
		tooltip="Accelerated BBS Play gives homeworlds 25% more surface minerals, extra starting population based on growth rate, and slightly higher mineral concentrations. Max Start creates a game with maxed tech levels, minerals, etc."
	/>
</div>
