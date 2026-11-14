<script lang="ts">
	import { goto } from '$app/navigation';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { dedicatedMinesweeper } from '#lib/types/automationRoles.js';
	import type { Fleet } from '#lib/types/cs-proto.js';
	import { getMapObjectName } from '#lib/types/MapObject.js';

	const { game, player, universe, commandMapObject, zoomToMapObject } = getGameContext();
	const number = new Intl.NumberFormat();

	let rows = $derived.by(() => {
		const roles = $player.playerOrders.automation?.fleetRoles ?? {};
		const designs = new Map($universe.getMyDesigns().map((design) => [design.num, design.spec]));
		return $universe.getMyFleets('num', false).flatMap((fleet) => {
			const object = fleet.mapObject;
			if (!object || (roles[object.num] !== 'minesweeper' &&
				!fleet.tokens.some((token) => dedicatedMinesweeper(designs.get(token.designNum))))) {
				return [];
			}
			const result = /^Year (\d+) result: (\d+) mines removed/.exec(object.tags['last-mine-sweep'] ?? '');
			const mines = result && Number(result[1]) === $game.year ? Number(result[2]) : 0;
			return [{ fleet, num: object.num, name: getMapObjectName(fleet), mines }];
		}).sort((a, b) => b.mines - a.mines || a.num - b.num);
	});
	let total = $derived(rows.reduce((sum, row) => sum + row.mines, 0));

	function goToFleet(fleet: Fleet) {
		commandMapObject(fleet);
		zoomToMapObject(fleet);
		void goto(`/games/${$game.id}`);
	}
</script>

<section class="mx-auto max-w-3xl p-4" aria-labelledby="mine-sweeping-heading">
	<h1 id="mine-sweeping-heading" class="text-2xl font-bold">Mine Sweeping</h1>
	<div class="my-4 rounded-box bg-base-200 p-4">
		<p class="text-sm opacity-70">Year {$game.year} · Mines removed by your minesweepers</p>
		<p class="mt-1 text-3xl font-bold tabular-nums">{number.format(total)}</p>
	</div>

	{#if rows.length === 0}
		<p>No minesweepers available.</p>
	{:else}
		<ul class="grid gap-2">
			{#each rows as row (row.num)}
				<li class="flex items-center gap-3 rounded-box border border-base-300 p-3">
					<span class="min-w-0 flex-1 break-words font-medium">{row.name}</span>
					<span class="shrink-0 text-sm tabular-nums">{number.format(row.mines)} mines</span>
					<button type="button" class="btn btn-square btn-primary shrink-0"
						aria-label={`Go! ${row.name}`} onclick={() => goToFleet(row.fleet)}>Go!</button>
				</li>
			{/each}
		</ul>
	{/if}
</section>
