<script lang="ts">
	import { getGameContext } from '#lib/services/GameContext.js';
	import { PlanetViewState } from '#lib/types/PlayerSettings.js';
	import { GlobeAlt } from '@steeze-ui/heroicons';
	import { Icon } from '@steeze-ui/svelte-icon';

	const { settings } = getGameContext();
	let active = $derived($settings.planetViewState === PlanetViewState.Influence);
</script>

<button
	type="button"
	title="Territorial influence"
	aria-label="Territorial influence"
	aria-pressed={active}
	class="btn btn-xs size-full p-0"
	class:btn-primary={active}
	class:btn-ghost={!active}
	onclick={() => ($settings.planetViewState = PlanetViewState.Influence)}
>
	<Icon src={GlobeAlt} class="size-6" />
</button>
