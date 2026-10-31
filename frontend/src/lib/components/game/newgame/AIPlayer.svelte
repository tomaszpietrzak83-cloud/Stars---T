<script lang="ts">
	import { difficultyLevels } from '#lib/types/aiDifficulty.js';
	import { AiDifficulty, type NewGamePlayer } from '#lib/types/cs-proto.js';
	import type { Race } from '#lib/types/cs-proto.js';
	import { loadRaceLibrary } from './raceLibrary';
	import { onMount } from 'svelte';
	let races: Race[]=$state([]);
	onMount(async()=>{ try {races=await loadRaceLibrary();} catch {races=[];} });

	type Props = {
		player: NewGamePlayer;
	};

	let { player = $bindable() }: Props = $props();
</script>

<div class="w-3/4 ml-auto grow">
	<label class="flex gap-2 my-2">Race <select class="select select-sm grow" aria-label="AI race" value={player.race?.id?.toString()??''} onchange={(event)=>{player.race=races.find(race=>race.id.toString()===event.currentTarget.value);}}><option value="">Random race</option>{#each races as race (race.id)}<option value={race.id.toString()}>{race.pluralName}</option>{/each}</select></label>
 <label class="flex gap-2 my-2">Difficulty <select class="select select-sm grow" aria-label="Difficulty" bind:value={player.aiDifficulty}>{#each difficultyLevels as level (level.value)}<option value={level.value}>{level.label}</option>{/each}</select></label>
</div>

<!-- <ColorInput bind:value={player.color} name="color" /> -->
