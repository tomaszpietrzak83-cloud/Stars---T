<script lang="ts">
	import HelpHint from '#lib/components/game/HelpHint.svelte';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { automationRoles, designAutoRole } from '#lib/types/automationRoles.js';
	import { create } from '@bufbuild/protobuf';
	import { FleetSchema, type ShipProductionResponse } from '#lib/types/cs-proto.js';
	const { game, player, universe, readOnly, previewShipProduction, queueShipProduction } =
		getGameContext();
	let designs = $derived(
		$universe
			.getMyDesigns()
			.filter(
				(d) =>
					!d.spec?.starbase &&
					!d.originalPlayerNum &&
					!$player.playerOrders.designMaintenance[d.num]
			)
	);
	let designNum = $state(0),
		quantity = $state(1),
		role = $state('__design__');
	let plan = $state<ShipProductionResponse>(),
		pending = $state(false),
		saving = $state(false),
		error = $state(''),
		success = $state('');
	let request = 0;
	let refresh = $state(0);
	const minerals = [
		{ key: 'ironium', short: 'I', label: 'Ironium', color: 'text-ironium' },
		{ key: 'boranium', short: 'B', label: 'Boranium', color: 'text-boranium' },
		{ key: 'germanium', short: 'G', label: 'Germanium', color: 'text-germanium' }
	] as const;
	type MineralKey = (typeof minerals)[number]['key'];
	type SortKey = MineralKey | 'resources' | 'status';
	type SortDirection = 'ascending' | 'descending';
	let sortKey = $state<SortKey>();
	let sortDirection = $state<SortDirection>('descending');
	const sortStorageKey = $derived(`cs.shipProduction.sort.${$game.id}.${$player.num}`);
	$effect(() => {
		const storageKey = sortStorageKey;
		let saved: string | null = null;
		try {
			saved = sessionStorage.getItem(storageKey);
		} catch {
			// Sorting is still available when browser storage is disabled.
		}
		const [key, direction] = saved?.split(':') ?? [];
		sortKey = key === 'resources' || key === 'status'
			? key
			: minerals.find((mineral) => mineral.key === key)?.key;
		sortDirection = direction === 'ascending' ? 'ascending' : 'descending';
	});

	function sortValue(yard: ShipProductionResponse['yards'][number], key: SortKey) {
		if (key === 'resources') return yard.planet?.spec?.resourcesPerYearAvailable;
		if (key === 'status') return yard.reason || undefined;
		return yard.planet?.cargo?.[key];
	}

	let sortedYards = $derived.by(() => {
		const yards = plan?.yards ?? [];
		const key = sortKey;
		const direction = sortDirection === 'ascending' ? 1 : -1;
		if (!key) return yards;
		return yards.toSorted((a, b) => {
			const left = sortValue(a, key);
			const right = sortValue(b, key);
			if (left === undefined) return right === undefined ? 0 : 1;
			if (right === undefined) return -1;
			const comparison = typeof left === 'number' && typeof right === 'number'
				? left - right
				: String(left).localeCompare(String(right));
			return direction * comparison;
		});
	});
	function sortBy(key: SortKey) {
		if (sortKey === key) {
			sortDirection = sortDirection === 'descending' ? 'ascending' : 'descending';
		} else {
			sortDirection = key === 'status' ? 'ascending' : 'descending';
		}
		sortKey = key;
		try {
			sessionStorage.setItem(sortStorageKey, `${key}:${sortDirection}`);
		} catch {
			// Keep the current sort even when browser storage is disabled.
		}
	}
	let design = $derived(designs.find((d) => d.num === designNum));
	let roles = $derived(
		automationRoles(
			create(FleetSchema, {
				spec: { shipDesignSpec: design?.spec },
				tokens: design ? [{ designNum: design.num, quantity: 1 }] : []
			}),
			design ? [design] : []
		)
	);
	let valid = $derived(
		designNum > 0 && Number.isInteger(quantity) && quantity > 0 && quantity <= 100000
	);
	$effect(() => {
		if (!designs.some((d) => d.num === designNum)) {
			designNum = designs[0]?.num ?? 0;
			role = '__design__';
		}
	});
	function roleValue() {
		return role === '__design__' ? '' : role || 'manual';
	}
	$effect(() => {
		const num = designNum,
			count = quantity,
			chosen = role;
		const year = $game.year;
		const revision = refresh;
		const id = ++request;
		plan = undefined;
		error = '';
		if (!num || !Number.isInteger(count) || count < 1 || count > 100000) {
			pending = false;
			return;
		}
		pending = true;
		const timer = setTimeout(async () => {
			try {
				const result = await previewShipProduction(
					num,
					count,
					chosen === '__design__' ? '' : chosen || 'manual'
				);
				if (id === request) plan = result;
			} catch (cause) {
				if (id === request) error = String(cause);
			} finally {
				if (id === request) pending = false;
			}
		}, 180);
		return () => clearTimeout(timer);
	});
	async function fill() {
		const id = ++request;
		pending = true;
		error = '';
		plan = undefined;
		try {
			const result = await previewShipProduction(designNum, 100000, roleValue(), true);
			if (id === request) {
				quantity = Math.max(1, result.assignedQuantity);
				if (!result.assignedQuantity) error = 'No free shipbuilding capacity this year.';
			}
		} catch (cause) {
			if (id === request) error = String(cause);
		} finally {
			if (id === request) pending = false;
		}
	}
	async function apply() {
		if (!plan || pending || saving || $readOnly) return;
		saving = true;
		error = '';
		success = '';
		try {
			const result = await queueShipProduction(designNum, quantity, roleValue(), plan.sourceHash);
			success = `Queued ${result.assignedQuantity} ships: ${result.thisYearQuantity} this year, ${result.assignedQuantity - result.thisYearQuantity} later.`;
			plan = undefined;
			quantity = 1;
			request++;
			pending = false;
			refresh++;
		} catch (cause) {
			error = String(cause);
		} finally {
			saving = false;
		}
	}
	function changeQuantity(delta: number) {
		quantity = Math.max(1, Math.min(100000, quantity + delta));
		success = '';
	}
