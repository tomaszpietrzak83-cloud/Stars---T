<script lang="ts">
	import { MaxNameLength } from '#lib/types/Consts.js';
	import CheckboxInput from '#lib/components/CheckboxInput.svelte';
	import EnumSelect from '#lib/components/EnumSelect.svelte';
	import TextInput from '#lib/components/TextInput.svelte';
	import { BattleTacticDescriptions, battleTargetToString } from '#lib/types/Enums.js';
	import {
		BattleAttackWho,
		type BattlePlan,
		BattleTactic,
		BattleTarget
	} from '#lib/types/cs-proto.js';

	type Props = {
		plan: BattlePlan;
	};

	let { plan = $bindable() }: Props = $props();

	// Local runes state for reactive bindings
	let name: string = $state(plan.name);
	let primaryTarget: BattleTarget = $state(plan.primaryTarget);
	let secondaryTarget: BattleTarget = $state(plan.secondaryTarget);
	let tactic: BattleTactic = $state(plan.tactic);
	let attackWho: BattleAttackWho = $state(plan.attackWho);
	let dumpCargo: boolean = $state(plan.dumpCargo);

	// Sync local state back to plan
	$effect(() => {
		plan.name = name;
		plan.primaryTarget = primaryTarget;
		plan.secondaryTarget = secondaryTarget;
		plan.tactic = tactic;
		plan.attackWho = attackWho;
		plan.dumpCargo = dumpCargo;
	});
</script>

<TextInput name="name" bind:value={name} required maxlength={MaxNameLength} />
<EnumSelect name="primaryTarget" enumType={BattleTarget} bind:value={primaryTarget} description="The type of target your ships look for first during combat." />
<EnumSelect
	name="secondaryTarget"
	description="The fallback target type when the primary target is unavailable."
	helpExample="Choose Bombers first and Armed Ships as fallback to engage armed escorts when no bomber is in reach."
	enumType={BattleTarget}
	showEmpty={true}
	typeTitle={battleTargetToString}
	bind:value={secondaryTarget}
/>
<EnumSelect name="tactic" enumType={BattleTactic} bind:value={tactic} description="Controls how a ship moves and engages during battle." />
{#if BattleTacticDescriptions[tactic]}
	<div class="w-full text-sm italic mb-2" data-type="tactic-description">
		{BattleTacticDescriptions[tactic]}
	</div>
{/if}
<EnumSelect name="attackWho" enumType={BattleAttackWho} bind:value={attackWho} description="Controls which player relations the fleet is willing to attack." />
<CheckboxInput name="dumpCargo" bind:checked={dumpCargo} description="Jettison mineral cargo when a battle starts to reduce combat mass. Colonists stay aboard." />
