<script lang="ts">
	import Resources from '#lib/components/icons/Resources.svelte';
	import { getGameContext } from '#lib/services/GameContext.js';
	import { PlanetViewState } from '#lib/types/PlayerSettings.js';
	const { settings } = getGameContext();
</script>

<a
	href="#resources-view"
	aria-label="Planet resources"
	aria-current={$settings.planetViewState === PlanetViewState.Resources ? 'true' : undefined}
	title="Resources: blue circle = annual production (largest planet = 100%); base above the planet"
	class:btn-primary={$settings.planetViewState === PlanetViewState.Resources}
	class:btn-ghost={$settings.planetViewState !== PlanetViewState.Resources}
	class="btn btn-xs size-full p-0"
	onclick={(event) => {
		event.preventDefault();
		$settings.planetViewState = PlanetViewState.Resources;
	}}
>
	<Resources class="size-6 text-[#87cefa]" />
</a>
