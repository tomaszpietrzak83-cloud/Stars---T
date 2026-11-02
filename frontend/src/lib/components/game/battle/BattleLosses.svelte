<script lang="ts">
 let { own, enemy, ownMass, enemyMass }: { own: number; enemy: number; ownMass?:number; enemyMass?:number } = $props();
</script>
<span class="enemy-loss font-semibold">Enemy losses: {enemy} <span title="Destroyed battle mass, including cargo; unavailable for incomplete old reports">({enemyMass===undefined?'—':enemyMass.toLocaleString('en-US')} kT)</span></span>
<span aria-hidden="true"> · </span>
<span class="own-loss font-semibold">Our losses: {own} <span title="Destroyed battle mass, including cargo; unavailable for incomplete old reports">({ownMass===undefined?'—':ownMass.toLocaleString('en-US')} kT)</span></span>
<style>
 .enemy-loss { color: #15803d; }
 .own-loss { color: #b91c1c; }
 :global([data-theme="business"]) .enemy-loss { color: #86efac; }
 :global([data-theme="business"]) .own-loss { color: #fca5a5; }
</style>
