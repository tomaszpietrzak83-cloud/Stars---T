<script lang="ts">
 import FuelRouteDetails from './FuelRouteDetails.svelte';
	import { clone, create } from '@bufbuild/protobuf';
	import { automationRoles } from '#lib/types/automationRoles.js';
 import ResearchTaxSettings from './ResearchTaxSettings.svelte';
 import { getGameContext } from '#lib/services/GameContext.js';
	import { LogisticsAutomationSchema, type LogisticsAutomation, type Fleet } from '#lib/types/cs-proto.js';
	const { player, universe, updatePlayerOrders } = getGameContext();
	let busy = $state(false);
	let error = $state('');
	let selected = $state<Record<number, boolean>>({});
 let roleSort = $state<'none'|'ascending'|'descending'>('none');
 function roleLabel(fleet: Fleet) { const role=automation.fleetRoles[fleet.mapObject!.num]??''; return automationRoles(fleet,$universe.getMyDesigns?.()??[],role).find(option=>option.value===role)?.label??role; }
	let automation = $derived($player.playerOrders.automation ?? create(LogisticsAutomationSchema));
	let repairThreshold=$state(30),scoutReturnTurns=$state(20);
 let sourcePercent=$state(90),destinationPercent=$state(60),escortMargin=$state(100),escortStrength=$state(100);
	$effect(()=>{
		scoutReturnTurns=automation.scoutMaxReturnTurns || 20;
  repairThreshold=automation.repairThresholdPercent || 30;
  sourcePercent=automation.manualPassengerThresholds?(automation.sourcePopulationPercent || (automation.passengerTransport?90:35)):33;
		destinationPercent=automation.manualPassengerThresholds?(automation.destinationPopulationPercent || (automation.passengerTransport?60:25)):20;
		escortMargin=automation.escortMarginLy || 100;
		escortStrength=automation.escortStrengthPercent || 100;
	});
	let fleets = $derived($universe.fleets.filter((fleet) => fleet.mapObject?.playerNum === $player.num && !fleet.starbase));
 let sortedFleets = $derived(roleSort==='none'?fleets:[...fleets].sort((a,b)=>{const order=roleLabel(a).localeCompare(roleLabel(b),'en')*(roleSort==='ascending'?1:-1); return order || (a.mapObject?.name??'').localeCompare(b.mapObject?.name??'','en',{numeric:true}) || a.mapObject!.num-b.mapObject!.num;}));
	let fuelBases = $derived(($universe.planets ?? []).filter(p => p.mapObject?.playerNum === $player.num && (p.spec?.planetStarbaseSpec?.dockCapacity ?? 0) !== 0));
	let fuelTankers = $derived(fleets.filter(f => automation.fleetRoles[f.mapObject!.num] === 'tanker'));
	async function save(change: (value: LogisticsAutomation) => void) {
		const before = $player.playerOrders.automation;
		const next = clone(LogisticsAutomationSchema, automation);
		change(next);
		$player.playerOrders.automation = next;
		busy = true; error = '';
		try { await updatePlayerOrders(); }
		catch (cause) { $player.playerOrders.automation = before; error = cause instanceof Error ? cause.message : 'Could not save automation.'; }
		finally { busy = false; }
	}
	function canRebuild(fleet: Fleet) { return fleet.tokens.some((token) => ($player.playerOrders.designMaintenance[token.designNum] ?? 0) > 0); }
	function canColonize(fleet: Fleet) { return !!fleet.spec?.shipDesignSpec?.colonizer && fleet.tokens.length > 0 && fleet.tokens.every(token => $universe.getMyDesigns().find(design => design.num === token.designNum)?.spec?.colonizer); }
	function reserved(fleet: Fleet) { const id = Number(fleet.mapObject?.tags['fleet-assembly']); const plan = automation.fleetAssemblies[id]; return !!plan && plan.completedRevision !== plan.revision; }
	let selectedRebuilds = $derived(fleets.filter(f => selected[f.mapObject!.num] && canRebuild(f) && !f.mapObject?.tags['design-maintenance'] && !reserved(f)));
	function status(fleet: Fleet): string {
		const tags = fleet.mapObject?.tags ?? {};
		if (tags['automatic-fuel-status']) return tags['automatic-fuel-status'];
		try { if (tags['automatic-tanker']) return (JSON.parse(tags['automatic-tanker']) as { status: string }).status; } catch { /* old status */ }
		const plan = automation.fleetAssemblies[Number(tags['fleet-assembly'])];
		if (plan) return `${plan.name}: ${plan.status || 'Order pending'}`;
		if (tags['scout-job'] && automation.fleetRoles[fleet.mapObject!.num] === 'scout') return tags['scout-job'];
		for (const key of ['automatic-patrol', 'design-maintenance', 'logistics-job', 'mining-job', 'colonization-job', 'terraform-job', 'automatic-safety', 'bomber-orders']) {
			try { if (tags[key]) return (JSON.parse(tags[key]) as { status?: string }).status ?? 'Order pending'; } catch { /* unreadable old status */ }
		}
		return tags['escort-for'] ? `Escorting fleet #${tags['escort-for']}` : '';
	}
