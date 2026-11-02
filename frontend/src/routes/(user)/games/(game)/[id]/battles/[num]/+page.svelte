<script lang="ts">
 import BattleLosses from '#lib/components/game/battle/BattleLosses.svelte';
 import {getBattleRecordDetails} from '#lib/types/Battle.js';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Breadcrumb from '#lib/components/game/Breadcrumb.svelte';
	import BattleView from '#lib/components/game/battle/BattleView.svelte';
	import { getGameContext } from '#lib/services/GameContext.js';

	const { game, universe, player, gotoBattle } = getGameContext();
	let num = parseInt(page.params.num || '0');

	let battle = $derived($universe.getBattle(num));
 let details=$derived(battle?getBattleRecordDetails(battle,$player,$universe):undefined);

	function gotoTarget() {
		if (!battle) {
			return;
		}
		gotoBattle(battle.num);
		goto(`/games/${$game.id}`);
	}
</script>

{#if battle}
	<Breadcrumb>
		{#snippet crumbs()}
			<li><a class="cs-link" href={`/games/${$game.id}/battles`}>Battles</a></li>
			<li class="flex flex-row gap-1">
				{$universe.getBattleLocation(battle)}
				<button
					onclick={gotoTarget}
					class="btn btn-outline btn-sm normal-case btn-secondary p-2"
					title="goto">Goto</button
				>
			</li>
		{/snippet}
	</Breadcrumb>

	<div class="grow px-1">
		<BattleView playerFinder={$universe} designFinder={$universe} battleRecord={battle} />
	</div>
 {#if details}<section class="mt-4 rounded border border-base-content/30 bg-base-200 p-4" aria-label="Battle result summary"><h2 class="font-bold text-lg">Battle summary</h2><BattleLosses own={details.ourDead} enemy={details.theirDead} ownMass={details.ourDeadTonnage} enemyMass={details.theirDeadTonnage}/><p class="mt-2">Remaining: ours {details.oursLeft}, enemy {details.theirsLeft}.</p></section>{/if}
{/if}
