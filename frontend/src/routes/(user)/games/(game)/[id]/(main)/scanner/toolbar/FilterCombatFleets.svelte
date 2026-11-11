<script lang="ts">
	import { getGameContext } from '#lib/services/GameContext.js';
	import { Bolt, BoltSlash } from '@steeze-ui/heroicons';
	import { Icon } from '@steeze-ui/svelte-icon';

	let { hide = false }: { hide?: boolean } = $props();
	const { settings } = getGameContext();
	const title = $derived(hide ? 'Hide Combat Fleets' : 'Combat Fleets Only');
	const mode = $derived(hide ? 'hideCombatFleets' : 'showCombatFleetsOnly');
	const otherMode = $derived(hide ? 'showCombatFleetsOnly' : 'hideCombatFleets');
	const active = $derived($settings[mode]);

	function toggle() {
		$settings[otherMode] = false;
		$settings[mode] = !active;
	}
</script>

<button
	type="button"
	{title}
	aria-label={title}
	aria-pressed={active}
	class:btn-primary={active}
	class:btn-ghost={!active}
	class="btn btn-xs size-full p-0"
	onclick={toggle}
>
	<Icon src={hide ? BoltSlash : Bolt} class="size-6" />
</button>
