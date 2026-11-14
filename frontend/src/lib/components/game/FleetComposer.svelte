<script lang="ts">
	import HelpHint from '#lib/components/game/HelpHint.svelte';
	import { clone, create } from '@bufbuild/protobuf';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { FleetAssemblySchema, LogisticsAutomationSchema, type Fleet, type FleetAssembly } from '#lib/types/cs-proto.js';
	const { player, universe, updatePlayerOrders } = getGameContext();
	let automation = $derived($player.playerOrders.automation ?? create(LogisticsAutomationSchema));
	let plans = $derived(Object.entries(automation.fleetAssemblies));
	let designs = $derived($universe.getMyDesigns().filter(d => !d.spec?.starbase && !$player.playerOrders.designMaintenance[d.num]));
	let stations = $derived($universe.planets.filter(p => p.mapObject?.playerNum === $player.num && p.spec?.planetStarbaseSpec?.hasStarbase));
	let fleets = $derived($universe.fleets.filter(f => f.mapObject?.playerNum === $player.num && !f.starbase));
	let editing = $state<number | null>(null);
	let draft = $state(create(FleetAssemblySchema));
	let busy = $state(false), error = $state('');
	function start(id = 0, fleet?: Fleet) {
		editing = id;
		draft = id ? clone(FleetAssemblySchema, automation.fleetAssemblies[id]) : create(FleetAssemblySchema, {
			name: fleet?.mapObject?.name ?? 'New fleet', rallyPlanetNum: stations[0]?.mapObject?.num ?? 0, buildMissing: true,
			designs: Object.fromEntries((fleet?.tokens ?? []).map(t => [t.designNum, t.quantity])),
			sourceFleets: fleet ? { [fleet.mapObject!.num]: true } : {}
		});
		error = '';
	}
	function available(fleet: Fleet) {
		const tags = fleet.mapObject?.tags ?? {};
		return !tags['design-maintenance'] && (!tags['fleet-assembly'] || Number(tags['fleet-assembly']) === editing);
	}
	async function persist(change: (value: typeof automation) => void) {
		const before = $player.playerOrders.automation;
		const next = clone(LogisticsAutomationSchema, automation);
		change(next); $player.playerOrders.automation = next; busy = true; error = '';
		try { await updatePlayerOrders(); editing = null; }
		catch (cause) { $player.playerOrders.automation = before; error = cause instanceof Error ? cause.message : 'Could not save the fleet composition.'; }
		finally { busy = false; }
	}
	function save() {
		const id = editing || Math.max(0, ...Object.keys(automation.fleetAssemblies).map(Number)) + 1;
		const previous = automation.fleetAssemblies[id] ?? create(FleetAssemblySchema);
		const next: FleetAssembly = { ...clone(FleetAssemblySchema, previous), name: draft.name.trim(), rallyPlanetNum: draft.rallyPlanetNum,
			designs: Object.fromEntries(Object.entries(draft.designs).filter(([, n]) => n > 0)), sourceFleets: { ...draft.sourceFleets },
			buildMissing: draft.buildMissing, revision: previous.revision + 1 };
		return persist(value => { value.fleetAssemblies[id] = next; });
	}
</script>

