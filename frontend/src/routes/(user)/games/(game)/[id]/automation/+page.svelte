<script lang="ts">
	import FleetAutomation from '#lib/components/game/FleetAutomation.svelte';
	import FleetComposer from '#lib/components/game/FleetComposer.svelte';
</script>

<div class="w-full max-w-6xl mx-auto overflow-y-auto p-3">
	<h1 class="text-2xl font-semibold mb-3">Automation</h1>
	<FleetComposer />
	<FleetAutomation />
</div>
