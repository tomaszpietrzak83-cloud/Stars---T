<script lang="ts">
	import { create, clone } from '@bufbuild/protobuf';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { techs } from '#lib/services/Stores.js';
	import {
		BattlePlanSchema,
		BattleTactic,
		BattleTarget,
		TechCategory,
		type ShipDesignSpec,
		type BattlePlan
	} from '#lib/types/cs-proto.js';
	import { BattleTacticDescriptions } from '#lib/types/Enums.js';
	import { dedicatedMinesweeper } from '#lib/types/automationRoles.js';
	import BattlePlanEditor from '../../../../routes/(user)/games/(game)/[id]/(plans)/battle-plans/BattlePlanEditor.svelte';

	let { selection = $bindable(0), spec }: { selection?: number; spec: ShipDesignSpec } = $props();
	const { player, updateBattlePlan, createBattlePlan } = getGameContext();
	let editing = $state(false);
	let draft: BattlePlan = $state(create(BattlePlanSchema));
	let saving = $state(false);
	let newPlan = $state(false);
	let error = $state('');
	let plans = $derived($player.playerPlans?.battlePlans ?? []);
	let automatic = $derived.by(() => {
		if (dedicatedMinesweeper(spec))
			return { name: 'Minesweeper retreat', tactic: BattleTactic.DISENGAGE };
		if (!spec.hasWeapons) return { name: 'Chicken', tactic: BattleTactic.DISENGAGE };
		let beams = 0,
			torpedoes = 0;
		for (const slot of spec.weaponSlots) {
			const component = $techs.getHullComponent(slot.hullComponent);
			if (component?.tech?.category === TechCategory.TORPEDO)
				torpedoes += component.power * slot.quantity;
			else if (component && !component.damageShieldsOnly) beams += component.power * slot.quantity;
		}
		if (torpedoes > 0 && torpedoes >= beams)
			return { name: 'Sniper', tactic: BattleTactic.MAXIMIZE_RANGE };
		if (beams > 0) return { name: 'Beam assault', tactic: BattleTactic.MAXIMIZE_DAMAGE };
		return { name: 'Shield support', tactic: BattleTactic.MAXIMIZE_DAMAGE_RATIO };
	});
	let selected = $derived(
		dedicatedMinesweeper(spec) ? undefined : plans.find((p) => p.num === selection - 1)
	);
	function edit() {
		const existing = selected ?? plans.find((p) => p.name === automatic.name);
		draft = existing
			? clone(BattlePlanSchema, existing)
			: create(BattlePlanSchema, {
					...(plans[0] ?? {}),
					name: automatic.name
				});
		newPlan = !existing;
		if (!selected) {
			draft.name = automatic.name;
			draft.tactic = automatic.tactic;
			draft.primaryTarget = spec.hasWeapons ? BattleTarget.ARMED_SHIPS : BattleTarget.UNSPECIFIED;
			draft.secondaryTarget = spec.hasWeapons ? BattleTarget.ANY : BattleTarget.UNSPECIFIED;
		}
		editing = true;
		error = '';
	}

	async function save() {
		saving = true;
		try {
			const saved = newPlan ? await createBattlePlan(draft) : await updateBattlePlan(draft);
			selection = saved.num + 1;
			editing = false;
		} catch (e) {
			error = String(e);
		} finally {
			saving = false;
		}
	}
</script>

<section class="border border-base-content/40 rounded p-3 my-2" aria-label="Design battle strategy">
	<label for="design-battle-strategy" class="font-semibold">Default battle strategy</label>
	{#if dedicatedMinesweeper(spec)}
		<select id="design-battle-strategy" class="select select-bordered w-full" disabled>
			<option>Automatic: Minesweeper retreat</option>
		</select>
	{:else}<select
			id="design-battle-strategy"
			class="select select-bordered w-full"
			bind:value={selection}
			disabled={dedicatedMinesweeper(spec)}
		>
			<option value={0}>Automatic: {automatic.name}</option>
			{#each plans as plan (plan.num)}<option value={plan.num + 1}>{plan.name}</option>{/each}
		</select>{/if}
	<p class="text-sm mt-1">{BattleTacticDescriptions[selected?.tactic ?? automatic.tactic]}</p>
	{#if selected}
		<p class="text-sm">
			Targets: {BattleTarget[selected.primaryTarget]} / {BattleTarget[selected.secondaryTarget] ||
				'None'}.
		</p>
	{/if}
	<button type="button" class="btn btn-sm mt-1" onclick={edit} disabled={dedicatedMinesweeper(spec)}
		>Preview / edit strategy</button
	>
	{#if dedicatedMinesweeper(spec)}<p class="text-sm">
			Dedicated minesweepers always disengage without firing; combat strategies do not override
			mine-clearing duty.
		</p>{/if}
	{#if !dedicatedMinesweeper(spec)}<p class="text-xs mt-1">
			Each ship stack uses its design strategy. An explicit fleet battle plan overrides it. Unarmed
			ships retreat.
		</p>{/if}
	{#if editing}
		<div class="border-t mt-3 pt-3" aria-label="Edit design battle strategy">
			<BattlePlanEditor bind:plan={draft} />
			<p class="text-sm">
				Saving updates this shared battle plan for every fleet and design using it.
			</p>
			{#if error}<p role="alert" class="text-error">{error}</p>{/if}
			<button type="button" class="btn btn-primary btn-sm" disabled={saving} onclick={save}
				>Save strategy and return</button
			>
			<button type="button" class="btn btn-sm" disabled={saving} onclick={() => (editing = false)}
				>Back to ship design</button
			>
		</div>
	{/if}
</section>
