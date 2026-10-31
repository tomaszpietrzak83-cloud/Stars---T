<script lang="ts">
	import { getGameContext } from '#lib/services/GameContext.js';
	import Designs from './Designs.svelte';

	const { game, universe } = getGameContext();
</script>

<a class="btn btn-sm mb-2" href={`/games/${$game.id}/automation`}>Fleet automation settings</a>
<Designs designs={$universe.allDesigns} />
