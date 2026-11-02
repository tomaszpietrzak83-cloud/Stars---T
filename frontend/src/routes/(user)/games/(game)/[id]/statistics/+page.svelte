<script lang="ts">
 import ItemTitle from '#lib/components/ItemTitle.svelte';
 import StatisticsDashboard from '#lib/components/game/StatisticsDashboard.svelte';
 import {getGameContext} from '#lib/services/GameContext.js';
 const {player} = getGameContext();
</script>
<div class="max-w-6xl mx-auto p-3"><ItemTitle>Statistics</ItemTitle><StatisticsDashboard stats={$player.stats}/></div>