<section class="bg-base-200 rounded-md p-3 mb-4" aria-label="Fleet composer">
	<h2 class="font-semibold text-lg flex items-center gap-2">Fleet composition and assembly <HelpHint label="Fleet assembly" text="Choose a rally station and target quantities. Selected source fleets can contribute ships, and shipyards can build the missing ones when enabled." /></h2>
	<p class="text-sm my-2">Choose a composition, existing ships and a rally station. Only the required ships are taken from selected fleets. Shipyards can build missing ships, and reinforcements follow the main fleet. Reports arrive at 70% and 100%. Completed orders do not replace subsequent losses.</p>
	<button class="btn btn-primary btn-sm" disabled={busy || !stations.length || plans.length >= 32} onclick={() => start()}>Create fleet composition</button>
	{#if !stations.length}<p class="text-sm my-2">An owned station is required as the rally point.</p>{/if}
	<div class="overflow-x-auto my-3"><table class="table table-sm"><thead><tr><th>Fleet</th><th>Composition</th><th>Status</th><th></th></tr></thead><tbody>
		{#each plans as [id, plan] (id)}<tr><td>{plan.name}</td><td>{Object.entries(plan.designs).map(([num, count]) => `${count} × ${$universe.getMyDesigns().find(d => d.num === Number(num))?.name ?? '#' + num}`).join(', ')}</td><td>{plan.status || 'Order pending next turn'}</td><td class="flex gap-2"><button class="btn btn-sm" disabled={busy} onclick={() => start(Number(id))}>Edit composition {plan.name}</button><button class="btn btn-sm" disabled={busy} onclick={() => persist(value => { delete value.fleetAssemblies[Number(id)]; })}>Cancel {plan.name}</button></td></tr>{/each}
	</tbody></table></div>
	{#if editing !== null}
		<form class="border border-base-300 rounded-md p-3 my-3" onsubmit={event => { event.preventDefault(); void save(); }}>
			<h3 class="font-semibold">{editing ? 'Edit composition' : 'New composition'}</h3>
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-3 my-3">
				<label>Fleet name<input class="input w-full" aria-label="Fleet composition name" maxlength="80" required bind:value={draft.name} /></label>
				<label>Rally station<select class="select w-full" aria-label="Rally station" bind:value={draft.rallyPlanetNum}>{#each stations as station (station.mapObject!.num)}<option value={station.mapObject!.num}>{station.mapObject!.name}</option>{/each}</select></label>
			</div>
			<fieldset class="my-3"><legend class="font-semibold">Desired ship quantities</legend><div class="grid grid-cols-1 sm:grid-cols-3 gap-3 my-2">{#each designs as design (design.num)}<label>{design.name}<input class="input w-full" aria-label={`Quantity ${design.name}`} type="number" min="0" max="10000" step="1" value={draft.designs[design.num] ?? 0} oninput={event => { draft.designs[design.num] = Number(event.currentTarget.value); }} /></label>{/each}</div></fieldset>
			<fieldset class="my-3"><legend class="font-semibold">Take required ships from existing fleets</legend><div class="max-h-64 overflow-y-auto">{#each fleets.filter(available) as fleet (fleet.mapObject!.num)}<label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={draft.sourceFleets[fleet.mapObject!.num] ?? false} onchange={event => { draft.sourceFleets[fleet.mapObject!.num] = event.currentTarget.checked; }} />{fleet.mapObject!.name} — {fleet.tokens.reduce((sum, token) => sum + token.quantity, 0)} ships</label>{/each}</div></fieldset>
			<label class="flex gap-2 my-3"><input class="checkbox" type="checkbox" bind:checked={draft.buildMissing} />Build missing ships at available shipyards</label>
			<p class="text-sm my-2">Reducing the composition releases surplus ships. Editing or cancelling removes unnecessary production without refunding resources already spent. Changes take effect when the next turn is generated.</p>
			<div class="flex gap-2"><button class="btn btn-primary btn-sm" type="submit" disabled={busy || !draft.name.trim() || !Object.values(draft.designs).some(n => n > 0)}>Save fleet composition</button><button class="btn btn-sm" type="button" disabled={busy} onclick={() => { editing = null; }}>Close editor</button></div>
		</form>
	{/if}
	<details class="my-3"><summary>Create from an existing fleet</summary><div class="flex flex-wrap gap-2 mt-2">{#each fleets.filter(f => !f.mapObject?.tags['fleet-assembly'] && !f.mapObject?.tags['design-maintenance']) as fleet (fleet.mapObject!.num)}<button class="btn btn-sm" disabled={busy || !stations.length} onclick={() => start(0, fleet)}>{fleet.mapObject!.name}</button>{/each}</div></details>
	{#if error}<p class="text-error my-2" role="alert">{error}</p>{/if}
</section>
