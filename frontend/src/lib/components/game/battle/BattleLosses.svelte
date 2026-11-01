<script lang="ts">
 let { own, enemy }: { own: number; enemy: number } = $props();
</script>
<span class="enemy-loss font-semibold">Enemy losses: {enemy}</span>
<span aria-hidden="true"> · </span>
<span class="own-loss font-semibold">Our losses: {own}</span>
<style>
 .enemy-loss { color: #15803d; }
 .own-loss { color: #b91c1c; }
 :global([data-theme="business"]) .enemy-loss { color: #86efac; }
 :global([data-theme="business"]) .own-loss { color: #fca5a5; }
</style>
