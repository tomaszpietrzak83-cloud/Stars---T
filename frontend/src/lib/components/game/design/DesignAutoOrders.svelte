<script lang="ts">
	import HelpHint from '#lib/components/game/HelpHint.svelte';
	import { create } from '@bufbuild/protobuf';
	import { FleetSchema, ShipDesignSchema, type ShipDesignSpec } from '#lib/types/cs-proto.js';
	import { automationRoles, dedicatedMinesweeper } from '#lib/types/automationRoles.js';
	let {
		selection = $bindable(''),
		spec,
		disabled = false
	}: { selection?: string; spec: ShipDesignSpec; disabled?: boolean } = $props();
	let roles = $derived(
		automationRoles(
			create(FleetSchema, {
				spec: { shipDesignSpec: spec },
				tokens: [{ designNum: 1, quantity: 1 }]
			}),
			[create(ShipDesignSchema, { num: 1, spec })],
			selection
		)
	);
</script>

{#if !spec.starbase}
	<section class="rounded border border-base-content/40 p-3 my-2" aria-label="Design Auto orders">
		<div class="flex items-center gap-2">
			<label for="design-auto-orders" class="font-semibold">Auto orders after construction</label>
			<HelpHint label="Design Auto orders" text="New ships of this design start with the selected role. The list shows roles compatible with the design." />
		</div>
		<select
			id="design-auto-orders"
			class="select select-bordered w-full"
			bind:value={selection}
			{disabled}
		>
			{#each roles as role (role.value)}<option value={role.value}>{role.label}</option>{/each}
		</select>
		{#if dedicatedMinesweeper(spec)}<p class="text-sm mt-1">
				Gatling-only design: dedicated mine clearing, no patrol or escort assignments. Retreats if
				intercepted.
			</p>{/if}
		<p class="text-xs mt-1">
			New ships start with this automation enabled. Upgraded versions inherit this setting. Existing
			fleets keep their current orders and automation.
		</p>
	</section>
{/if}
