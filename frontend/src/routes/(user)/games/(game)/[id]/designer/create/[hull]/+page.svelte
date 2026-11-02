<script lang="ts">
	import { designFamilyMembers, upgradeDesignName } from '#lib/types/designFamilies.js';
 import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Breadcrumb from '#lib/components/game/Breadcrumb.svelte';
	import ShipDesigner from '#lib/components/game/design/ShipDesigner.svelte';
	import { GameDBObjectSchema } from '#lib/types/cs-proto.js';
	import { ShipDesignSchema, type ShipDesign } from '#lib/types/cs-proto.js';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { techs } from '#lib/services/Stores.js';
	import { clone, create } from '@bufbuild/protobuf';
	import { onMount } from 'svelte';

	const { game, universe, player, createDesign, readOnly } = getGameContext();
	let hullName = page.params.hull;

	let hull = $derived($techs.getHull(hullName || ''));

	let design: ShipDesign | undefined = $state();

	let error = $state('');
 let upgradeFrom=$state(0);

	onMount(() => {
		design = create(ShipDesignSchema, {
			gameDbObject: {
				gameId: $game.id
			},
			playerNum: $player.num,
			hull: hullName
		});

		upgradeFrom=Number(page.url.searchParams.get('upgrade')??0);
 const copyParam = page.url.searchParams.get('copy')??(upgradeFrom?String(upgradeFrom):null);
		if (copyParam) {
			const copyDesign = $universe.getMyDesign(parseInt(copyParam));
			if (copyDesign) {
				design = clone(ShipDesignSchema, copyDesign);
				design.gameDbObject = create(GameDBObjectSchema);
    design.num=0;
				design.version++;
    if(upgradeFrom){const members=designFamilyMembers($universe.getMyDesigns(),$player.playerOrders.automation?.designFamilies??{},upgradeFrom);design.version=Math.max(1,...members.map(d=>d.version))+1;design.name=upgradeDesignName(copyDesign,design.version);}
			}
		}
	});

	async function save() {
		if (!design) {
			return;
		}
		error = '';
		try {
			const { valid, reason } = $universe.validateDesign(design);
			if (valid) {
				const created = await createDesign(design,upgradeFrom);
				goto(`/games/${$game.id}/designer/${created.num}`);
			} else {
				error = reason ?? 'invalid design';
			}
		} catch (e) {
			error = `${e}`;
		}
	}
</script>

<Breadcrumb>
	{#snippet crumbs()}
		<li><a class="cs-link" href={`/games/${$game.id}/designer`}>Ship Designs</a></li>
		<li><a class="cs-link" href={`/games/${$game.id}/designer/create`}>Choose Hull</a></li>
		<li>{design?.name == '' ? 'new' : design?.name}</li>
	{/snippet}
	{#snippet end()}
		<div class="flex justify-end mb-1">
			{#if !$readOnly}
				<button class="btn btn-success mx-1" type="submit" onclick={save}>Save</button>
			{/if}
		</div>
	{/snippet}
</Breadcrumb>
{#if hull && design}
	{#if upgradeFrom}<div class="alert alert-info my-3 block">Creating a new version of this design family on the same hull. All previous family versions will point to this successor. {#if design.spec?.starbase}Rebuild family queues the latest station at each eligible planet. The existing base remains operational until construction finishes.{:else}Existing ships are replaced by scrapping and rebuilding when you order a fleet or family rebuild; scouts finish their current expedition first.{/if}</div>{/if}
 <ShipDesigner bind:design {hull} onSave={save} {error} />
{/if}
