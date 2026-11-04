<script lang="ts">
	import { getGameContext } from '#lib/services/GameContext.js';
	const { settings } = getGameContext();
</script>

<button
	type="button"
	aria-label="Fleet routes"
	aria-pressed={$settings.showFleetRoutes !== false}
	title="Show or hide fleet routes"
	class="btn btn-xs size-full p-0"
	class:btn-primary={$settings.showFleetRoutes !== false}
	class:btn-ghost={$settings.showFleetRoutes === false}
	onclick={() => {
		$settings.showFleetRoutes = $settings.showFleetRoutes === false;
	}}
>
	<svg
		class="size-6"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		stroke-width="1.8"
		aria-hidden="true"
		><path d="M5 18 10 6l9 6M15 12h4V8" /><circle cx="5" cy="18" r="2" /><circle
			cx="10"
			cy="6"
			r="1.5"
		/></svg
	>
</button>