</script>

{#snippet sortHeader(key: SortKey, label: string, short?: string)}
	<th scope="col" aria-sort={sortKey === key ? sortDirection : undefined}
		class:w-px={short} class:px-1={short}>
		<button type="button" class="btn btn-ghost btn-xs whitespace-nowrap" class:px-1={short}
			aria-label={short ? label : undefined} title={short ? label : undefined}
			onclick={() => sortBy(key)}>
			{short ?? label}
			<span aria-hidden="true">{sortKey === key
				? (sortDirection === 'descending' ? '↓' : '↑')
				: '↕'}</span>
		</button>
	</th>
{/snippet}

<section class="min-w-0 w-full p-3" aria-label="Empire ship production">
	<h1 class="text-2xl font-semibold flex items-center gap-2">Ship production <HelpHint label="Ship production" text="Review ship construction across colonies and place orders at eligible shipyards. Dock capacity limits which designs a yard can build." /></h1>
	<p class="my-2">
		Distribute new ships across your existing bases. Green batches finish this year; blue batches
		need longer. Current concrete and paid orders stay ahead of new orders.
	</p>
	<div class="grid grid-cols-1 sm:grid-cols-3 gap-3 my-4">
		<label
			>Ship design<select
				class="select select-bordered w-full"
				aria-label="Production ship design"
				bind:value={designNum}
				disabled={saving}
				onchange={() => {
					role = '__design__';
					success = '';
				}}
				>{#each designs as item (item.num)}<option value={item.num}>{item.name}</option
					>{/each}</select
			></label
		>
		<label
			>Quantity<input
				class="input input-bordered w-full"
				aria-label="Total ship quantity"
				type="number"
				min="1"
				max="100000"
				step="1"
				bind:value={quantity}
				disabled={saving}
			/></label
		>
		<label
			>Auto orders after construction<select
				class="select select-bordered w-full"
				aria-label="Production Auto orders"
				bind:value={role}
				disabled={saving}
			>
				<option value="__design__"
					>Design default ({designAutoRole(design?.spec) || 'Manual'})</option
				>
				{#each roles as option (option.value)}<option value={option.value}>{option.label}</option
					>{/each}
			</select></label
		>
	</div>
	<div class="flex flex-wrap gap-2 items-center my-3">
		{#each [-10, -1, 1, 10, 100] as delta}<button
				type="button"
				class="btn btn-sm"
				disabled={saving || !design}
				onclick={() => changeQuantity(delta)}>{delta > 0 ? '+' : ''}{delta}</button
			>{/each}
		<button type="button" class="btn btn-sm" disabled={saving || pending || !design} onclick={fill}
			>Fill this year's capacity</button
		>
		<button
			type="button"
			class="btn btn-primary"
			disabled={$readOnly || saving || pending || !valid || !plan?.assignedQuantity}
			onclick={apply}>Queue ships</button
		>
	</div>
	<p class="text-sm">
		The chosen Auto orders apply to this new production only. Estimates include current queues,
		minerals, mining and the production budget.
	</p>
	{#if pending}<p role="status" class="my-3">Updating production estimate…</p>{/if}
	{#if error}<p role="alert" class="text-error my-3">{error}</p>{/if}
	{#if success}<p role="status" class="text-success my-3">{success}</p>{/if}
	{#if plan}
		<div class="flex flex-wrap gap-4 my-3" aria-label="Production allocation totals">
			<strong>{plan.assignedQuantity} / {quantity} assigned</strong>
			<span class="text-success">{plan.thisYearQuantity} this year</span><span class="text-info"
				>{plan.assignedQuantity - plan.thisYearQuantity} later</span
			>
			{#if plan.unassignedQuantity}<span class="text-warning"
					>{plan.unassignedQuantity} could not be assigned to eligible yards within 100 years</span
				>{/if}
		</div>
		{#if !plan.yards.length}<p>No owned bases are available.</p>{/if}
		<div class="overflow-x-auto">
			<table class="table table-sm" aria-label="Shipyard allocation">
				<thead>
					<tr>
						<th>Base</th>
						{@render sortHeader('resources', 'Queue resources', 'Res')}
						{#each minerals as mineral (mineral.key)}
							{@render sortHeader(mineral.key, `${mineral.label} (kT)`, mineral.short)}
						{/each}
						<th>Free this year</th>
						<th>Green batch</th>
						<th>Blue batch</th>
						<th>Completion</th>
						{@render sortHeader('status', 'Status')}
					</tr>
				</thead>
				<tbody>
					{#each sortedYards as yard (yard.planet?.mapObject?.num)}
						{@const planet = yard.planet}
						{@const mapObject = planet?.mapObject}
						<tr>
							<td>
								{#if mapObject}
									<a class="cs-link" href={`/games/${$game.id}/planets/${mapObject.num}/production`}>
										{mapObject.name}
									</a>
								{/if}
							</td>
							<td class="px-1 text-right">{planet?.spec?.resourcesPerYearAvailable ?? '—'}</td>
							{#each minerals as mineral (mineral.key)}
								<td class={`px-1 text-right ${mineral.color}`}>{planet?.cargo?.[mineral.key] ?? '—'}</td>
							{/each}
							<td>{yard.capacityThisYear}</td>
							<td class="text-success font-bold" data-type="current-year-ships"
								>{yard.thisYearQuantity || '—'}</td
							>
							<td class="text-info font-bold" data-type="future-year-ships"
								>{yard.laterQuantity || '—'}</td
							>
							<td
								class:text-info={yard.yearsToComplete > 1}
								class:text-success={yard.yearsToComplete === 1}
								>{yard.thisYearQuantity + yard.laterQuantity
									? `${yard.yearsToComplete} ${yard.yearsToComplete === 1 ? 'year' : 'years'}`
									: '—'}</td
							>
							<td class="whitespace-nowrap">{yard.reason}</td>
						</tr>{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
