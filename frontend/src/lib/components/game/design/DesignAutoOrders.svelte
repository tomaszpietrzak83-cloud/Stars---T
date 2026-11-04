<script lang="ts">
	import { create } from '@bufbuild/protobuf';
	import { FleetSchema, ShipDesignSchema, type ShipDesignSpec } from '#lib/types/cs-proto.js';
	import { automationRoles } from '#lib/types/automationRoles.js';
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
		<label for="design-auto-orders" class="font-semibold">Auto orders after construction</label>
		<select
			id="design-auto-orders"
			class="select select-bordered w-full"
			bind:value={selection}
			{disabled}
		>
			{#each roles as role (role.value)}<option value={role.value}>{role.label}</option>{/each}
		</select>
		<p class="text-xs mt-1">
			New ships start with this automation enabled. Upgraded versions inherit this setting. Existing
			fleets keep their current orders and automation.
		</p>
	</section>
{/if}
