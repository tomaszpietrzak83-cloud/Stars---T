<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { ownedBy } from '#lib/types/MapObject.js';
	import { CommandedPlanet } from '#lib/types/Planet.js';
	import ProductionQueue from '../../../dialogs/production/ProductionQueue.svelte';

	const { game, player, universe, readOnly, updatePlanetOrders } = getGameContext();
	const planetNum = $derived(Number(page.params.num));
	const source = $derived($universe.getPlanet(planetNum));
	const planet = $derived(
		source?.mapObject?.num === planetNum && ownedBy(source, $player.num)
			? new CommandedPlanet(source)
			: undefined
	);
	let saving = $state(false);
	let saveError = $state('');

	async function confirmQueue(planet: CommandedPlanet) {
		if (saving || planet.mapObject.num !== planetNum) return;
		saving = true;
		saveError = '';
		try {
			if (!$readOnly) await updatePlanetOrders(planet);
			await goto(`/games/${$game.id}/ship-production`);
		} catch (cause) {
			saveError = String(cause);
		} finally {
			saving = false;
		}
	}
</script>

{#if planet}
	<div class="grid min-w-0 gap-3 lg:h-full lg:grid-cols-[18rem_minmax(0,1fr)]">
		<aside
			aria-labelledby="production-planet-status"
			class="self-start rounded-xs border-2 border-base-300 bg-base-200 p-3 shadow-sm"
		>
			<h2 id="production-planet-status" class="mb-2 text-center text-lg font-semibold text-secondary">
				Planet status
			</h2>
			<dl class="grid grid-cols-[1fr_auto] gap-x-4 gap-y-1">
				<dt>Queue resources/year</dt>
				<dd class="text-right">{planet.spec.resourcesPerYearAvailable.toLocaleString()}</dd>
				<dt>Ironium on hand</dt>
				<dd class="whitespace-nowrap text-right text-ironium">{planet.cargo.ironium.toLocaleString()} kT</dd>
				<dt>Boranium on hand</dt>
				<dd class="whitespace-nowrap text-right text-boranium">{planet.cargo.boranium.toLocaleString()} kT</dd>
				<dt>Germanium on hand</dt>
				<dd class="whitespace-nowrap text-right text-germanium">{planet.cargo.germanium.toLocaleString()} kT</dd>
				<dt>Factories</dt>
				<dd class="whitespace-nowrap text-right">
					{planet.factories.toLocaleString()} /
					{planet.spec.maxFactories.toLocaleString()}
				</dd>
				<dt>Mines</dt>
				<dd class="whitespace-nowrap text-right">
					{planet.mines.toLocaleString()} /
					{planet.spec.maxMines.toLocaleString()}
				</dd>
				<dt>Defenses</dt>
				<dd class="text-right">{planet.defenses.toLocaleString()}</dd>
				<dt>Scanner</dt>
				<dd class="text-right">
					{planet.scanner || planet.spec.scanner ? 'Yes' : 'No'}
				</dd>
			</dl>
		</aside>
		<div class="min-h-0 min-w-0">
			{#if saveError}<p role="alert" class="text-error p-2">{saveError}</p>{/if}
			{#key planet.mapObject.num}
				<ProductionQueue {planet} onOk={confirmQueue} />
			{/key}
		</div>
	</div>
{:else}
	<p role="status" class="p-3">Production queue unavailable for this planet.</p>
{/if}
