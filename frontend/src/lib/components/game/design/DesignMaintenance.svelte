<script lang="ts">
	import { getGameContext } from '#lib/services/GameContext.js';
	import type { ShipDesign } from '#lib/types/cs-proto.js';

	let { design }: { design: ShipDesign } = $props();
	const { player, universe, updatePlayerOrders } = getGameContext();
	let selected = $state('0');
	let saving = $state(false);
	let error = $state('');
	let saved = $derived($player.playerOrders?.designMaintenance[design.num] ?? 0);
	let replacements = $derived($universe.getMyDesigns().filter((candidate) =>
		candidate.num !== design.num && candidate.hull === design.hull &&
		!candidate.originalPlayerNum && !candidate.spec?.starbase &&
		!$player.playerOrders?.designMaintenance[candidate.num]));
	let incoming = $derived(Object.values($player.playerOrders?.designMaintenance ?? {}).includes(design.num));
	let activeFleets = $derived($universe.fleets.flatMap((fleet) => {
		const raw = fleet.mapObject?.tags['design-maintenance'];
		if (!raw || !fleet.tokens.some((token) => token.designNum === design.num)) return [];
		try {
			const work = JSON.parse(raw) as { status?: string };
			return [{ num: fleet.mapObject!.num, name: fleet.mapObject!.name, status: work.status ?? 'Maintenance pending' }];
		} catch { return []; }
	}));
	$effect(() => { selected = String(saved); });

	async function save() {
		const previous = { ...$player.playerOrders!.designMaintenance };
		const next = { ...previous };
		if (Number(selected) === 0) delete next[design.num];
		else next[design.num] = Number(selected);
		saving = true;
		error = '';
		$player.playerOrders.designMaintenance = next;
		try { await updatePlayerOrders(); }
		catch (cause) {
			$player.playerOrders.designMaintenance = previous;
			error = cause instanceof Error ? cause.message : 'Could not save maintenance orders.';
		} finally { saving = false; }
	}
</script>

{#if design.playerNum === $player.num && !design.spec?.starbase && !design.originalPlayerNum}
	<section class="bg-base-200 rounded-md p-3 mt-3" aria-label="Design maintenance">
		<h2 class="font-semibold">Modernization and retirement</h2>
		<p class="text-sm my-2">Choose the replacement design, then order a fleet rebuild in Fleet automation. Obsolete sends all ships of this design for scrapping without replacements.</p>
		<label class="flex flex-wrap items-center gap-2">
			<span>Action for {design.name}</span>
			<select class="select select-bordered" bind:value={selected} disabled={saving || incoming}>
				<option value="0">Keep active</option>
				<option value="-1">Obsolete — send for scrapping</option>
				{#each replacements as candidate (candidate.num)}
					<option value={String(candidate.num)}>Modernize to {candidate.name}</option>
				{/each}
			</select>
			<button class="btn btn-primary" onclick={save} disabled={saving || incoming || selected === String(saved)}>{saving ? 'Saving…' : 'Apply'}</button>
		</label>
		{#if incoming}<p class="text-sm mt-2">This design is a modernization target. Clear its incoming modernization orders before retiring or replacing it.</p>{/if}
		<p class="text-sm mt-2">Rebuilding scraps the entire old ship and queues a full-price new ship. Normal scrapping recovery supplies minerals for production. The replacement is undamaged. Fleets split at an owned base, use available shipyards and regroup afterwards.</p>
		<p class="text-sm mt-2">Scrapping is irreversible once the turn is generated. Replacement orders use the normal production queue. Cancel unbuilt replacements there; canceling does not restore the scrapped ships. Fleets in the process cannot be manually split or merged.</p>
		{#each activeFleets as fleet (fleet.num)}<p class="text-sm mt-2">{fleet.name}: {fleet.status}</p>{/each}
		{#if error}<p role="alert" class="text-error mt-2">{error}</p>{/if}
	</section>
{/if}
