<script lang="ts">
 import BattleLosses from '#lib/components/game/battle/BattleLosses.svelte';
	import { goto } from '$app/navigation';
	import Breadcrumb from '#lib/components/game/Breadcrumb.svelte';
	import SortableTableHeader from '#lib/components/table/SortableTableHeader.svelte';
	import Table from '#lib/components/table/Table.svelte';
	import type { TableColumn } from '#lib/components/table/Table.js';
	import TableSearchInput from '#lib/components/table/TableSearchInput.svelte';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { type BattleRecordDetails } from '#lib/types/Battle.js';
	import { Check } from '@steeze-ui/heroicons';
	import { Icon } from '@steeze-ui/svelte-icon';

	const { game, player, universe, settings, gotoBattle } = getGameContext();

	// filterable battles
	let search = $state('');

	let filteredBattles = $derived(
		$universe
			.getBattles($settings.sortBattlesKey, $settings.sortBattlesDescending, $player)
			.filter((i) => i.location.toLowerCase().indexOf(search.toLowerCase()) != -1)
	);

let totals=$derived.by(()=>({
 own:filteredBattles.reduce((n,b)=>n+b.ourDead,0),enemy:filteredBattles.reduce((n,b)=>n+b.theirDead,0),
 ownMass:filteredBattles.some(b=>b.ourDeadTonnage===undefined)?undefined:filteredBattles.reduce((n,b)=>n+(b.ourDeadTonnage??0),0),
 enemyMass:filteredBattles.some(b=>b.theirDeadTonnage===undefined)?undefined:filteredBattles.reduce((n,b)=>n+(b.theirDeadTonnage??0),0)
 }));
	const columns: TableColumn<BattleRecordDetails>[] = [
		{
			key: 'location',
			title: 'Location'
		},
		{
			key: 'present',
			title: 'Present',
			hidden: $player.getAllies().length === 1 // hide if we're only friends with ourself
		},
		{
			key: 'numPlayers',
			title: 'Players'
		},
		{
			key: 'numShips',
			title: 'Ships'
		},
		{
			key: 'ours',
			title: 'Ours'
		},
		{
			key: 'theirs',
			title: 'Theirs'
		},
		{
			key: 'ourDead',
			title: 'Our Dead'
		},
		{
			key: 'theirDead',
			title: 'Their Dead'
		},
		{
			key: 'oursLeft',
			title: 'Ours Left'
		},
		{
			key: 'theirsLeft',
			title: 'Theirs Left'
		}
	];

	function gotoTarget(row: BattleRecordDetails) {
		gotoBattle(row.num);
		goto(`/games/${$game.id}`);
	}

	function onSorted(column: TableColumn<BattleRecordDetails>, sortDescending: boolean) {
		$settings.sortBattlesDescending = sortDescending;
		$settings.sortBattlesKey = column.key;
	}
</script>

<Breadcrumb>
	{#snippet crumbs()}
		<li>Battles</li>
	{/snippet}
</Breadcrumb>

<div class="w-full">
	<div class="flex flex-row justify-between m-2">
		<TableSearchInput bind:value={search} />
	</div>
	<Table
		{columns}
		rows={filteredBattles}
		externalSortAndFilter={true}
		classes={{
			table: 'table table-zebra table-compact table-auto w-full',
			th: 'sticky top-0 bg-base-200 z-10'
		}}
	>
		{#snippet head({ column })}
			<span>
				<SortableTableHeader
					{column}
					isSorted={$settings.sortBattlesKey === column.key}
					sortDescending={$settings.sortBattlesDescending}
					{onSorted}
				/>
			</span>
		{/snippet}

		{#snippet cell({ column, row, cell })}
			<span>
				{#if column.key == 'location'}
					<div class="flex flex-row justify-between">
						<a class="cs-link text-xl text-left" href={`/games/${$game.id}/battles/${row.num}`}
							>{row.location}</a
						>
						<button
							type="button"
							onclick={() => gotoTarget(row)}
							class="btn btn-outline btn-sm normal-case btn-secondary p-2 mx-1"
							title="goto">Goto</button
						>
					</div>
				{:else if column.key == 'ourDead'}
                    <span class="text-error font-semibold">{cell ?? 0} ({row.ourDeadTonnage===undefined?'—':row.ourDeadTonnage.toLocaleString('en-US')} kT)</span>
                {:else if column.key == 'theirDead'}
                    <span class="text-success font-semibold">{cell ?? 0} ({row.theirDeadTonnage===undefined?'—':row.theirDeadTonnage.toLocaleString('en-US')} kT)</span>
                {:else if column.key == 'present'}
					{#if row.present}
						<Icon src={Check} size="24" class="stroke-success" />
					{/if}
				{:else}
					{cell ?? ''}
				{/if}
			</span>
		{/snippet}
	</Table>
</div>

<section class="mt-4 rounded border border-base-content/30 bg-base-200 p-4" aria-label="Battle report totals">
 <h2 class="font-bold text-lg">Battle summary</h2>
 <p>{filteredBattles.length} battles in this report.</p>
 <BattleLosses own={totals.own} enemy={totals.enemy} ownMass={totals.ownMass} enemyMass={totals.enemyMass}/>
</section>
