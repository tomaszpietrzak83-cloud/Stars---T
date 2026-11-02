<script lang="ts">
	import { untrack, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import ItemTitle from '#lib/components/ItemTitle.svelte';
	import SectionHeader from '#lib/components/SectionHeader.svelte';
	import { gameClient } from '#lib/services/connect.js';
	import {
		VictoryConditionAttainTechLevels,
		VictoryConditionExceedsSecondPlaceScore,
		VictoryConditionOwnPlanets
	} from '#lib/types/Consts.js';
	import {
		AiDifficulty,
		Density,
		type NewGamePlayer as GamePlayer,
		type GameSettings,
		GameSettingsSchema,
		GameStartMode,
		NewGamePlayerSchema,
		NewGamePlayerType,
		PlayerPositions,
		Size
	} from '#lib/types/cs-proto.js';
	import { create, toJsonString, fromJsonString, clone } from '@bufbuild/protobuf';
	import { PlusCircle } from '@steeze-ui/heroicons';
	import { Icon } from '@steeze-ui/svelte-icon';
	import GameSettingsEditor from './GameSettingsEditor.svelte';
	import NewGamePlayer from './NewGamePlayer.svelte';
	import { getColor, getFirstAvailableColor } from './playerColors';
	import VictoryConditions from './VictoryConditions.svelte';
	import { recommendedEmpires } from './mapPopulation';
	import { randomGameName } from './gameNames';
 import { difficultyLevels, variedDifficulty, difficultyDetails } from '#lib/types/aiDifficulty.js';

	type Props = {
		players?: GamePlayer[];
		name?: string;
	};

	let {
		players = [
			create(NewGamePlayerSchema, {
				type: NewGamePlayerType.HOST,
				color: getColor(0),
				aiDifficulty: AiDifficulty.UNSPECIFIED,
				defaultHullSet: 0
			}),
			create(NewGamePlayerSchema, {
				type: NewGamePlayerType.AI,
				color: getColor(1),
				aiDifficulty: AiDifficulty.NORMAL,
				defaultHullSet: 0
			}),
			create(NewGamePlayerSchema, {
				type: NewGamePlayerType.AI,
				color: getColor(2),
				aiDifficulty: AiDifficulty.NORMAL,
				defaultHullSet: 0
			})
		],
		name = randomGameName()
	}: Props = $props();

	let settings: GameSettings = $state(
		create(GameSettingsSchema, {
			name: untrack(() => name),
			public: false,
			size: Size.SMALL,
			density: Density.NORMAL,
			playerPositions: PlayerPositions.MODERATE,
			randomEvents: true,
			computerPlayersFormAlliances: false,
			publicPlayerScores: false,
			maxMinerals: false,
			startMode: GameStartMode.UNSPECIFIED, // normal
			quickStartTurns: 0,
			players: untrack(() => players),
			victoryConditions: {
				conditions:
					VictoryConditionOwnPlanets |
					VictoryConditionAttainTechLevels |
					VictoryConditionExceedsSecondPlaceScore,
				numCriteriaRequired: 1,
				yearsPassed: 50,
				ownPlanets: 60,
				attainTechLevel: 22,
				attainTechLevelNumFields: 4,
				exceedsScore: 11000,
				exceedsSecondPlaceScore: 100,
				productionCapacity: 100,
				ownCapitalShips: 100,
				highestScoreAfterYears: 100
			}
		})
	);

	let submitting = $state(false);
	let createError = $state('');
 const defaults = clone(GameSettingsSchema, untrack(()=>settings));
 const preferencesKey='tomek-stars.new-game.v1';
 let preferencesReady=$state(false);
 onMount(()=>{
  try { const raw=localStorage.getItem(preferencesKey); if(raw) settings=fromJsonString(GameSettingsSchema,raw); } catch { /* Fall back to defaults if storage is unavailable or obsolete. */ }
  try{const saved=Number(localStorage.getItem(preferencesKey+'.ai-level'));if(difficultyLevels.some(level=>level.value===saved))allAIDifficulty=saved;}catch{}
  preferencesReady=true;
 });
 $effect(()=>{if(preferencesReady) {try{localStorage.setItem(preferencesKey,toJsonString(GameSettingsSchema,settings));localStorage.setItem(preferencesKey+'.ai-level',String(allAIDifficulty));}catch{}}});
 function resetSettings(){settings=clone(GameSettingsSchema,defaults);settings.name=randomGameName();allAIDifficulty=AiDifficulty.NORMAL;}
 let aiCount=$derived(settings.players.filter(p=>p.type===NewGamePlayerType.AI).length);
 function setAICount(count:number){
  count=Math.max(0,Math.min(144,count));
  const humans=settings.players.filter(p=>p.type!==NewGamePlayerType.AI);
  const bots=settings.players.filter(p=>p.type===NewGamePlayerType.AI).slice(0,count);
  settings.players=[...humans,...bots];
  while(settings.players.length<humans.length+count) addPlayer();
 }

	let allAIDifficulty = $state(AiDifficulty.NORMAL);
	function changeAllAIDifficulty() {
		for (const player of settings.players) if (player.type===NewGamePlayerType.AI) player.aiDifficulty=allAIDifficulty;
	}
	function varyAIDifficulty() {
  for(const player of settings.players) if(player.type===NewGamePlayerType.AI) player.aiDifficulty=variedDifficulty(allAIDifficulty);
 }
 let suggestedEmpires = $derived(recommendedEmpires(settings.size));
	function fillMapWithAI() {
		const total = suggestedEmpires;
		if (!total) return;
		while (settings.players.length < total) addPlayer();
	}

	const onSubmit = async () => {
		submitting = true;
		createError = '';
		try {
			if (settings.size>=Size.VAST) {
				const response=await fetch('/api/grpc/craig_stars.v1.UserService/GetMe',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'});
				if (!response.headers.get('X-App-Capabilities')?.includes('expanded-maps')) throw new Error('An older game server is running and does not support expanded maps. Restart the game using the tomek-stars shortcut.');
			}
			const resp = await gameClient.createGame({ settings: settings });
			if (resp.game?.game) {
				await goto(`/games/${resp.game.game.id}`);
			}
		} catch (error) {
			createError=error instanceof Error ? error.message : 'Could not create the game.';
		} finally {
			submitting = false;
		}
	};

	const addPlayer = () => {
		const usedColors = new Set<string>(settings.players.map<string>((p) => p.color));

		settings.players = [
			...settings.players,
			create(NewGamePlayerSchema, {
				type: NewGamePlayerType.AI,
				color: getFirstAvailableColor(usedColors),
				aiDifficulty: allAIDifficulty,
				defaultHullSet: 0
			})
		];
	};

	const removePlayer = (player: GamePlayer) => {
		settings.players = settings.players.filter((p) => p !== player);
	};
</script>

<form
	onsubmit={(e) => {
		e.preventDefault();
		onSubmit();
	}}
>
	<div class="w-full flex justify-end gap-2">
		<button type="submit" disabled={submitting} class="btn btn-success"
			>{submitting ? 'Creating...' : 'Create Game'}</button
		>
	</div>

	<ItemTitle>New Game</ItemTitle>
	{#if createError}<div class="alert alert-error" role="alert">{createError}</div>{/if}

	<GameSettingsEditor bind:settings allowRandomName />
	{#if suggestedEmpires}
		<p class="text-sm my-2">This map supports a population preset of {suggestedEmpires} empires, proportional to its area. Existing players are kept; remaining slots are filled with AI. You can still edit or remove opponents.</p>
		<button type="button" class="btn btn-sm my-2" disabled={settings.players.length >= suggestedEmpires} onclick={fillMapWithAI}>Fill map to {suggestedEmpires} empires</button>
	{/if}

 <label class="block my-3">AI opponents: {aiCount}<input aria-label="AI opponents" class="range w-full" type="range" min="0" max="144" step="1" value={aiCount} oninput={e=>setAICount(Number(e.currentTarget.value))} /></label>
 <label class="block my-3">Starting location <select aria-label="Starting location" class="select" bind:value={settings.hostStartPosition}><option value="">Random</option><option value="edge">Edge</option><option value="center">Center</option></select></label>
 <p class="text-xs opacity-75 mb-3">Your capital only: Random uses normal placement; Edge selects a planet around 5% of the map width or height from the nearest border; Center selects one near the middle. The closest available planet is used.</p>
 <SectionHeader>
  <button type="button" class="btn-ghost w-full flex flex-row" onclick={addPlayer}>
			Players
			<div class="ml-auto">
				<Icon src={PlusCircle} size="24" class="hover:stroke-accent" />
			</div>
		</button></SectionHeader
	>

	<div class="flex flex-wrap gap-2 items-center my-3">
		<label>Difficulty for all AI <select class="select select-sm" aria-label="Difficulty for all AI" bind:value={allAIDifficulty}>{#each difficultyLevels as level (level.value)}<option value={level.value}>{level.label}</option>{/each}</select></label>
		<button type="button" class="btn btn-sm" onclick={changeAllAIDifficulty}>Apply to all AI</button>
  <button type="button" class="btn btn-sm" onclick={varyAIDifficulty}>Vary AI difficulty</button>
  <p class="text-xs">Variation uses the selected level as its centre: unchanged and ±1 are most common; ±2, ±3 and ±4 become progressively rarer. Values stay within Easy–Cheater. Click again to reroll.</p>
  <details class="text-xs mt-2"><summary>How AI difficulty works</summary>
   <p class="my-2">Difficulty changes expansion, production planning and attack timing. All levels use legal game orders. Pace controls how much the AI tries to build; it does not create free resources. Attack offsets are relative to Normal before racial personality adjustments.</p>
   <div class="overflow-x-auto"><table class="table table-xs"><thead><tr><th>Level</th><th>Colony ships in flight</th><th>Production pace</th><th>Attack timing (years)</th><th>Research reserve (%)</th></tr></thead><tbody>{#each difficultyDetails as level (level.name)}<tr><td>{level.name}</td><td>{level.colonizers}</td><td>{level.pace}</td><td>{level.attack}</td><td>{level.research}</td></tr>{/each}</tbody></table></div>
   <p class="my-2">Elite uses Hard strategy with partial factory, mining and population-efficiency improvements, preserving stronger existing racial advantages. Cheater uses Hard strategy and specially strengthened race presets. Hard and higher also prioritize racial capabilities and economic research.</p>
  </details>
	</div>

	{#each settings.players as player, i (i)}
		<NewGamePlayer
			bind:player={settings.players[i]}
			index={i + 1}
			onRemove={() => removePlayer(player)}
		/>
	{/each}


	<SectionHeader>Victory Conditions</SectionHeader>
	<VictoryConditions bind:settings />
 <button type="button" class="btn mt-4" onclick={resetSettings}>Reset to defaults</button>
</form>