</script>

<section class="bg-base-200 rounded-md p-3 mb-4" aria-label="Fleet automation">
	<h2 class="font-semibold text-lg">Fleet automation</h2>
 <p class="text-sm my-2">Bomber wolf packs and patrol groups form actual fleets for coordinated attacks. Split all dissolves a temporary pack and restores its ships’ previous roles. Bomber scanner safety budgets 98% stealth and a full turn of warning; escorts and free patrols can provide protection. Scouts prioritize unknown enemy fleets heading towards your colonies and accept combat risk while obtaining reconnaissance.</p>
	<p class="text-sm my-2">Fuel planning covers every automatic role, fleet rebuilding and assembly. It chooses the fastest safe route through known owned docks, accounting for engines, mines and a return reserve after loading. Tankers arrange rendezvous, refuel friendly fleets and replenish their stock. Assistance on a fast route is requested only when it improves arrival time.</p>
	<details class="border border-base-300 rounded-md p-3 my-3" open>
		<summary class="font-semibold">Fuel network: {fuelBases.length} docks and {fuelTankers.length} tankers</summary>
		<label class="flex gap-2 my-2"><input type="checkbox" class="checkbox" disabled={busy} checked={automation.serveTerraformingWorlds} onchange={e=>save(a=>{a.serveTerraformingWorlds=e.currentTarget.checked})} />Serve worlds requiring terraforming</label>
 <p class="text-xs">Colonizers and transports may select currently hostile worlds that your present terraforming technology can make habitable. Claim Adjuster always accounts for instant terraforming. Other races must pay for planetary terraforming.</p>
 <label class="flex gap-2 my-2"><input type="checkbox" class="checkbox" disabled={busy} checked={automation.infrastructureTransport} onchange={e=>save(a=>{a.infrastructureTransport=e.currentTarget.checked})} />Carry surplus installations on existing routes</label>
 <p class="text-xs">Factories, mines and defenses are picked up opportunistically, retaining 95% of current operating capacity. No dedicated collection trips are created.</p>
 <p class="text-sm my-2">Owned stations with docks provide fuel; ordinary colonies do not. Idle tankers fill gaps between docks and travel lanes, patrolling at up to warp 4. Fleets can travel to a deployed tanker for an agreed rendezvous. Tankers retain a return reserve; onboard generators replenish actual fuel each turn. Multi-stop itineraries reserve finite tanker supplies before departure, include every convoy member and stop for replanning if a required tanker or dock becomes unavailable.</p>
		<div class="overflow-auto max-h-64"><table class="table table-sm"><thead><tr><th>Node</th><th>Position</th><th>Fuel</th><th>Status</th></tr></thead><tbody>
			{#each fuelBases as base (base.mapObject!.num)}<tr><td>{base.mapObject!.name}</td><td>{base.mapObject!.position?.x}, {base.mapObject!.position?.y}</td><td>Refills the tank</td><td>Fixed node - dock</td></tr>{/each}
			{#each fuelTankers as tanker (tanker.mapObject!.num)}<tr><td>{tanker.mapObject!.name}</td><td>{tanker.mapObject!.position?.x}, {tanker.mapObject!.position?.y}</td><td>{tanker.fuel} / {tanker.spec?.shipDesignSpec?.fuelCapacity ?? 0}</td><td>{status(tanker)}</td></tr>{/each}
		</tbody></table></div>
 <details class="mt-2" aria-label="Reserved fuel routes"><summary>Reserved fuel routes</summary>{#each fleets.filter(f=>f.mapObject?.tags['fuel-network-plan']) as fleet (fleet.mapObject?.num)}<p class="font-semibold text-sm mt-2">{fleet.mapObject?.name}</p><FuelRouteDetails {fleet} />{/each}</details>
	</details>
	<p class="text-sm my-2">Scouts plan fuel-checked petal expeditions: outward scans in separate directions, then scans along a neighbouring return arm to their fuel dock. Multi-year legs use the engine’s cruise setting rather than crawling at warp 1–3; the entire loop is checked against actual fuel. Obsolete and successor orders wait until outward exploration ends. Automatic miners choose explored worlds using production mineral shortages, concentration and travel time; assignments last at least 10 working turns and change only for a substantial improvement. Role assignment takes control of the route. Automatic transports can collect mineral stock from unowned planets where your automatic miners are working.</p>
	<p class="text-sm my-2">Colony ships offer Nearest, Farthest, Best world and Auto modes. Auto balances known habitability against engine-aware travel time. All colonization modes require a fuel-checked cruise-speed route taking at most 50 turns; unreachable reservations are replaced by another eligible target. Targets are reserved to prevent duplicates. Stacks split into individual colony ships; mixed fleets must be separated first because colonization consumes the entire fleet. Colony ships fill their holds with settlers from owned colonies, leaving at least half their population and 5,000 people behind.</p>
	<fieldset class="border border-base-300 rounded-md p-3 my-3">
		<legend class="font-semibold">Automation settings</legend>
  <label class="block my-2">Scout maximum planned return time (turns)<input class="input input-bordered w-full" aria-label="Scout maximum return turns" type="number" min="1" max="200" bind:value={scoutReturnTurns} /></label>
  <label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={!automation.disableAutomaticRepair} disabled={busy} onchange={event=>save(value=>{value.disableAutomaticRepair=!event.currentTarget.checked;})} />Withdraw the whole automatic fleet for repairs</label>
  <label class="block my-2">Repair withdrawal at ship armour damage (%)<input class="input input-bordered w-full" aria-label="Repair damage threshold (%)" type="number" min="1" max="100" bind:value={repairThreshold} /></label>
  <label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={!automation.disableInterceptionEvasion} disabled={busy} onchange={event=>save(value=>{value.disableInterceptionEvasion=!event.currentTarget.checked;})} />Evade detected interception courses</label>
  <label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={automation.automaticPackets} disabled={busy} onchange={event=>save(value=>{value.automaticPackets=event.currentTarget.checked;})} />Supply shipyards by mineral packets between owned mass drivers</label>
  <p class="text-xs my-2">Repairs use the most damaged ship stack, withdraw the whole fleet and resume its mission after full repair. Nearby automatic tankers can provide a mobile workshop when it is faster than a dock; the best repair bonus applies to every damaged ship and does not stack. Evasion uses fresh visible courses, so hidden fleets cannot be predicted. Packet speed never exceeds the receiving mass driver's safe speed. Manually queued packets retain control.</p>
		<label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={!automation.disableMineralSupply} disabled={busy} onchange={(event)=>save((value)=>{value.disableMineralSupply=!event.currentTarget.checked;})} />Supply shipyard queues from surplus minerals</label>
		<label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={!automation.disableSalvage} disabled={busy} onchange={(event)=>save((value)=>{value.disableSalvage=!event.currentTarget.checked;})} />Collect known debris with automatic transports</label>
		<label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={!automation.disablePassengerTransport} disabled={busy} onchange={(event)=>save((value)=>{value.disablePassengerTransport=!event.currentTarget.checked;value.passengerTransport=event.currentTarget.checked;value.sourcePopulationPercent=sourcePercent;value.destinationPopulationPercent=destinationPercent;})} />Move passengers between source planets and growing colonies</label>
  <p class="text-xs my-2">Colonist deliveries take priority. Good colonies can receive a mixed load of 80% passengers and up to 20% ironium/boranium for a future base, after forecasting their own mining. Germanium is excluded from advance stockpiling. Otherwise free transports can carry full mineral loads to an active ship or starbase queue.</p>
		<label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={automation.manualPassengerThresholds} disabled={busy} onchange={event=>save(value=>{value.manualPassengerThresholds=event.currentTarget.checked;value.sourcePopulationPercent=sourcePercent;value.destinationPopulationPercent=destinationPercent;})} />Use manual population thresholds</label>
  <p class="text-xs my-2">By default, sources retain one third of capacity, which maximizes absolute population growth. Transports wait empty at the source until a full, fuel-safe load is available. Growing destinations are supplied to 20%. Manual thresholds override this rule.</p>
  <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 my-3">
			<label>Source population reserve (%)<input aria-label="Source population reserve (%)" class="input input-bordered w-full" type="number" min="1" max="100" disabled={!automation.manualPassengerThresholds} bind:value={sourcePercent} /></label>
			<label>Destination population target (%)<input aria-label="Destination population target (%)" class="input input-bordered w-full" type="number" min="1" max="99" disabled={!automation.manualPassengerThresholds} bind:value={destinationPercent} /></label>
			<label>Threat corridor (ly)<input aria-label="Threat corridor (ly)" class="input input-bordered w-full" type="number" min="1" max="1000" bind:value={escortMargin} /></label>
			<label>Required escort strength (%)<input aria-label="Required escort strength (%)" class="input input-bordered w-full" type="number" min="50" max="1000" bind:value={escortStrength} /></label>
		</div>
		<button class="btn btn-primary btn-sm" disabled={busy} onclick={()=>save((value)=>{value.scoutMaxReturnTurns=scoutReturnTurns;value.repairThresholdPercent=repairThreshold;value.sourcePopulationPercent=sourcePercent;value.destinationPopulationPercent=destinationPercent;value.escortMarginLy=escortMargin;value.escortStrengthPercent=escortStrength;})}>Save thresholds</button>
		<p class="text-sm mt-2">The source keeps its reserve. Incoming passenger convoys count toward the destination target. Colonies must be owned and suitable for the race. Existing passenger convoys finish their deliveries if new passenger jobs are disabled.</p>
		<label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={automation.singleShipyardRebuild} disabled={busy} onchange={(event)=>save((value)=>{value.singleShipyardRebuild=event.currentTarget.checked;})} />Keep fleet rebuilds at one shipyard instead of spreading production</label>
	</fieldset>
	<ResearchTaxSettings {automation} {busy} onSave={save} />
 <p class="text-sm my-2">Assign idle transports to collect known debris, supply shipyard queues from surplus stock and deliver minerals to owned shipyards. Routes near known armed fleets wait for enough assigned escorts. No known threat does not guarantee safety.</p>
	<label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={automation.battleSalvage} disabled={busy} onchange={(event) => save((value) => { value.battleSalvage = event.currentTarget.checked; })} />Collect debris after victorious battles, up to available cargo capacity</label>
	<label class="flex gap-2 my-2"><input class="checkbox" type="checkbox" checked={automation.offloadCombatCargo} disabled={busy} onchange={(event) => save((value) => { value.offloadCombatCargo = event.currentTarget.checked; })} />Send free automatic transports to collect minerals from combat fleets</label>
	<p class="text-sm my-2">Escort fleets travel with the transport at a shared speed. Automatic transport takes control of the route. Any owned planet above the population reserve may supply passengers. The loaded delivery and return fuel are checked before pickup. High-growth colonies receive priority; up to one third of transports can reclaim weak, valuable enemy colonies. Patrols can deliver spare fuel to nearby stranded automatic fleets while retaining their own return reserve.</p>
	<div class="overflow-x-auto">
		<div class="flex flex-wrap gap-2 my-3"><button class="btn btn-sm" disabled={busy} onclick={() => { selected = Object.fromEntries(fleets.map(f => [f.mapObject!.num, true])); }}>Select all fleets</button><button class="btn btn-sm" disabled={busy} onclick={() => { selected=Object.fromEntries(fleets.filter(f=>canRebuild(f) && !f.mapObject?.tags['design-maintenance'] && !reserved(f)).map(f=>[f.mapObject!.num,true])); }}>Check all for scrap and rebuild</button><button class="btn btn-sm" onclick={() => { selected = {}; }}>Clear selection</button><button class="btn btn-primary btn-sm" disabled={busy || !selectedRebuilds.length} onclick={() => save(value => { for (const fleet of selectedRebuilds) value.rebuildFleets[fleet.mapObject!.num] = true; })}>Rebuild selected ({selectedRebuilds.length})</button></div>
		<table class="table table-sm">
			<thead><tr><th>Select</th><th>Fleet</th><th aria-sort={roleSort}><button class="btn btn-ghost btn-xs" onclick={()=>{roleSort=roleSort==='ascending'?'descending':'ascending';}}>Automatic role {roleSort==='ascending'?'↑':roleSort==='descending'?'↓':'↕'}</button></th><th>Rebuilding</th><th>Status</th></tr></thead>
			<tbody>{#each sortedFleets as fleet (fleet.mapObject?.num)}
				{@const num = fleet.mapObject!.num}
				{@const maintenance = !!fleet.mapObject?.tags['design-maintenance']}
				<tr><td><input class="checkbox" type="checkbox" aria-label={`Select ${fleet.mapObject?.name}`} bind:checked={selected[num]} /></td><td>{fleet.mapObject?.name}</td><td>
					<select class="select select-sm" aria-label={`Role for ${fleet.mapObject?.name}`} value={automation.fleetRoles[num] ?? ''} disabled={busy || maintenance || reserved(fleet)} onchange={(event) => { const role = event.currentTarget.value; save((value) => { if (role) value.fleetRoles[num] = role; else delete value.fleetRoles[num]; }); }}>
{#each automationRoles(fleet,$universe.getMyDesigns?.()??[],automation.fleetRoles[num]??'') as option (option.value)}<option value={option.value}>{option.label}</option>{/each}
					</select>
				</td><td><button class="btn btn-sm" disabled={busy || maintenance || reserved(fleet) || !canRebuild(fleet) || automation.rebuildFleets[num]} onclick={() => save((value) => { value.rebuildFleets[num] = true; })}>{automation.rebuildFleets[num] ? 'Rebuild requested' : 'Scrap and rebuild'}</button></td><td>{status(fleet)}</td></tr>
			{/each}</tbody>
		</table>
	</div>
	{#if error}<p class="text-error mt-2" role="alert">{error}</p>{/if}
</section>
