<script lang="ts">
	import { clone, create } from '@bufbuild/protobuf';
	import { TransportPolicySchema, type LogisticsAutomation } from '#lib/types/cs-proto.js';
	let {
		automation,
		busy = false,
		onSave
	}: {
		automation: LogisticsAutomation;
		busy?: boolean;
		onSave: (change: (value: LogisticsAutomation) => void) => Promise<void>;
	} = $props();
	const fields = [
		{
			key: 'colonyHabitabilityCutoff',
			label: 'Lower-habitability cutoff (%)',
			fallback: 75,
			max: 100,
			step: 1
		},
		{
			key: 'colonyPopulationTarget',
			label: 'Lower-habitability population target and reserve (%)',
			fallback: 50,
			max: 99,
			step: 1
		},
		{
			key: 'capturedPopulationTarget',
			label: 'Captured colony population target (%)',
			fallback: 75,
			max: 99,
			step: 1
		},
		{
			key: 'defenseReserve',
			label: 'Defenses to retain at source (units)',
			fallback: 80,
			max: 1000,
			step: 1
		},
		{
			key: 'installationReservePercent',
			label: 'Factory and mine operating reserve (%)',
			fallback: 95,
			max: 100,
			step: 1
		},
		{
			key: 'attritionMinimumPeople',
			label: 'Minimum weakening landing (people)',
			fallback: 200000,
			max: 10000000,
			step: 100
		},
		{
			key: 'landingEscortShips',
			label: 'Patrol ships for landing assembly',
			fallback: 2,
			max: 100,
			step: 1
		}
	] as const;
	function defaults() {
		const value = create(TransportPolicySchema);
		for (const field of fields) value[field.key] = field.fallback;
		return value;
	}
	let policy = $state(defaults());
	$effect(() => {
		const value = clone(TransportPolicySchema, automation.transportPolicy ?? defaults());
		for (const field of fields) if (!value[field.key]) value[field.key] = field.fallback;
		policy = value;
	});
	let valid = $derived(
		fields.every(
			(f) =>
				Number.isInteger(policy[f.key]) &&
				policy[f.key] >= f.step &&
				policy[f.key] <= f.max &&
				policy[f.key] % f.step === 0
		)
	);
</script>

<fieldset class="border border-base-300 rounded p-3 my-4" disabled={busy}>
	<legend class="font-bold px-1">Transport development and landings</legend>
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
		{#each fields as field (field.key)}
			<label
				>{field.label}<input
					class="input input-bordered w-full"
					aria-label={field.label}
					type="number"
					min={field.step}
					max={field.max}
					step={field.step}
					bind:value={policy[field.key]}
				/></label
			>
		{/each}
	</div>
	<label class="flex gap-2 my-3"
		><input
			type="checkbox"
			class="checkbox"
			checked={!policy.landingsFirst}
			onchange={(e) => (policy.landingsFirst = !e.currentTarget.checked)}
		/>Supply colonies and shipyards before preparing new landings</label
	>
	<label class="flex gap-2 my-3"
		><input
			type="checkbox"
			class="checkbox"
			checked={!policy.disableCapturedSupport}
			onchange={(e) => (policy.disableCapturedSupport = !e.currentTarget.checked)}
		/>Reinforce captured colonies until their first starbase is built</label
	>
	<label class="flex gap-2 my-3"
		><input
			type="checkbox"
			class="checkbox"
			checked={!policy.disableAttrition}
			onchange={(e) => (policy.disableAttrition = !e.currentTarget.checked)}
		/>Allow large landings that weaken defenders without capturing the planet</label
	>
	<label class="flex gap-2 my-3"
		><input
			type="checkbox"
			class="checkbox"
			checked={!policy.disableRetarget}
			onchange={(e) => (policy.disableRetarget = !e.currentTarget.checked)}
		/>Redirect armies from unavailable base assaults to nearby weaker colonies</label
	>
	<p class="text-xs my-3">
		Population percentages refer to planet capacity, not transport holds. Below the habitability
		cutoff, the population reserve also prevents immediate re-export. Captured colonies use their
		separate target until a starbase exists. Manual passenger settings still control ordinary
		destinations.
	</p>
	<p class="text-xs my-3">
		Transports depart with full useful loads. Installations travel on existing supply routes.
		Landing groups still need reconnaissance and a clear or adequately protected orbit; a hostile
		base blocks the drop. Weakening landings must also cause meaningful expected losses.
	</p>
	{#if !valid}<p class="text-error text-sm" role="alert">
			Enter whole numbers within the displayed limits; landing population must be a multiple of 100.
		</p>{/if}
	<div class="flex flex-wrap gap-2 mt-3">
		<button
			class="btn btn-primary btn-sm"
			disabled={busy || !valid}
			onclick={() =>
				onSave((value) => {
					value.transportPolicy = clone(TransportPolicySchema, policy);
				})}>Save transport policy</button
		>
		<button class="btn btn-sm" onclick={() => (policy = defaults())}
			>Reset transport policy to defaults</button
		>
	</div>
</fieldset>
