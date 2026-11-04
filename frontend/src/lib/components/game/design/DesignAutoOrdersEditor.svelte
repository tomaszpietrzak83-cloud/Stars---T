<script lang="ts">
	import { untrack } from 'svelte';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { ShipDesignSpecSchema, type ShipDesign } from '#lib/types/cs-proto.js';
	import { create } from '@bufbuild/protobuf';
	import DesignAutoOrders from './DesignAutoOrders.svelte';
	const { readOnly, updateDesignAutoOrders } = getGameContext();
	let { design }: { design: ShipDesign } = $props();
	let selection = $state(
		untrack(() =>
			design.spec?.defaultAutoRole === 'manual' ? '' : (design.spec?.defaultAutoRole ?? '')
		)
	);
	let saving = $state(false);
	let error = $state('');
	let changed = $derived((selection || 'manual') !== (design.spec?.defaultAutoRole || 'manual'));
	async function save() {
		saving = true;
		error = '';
		try {
			await updateDesignAutoOrders(design.num, selection || 'manual');
		} catch (e) {
			error = String(e);
		} finally {
			saving = false;
		}
	}
</script>

<DesignAutoOrders
	bind:selection
	spec={design.spec ?? create(ShipDesignSpecSchema)}
	disabled={$readOnly || saving}
/>
{#if changed && !$readOnly}<button
		class="btn btn-primary btn-sm"
		type="button"
		disabled={saving}
		onclick={save}>Save Auto orders</button
	>{/if}
{#if error}<p role="alert" class="text-error">{error}</p>{/if}
