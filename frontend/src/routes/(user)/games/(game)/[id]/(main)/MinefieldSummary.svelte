<script lang="ts">
	import HelpHint from '#lib/components/game/HelpHint.svelte';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { type Minefield, type MinefieldSpec, MinefieldType } from '#lib/types/cs-proto.js';
	import { enumToString } from '#lib/types/Enums.js';
	import { ownedBy } from '#lib/types/MapObject.js';
	import type { ChangeEventHandler } from 'svelte/elements';

	const { cs, game, player, universe, updateMinefieldOrders } = getGameContext();

	type Props = {
		minefield: Minefield;
	};

	let { minefield = $bindable() }: Props = $props();

	let spec: MinefieldSpec | undefined = $state();

	let stats = $derived($game.rules.minefieldStatsByType[minefield.minefieldType]);

	$effect(() => {
		cs.wasmService.computeMinefieldSpec({ minefield: minefield as Minefield }).then((resp) => {
			spec = resp.spec;
		});
	});


	// update the minefield to detonate on the server
	const minefieldDetonateChecked: ChangeEventHandler<HTMLInputElement> = async (e) => {
		if (minefield.minefieldOrders) {
			minefield.minefieldOrders.detonate = e.currentTarget.checked;
			await updateMinefieldOrders(minefield);
		} else {
			console.error("can't detonate minefield not owned by player");
		}
	};
</script>

<div class="flex flex-row md:min-h-44">
	<div class="flex flex-col">
		<div class="avatar">
			<div class="mapobject-avatar-wrapper">
				<div
					class:standard-minefield={minefield.minefieldType === MinefieldType.STANDARD}
					class:heavy-minefield={minefield.minefieldType === MinefieldType.HEAVY}
					class:speed-bump-minefield={minefield.minefieldType === MinefieldType.SPEED_BUMP}
					class="mapobject-avatar"
				></div>
			</div>
		</div>
		<div class="text-center">{$universe.getPlayerPluralName(minefield.mapObject?.playerNum)}</div>
	</div>

	<div class="flex flex-col grow">
		<div class="flex flex-row">
			<div class="w-40">Location:</div>
			<div>
				({minefield.mapObject?.position?.x ?? 0}, {minefield.mapObject?.position?.y ?? 0})
			</div>
		</div>
		<div class="flex flex-row">
			<div class="w-40">Field Type:</div>
			<div>
				{enumToString(MinefieldType, minefield.minefieldType)}
			</div>
		</div>
		<div class="flex flex-row">
			<div class="w-40">Field Radius:</div>
			<div>
				{Math.sqrt(minefield.numMines).toFixed()} l.y. ({minefield.numMines} mines)
			</div>
		</div>
		<div class="flex flex-row">
			<div class="w-40">Maximum Safe Speed:</div>
			<div>
				Warp {stats.maxSpeed}
			</div>
		</div>
		<div class="flex flex-row">
			<div class="w-40">Chance/l.y. of a Hit:</div>
			<div>
				{(stats.chanceOfHit * 100).toFixed(2)}%
				<HelpHint label="Mine hit chance" text="The chance of a hit per light-year rises for each warp step above the field’s safe speed." example="At Warp 6 through a Warp 4 field, you are two warp steps above the safe speed." />
			</div>
		</div>
		<div class="flex flex-row">
			<div class="w-40">Dmg done to each ship:</div>
			<div>
				{stats.damagePerEngine} ({stats.damagePerEngineRs}) / engine
				<HelpHint label="Mine damage per engine" text="Parentheses show the damage for fleets with ram-scoop engines. The first number applies to other fleets." />
			</div>
		</div>
		<div class="flex flex-row">
			<div class="w-40">Min damage done to fleet:</div>
			<div>
				{stats.minDamagePerFleet} ({stats.minDamagePerFleetRs})
				<HelpHint label="Minimum mine damage" text="A mine hit deals at least this much damage to the fleet. Parentheses apply to fleets with ram-scoop engines." />
			</div>
		</div>
		{#if spec && ownedBy(minefield, $player.num)}
			<div class="flex flex-row">
				<div class="w-40">Decay Rate:</div>
				<div>
					{spec.decayRate} / year
				</div>
			</div>
			{#if spec.canDetonate}
				<div class="flex flex-row mt-2">
					<label>
						<input
							checked={minefield.minefieldOrders?.detonate}
							onchange={minefieldDetonateChecked}
							class="checkbox checkbox-xs"
							type="checkbox"
						/> Detonate
					</label>
					<HelpHint label="Detonate minefield" text="Detonation consumes mines and damages ships inside the field, including friendly ships. Friendly minelayers are exempt." />
				</div>
			{/if}
		{/if}
	</div>
</div>
