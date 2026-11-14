<script lang="ts">
	import type { HullStatistics, PlayerStats, StatisticsCounters } from '#lib/types/cs-proto.js';
	import HelpHint from './HelpHint.svelte';

	let { stats }: { stats?: PlayerStats } = $props();
	type HullMetric = Exclude<keyof HullStatistics, '$typeName' | '$unknown'>;
	type Counter = Exclude<keyof StatisticsCounters, '$typeName' | '$unknown' | 'hulls'>;
	type Row = { key: Counter; label: string; unit?: string; hull?: HullMetric };
	const hullRow = (key: HullMetric, label: string, unit?: string): Row => ({
		key, label, unit, hull: key
	});
	const groups: { title: string; rows: Row[] }[] = [
		{ title: 'Combat', rows: [
			hullRow('shipsDestroyed', 'Enemy ships destroyed'),
			hullRow('destroyedTonnage', 'Enemy tonnage destroyed', 'kT'),
			hullRow('shipsLost', 'Own ships lost'),
			hullRow('lostTonnage', 'Own tonnage lost', 'kT'),
			{ key: 'battlesFought', label: 'Battles fought' },
			{ key: 'battlesWon', label: 'Battles won' }
		]},
		{ title: 'Construction and recycling', rows: [
			hullRow('shipsBuilt', 'Ships built'),
			hullRow('builtTonnage', 'Built ship tonnage', 'kT'),
			{ key: 'factoriesBuilt', label: 'Factories built' },
			{ key: 'minesBuilt', label: 'Mines built' },
			{ key: 'defensesBuilt', label: 'Defenses built' },
			{ key: 'shipsScrapped', label: 'Ships scrapped' },
			{ key: 'scrappedTonnage', label: 'Scrapped tonnage', unit: 'kT' }
		]},
		{ title: 'Economy', rows: [
			{ key: 'resourcesProduced', label: 'Resources produced' },
			{ key: 'productionResourcesSpent', label: 'Resources allocated to production' },
			{ key: 'allocationRefundsResources', label: 'Production allocation refunds' },
			{ key: 'researchResourcesSpent', label: 'Research points after race modifiers' }
		]},
		{ title: 'Mining and salvage', rows: [
			{ key: 'ironiumMined', label: 'Ironium mined', unit: 'kT' },
			{ key: 'boraniumMined', label: 'Boranium mined', unit: 'kT' },
			{ key: 'germaniumMined', label: 'Germanium mined', unit: 'kT' }
		]}
	];
	const mineralSpending: Row[] = [
		{ key: 'ironiumSpent', label: 'Ironium spent', unit: 'kT' },
		{ key: 'boraniumSpent', label: 'Boranium spent', unit: 'kT' },
		{ key: 'germaniumSpent', label: 'Germanium spent', unit: 'kT' }
	];
	const salvageRows: Row[] = [
		{ key: 'ironiumSalvaged', label: 'Ironium recovered', unit: 'kT' },
		{ key: 'boraniumSalvaged', label: 'Boranium recovered', unit: 'kT' },
		{ key: 'germaniumSalvaged', label: 'Germanium recovered', unit: 'kT' }
	];
	const formatter = new Intl.NumberFormat('en-US');
	const format = (value?: bigint | number) => formatter.format(value ?? 0);
	const value = (c: StatisticsCounters | undefined, key: Counter) => c?.[key] ?? 0n;
	const battleSalvage = (c?: StatisticsCounters) =>
		salvageRows.reduce((sum, row) => sum + value(c, row.key), 0n);

	const hullsFor = (c: StatisticsCounters | undefined, key: HullMetric) =>
		Object.entries(c?.hulls ?? {})
			.map(([hull, metrics]) => ({ hull, amount: metrics[key] }))
			.filter(({ amount }) => amount > 0n)
			.sort((a, b) => a.amount === b.amount ? a.hull.localeCompare(b.hull) : a.amount > b.amount ? -1 : 1);

	let totals = $derived(stats?.accounting?.counters);
	let history = $derived([...(stats?.accounting?.history ?? [])].reverse());
	let legacySalvage = $derived(value(totals, 'mineralsSalvaged') - battleSalvage(totals));
</script>

{#snippet metric(row: Row)}
	<div class="flex justify-between gap-4">
		<dt>{row.label}</dt>
		<dd class="font-mono font-bold text-right" data-stat={row.key}>
			{format(value(totals, row.key))}{row.unit ? ' ' + row.unit : ''}
		</dd>
	</div>
{/snippet}

<div class="space-y-5">
	<p class="text-base-content/90">
		{#if stats?.accounting?.sinceYear}
			Detailed counters tracked from year <strong>{stats.accounting.sinceYear}</strong>.
		{:else}
			Detailed tracking starts with the next generated turn.
		{/if}
	</p>
	<p>Previously recorded production: <strong>{format(Math.max(0, (stats?.tokensBuilt ?? 0) - (stats?.starbasesBuilt ?? 0)))}</strong> ships and <strong>{format(stats?.starbasesBuilt)}</strong> starbases. Colonies founded: <strong>{format(stats?.planetsColonized)}</strong>.</p>
	<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
		{#each groups as group (group.title)}
			<section class="rounded border border-base-content/25 bg-base-200 p-4">
				<h2 class="mb-3 flex items-center gap-2 text-lg font-bold">
					{group.title}
					{#if group.title === 'Combat'}
						<HelpHint label="Combat statistics" text="Kills go to the player whose attack destroyed the ship. Combat totals include orbital bases. Tonnage is the design mass without cargo." />
					{:else if group.title === 'Economy'}
						<HelpHint label="Economy statistics" text="Production counts new allocations, including unfinished orders. Recycling credits and allocation refunds can supplement a year's planetary output. Research points include race modifiers." />
					{/if}
				</h2>
				{#each group.rows as row (row.key)}
					{#if row.hull}
						{@const breakdown = hullsFor(totals, row.hull)}
						{@const unclassified = value(totals, row.key) - breakdown.reduce((sum, entry) => sum + entry.amount, 0n)}
						<details class="mb-2">
							<summary class="cursor-pointer rounded focus-visible:outline-2">
								<span class="inline-flex w-[calc(100%-1.4rem)] justify-between gap-3 align-top">
									<span>{row.label}</span>
									<strong class="text-right font-mono" data-stat={row.key}>{format(value(totals, row.key))}{row.unit ? ' ' + row.unit : ''}</strong>
								</span>
							</summary>
							<dl class="ml-4 mt-1 space-y-1 border-l border-base-content/25 pl-3 text-sm">
								{#each breakdown as { hull, amount } (hull)}
									<div class="flex justify-between gap-2"><dt>{hull}</dt><dd class="font-mono">{format(amount)}{row.unit ? ' ' + row.unit : ''}</dd></div>
								{/each}
								{#if unclassified > 0n}
									<div class="flex justify-between gap-2"><dt>Hull not recorded</dt><dd class="font-mono">{format(unclassified)}{row.unit ? ' ' + row.unit : ''}</dd></div>
								{:else if !breakdown.length}
									<div class="text-base-content/75">No hulls recorded.</div>
								{/if}
							</dl>
						</details>
					{:else}
						<dl class="mb-2">{@render metric(row)}</dl>
					{/if}
				{/each}
				{#if group.title === 'Economy'}
					<h3 class="mt-4 mb-2 flex items-center gap-2 font-semibold">
						Minerals spent
						<HelpHint label="Minerals spent" text="Building can use starting stocks, salvaged minerals and minerals returned by scrapping ships. The same minerals can be spent again after recycling." example="Mine 100 kT, use it to build, then scrap and reuse 50 kT. Spending reaches 150 kT while mining stays at 100 kT." />
					</h3>
					<dl class="space-y-2">
						{#each mineralSpending as row (row.key)}{@render metric(row)}{/each}
					</dl>
				{:else if group.title === 'Mining and salvage'}
					<h3 class="mt-4 mb-2 flex items-center gap-2 font-semibold">
						Battle debris recovered
						<HelpHint label="Battle debris recovered" text="Counts minerals loaded from battle debris by fleets or deposited directly on an owned planet after a battle. Debris still floating in space is counted when recovered. The mineral breakdown starts with newly recorded battles." />
					</h3>
					<dl class="space-y-2">
						<div class="flex justify-between gap-4"><dt>Total battle debris</dt><dd class="font-mono font-bold">{format(battleSalvage(totals))} kT</dd></div>
						{#each salvageRows as row (row.key)}{@render metric(row)}{/each}
						{#if legacySalvage > 0n}
							<div class="flex justify-between gap-4 text-sm"><dt>Earlier salvage, source unclassified</dt><dd class="font-mono">{format(legacySalvage)} kT</dd></div>
						{/if}
					</dl>
				{/if}
			</section>
		{/each}
	</div>
	<section class="overflow-x-auto rounded border border-base-content/25 bg-base-200 p-4">
		<h2 class="mb-3 text-lg font-bold">Yearly history</h2>
		{#if history.length}
			<table class="table table-sm">
				<thead><tr><th>Year</th><th>Built</th><th>Destroyed</th><th>Lost</th><th>Production resources</th><th>Research points</th><th>Minerals mined (kT)</th><th>Battle debris recovered (kT)</th></tr></thead>
				<tbody>
					{#each history as { year, counters } (year)}
						<tr>
							<td>{year}</td>
							<td>{format(value(counters, 'shipsBuilt'))}</td>
							<td class="text-success">{format(value(counters, 'shipsDestroyed'))}</td>
							<td class="text-error">{format(value(counters, 'shipsLost'))}</td>
							<td>{format(value(counters, 'productionResourcesSpent'))}</td>
							<td>{format(value(counters, 'researchResourcesSpent'))}</td>
							<td>{format(value(counters, 'ironiumMined') + value(counters, 'boraniumMined') + value(counters, 'germaniumMined'))}</td>
							<td>{format(battleSalvage(counters))}</td>
						</tr>
					{/each}
				</tbody>
			</table>
			<p class="mt-2 text-sm">Up to 200 recent years are retained. Lifetime counters continue accumulating.</p>
		{:else}
			<p>No completed year recorded yet.</p>
		{/if}
	</section>
</div>
