<script lang="ts">
	import type { Planet } from '#lib/types/cs-proto.js';
	type Props = { planets: Planet[]; playerNum: number; researchHref: string };
	let { planets, playerNum, researchHref }: Props = $props();
	let totals = $derived.by(() => {
		let resources = 0,
			research = 0;
		for (const planet of planets) {
			if (playerNum <= 0 || planet.mapObject?.playerNum !== playerNum) continue;
			const spec = planet.spec;
			if (!spec) continue;
			resources += spec.resourcesPerYear;
			// Empty queues spend nothing. Derive their contribution even for older saved estimates.
			research +=
				planet.planetOrders?.productionQueue.length === 0
					? spec.resourcesPerYear
					: spec.resourcesPerYearResearch + spec.resourcesPerYearResearchEstimatedLeftover;
		}
		return { resources, research };
	});
</script>

<div
	class="flex flex-col text-right text-xs leading-tight tabular-nums mr-2"
	aria-label="Empire annual resources"
>
	<span title="Total annual resources produced by your planets"
		>Resources <strong>{totals.resources.toLocaleString('en-US')}</strong></span
	>
	<a
		class="text-blue-600 dark:text-blue-400 hover:underline"
		href={researchHref}
		title="Estimated annual research, including unused production resources"
		>Research <strong>{totals.research.toLocaleString('en-US')}</strong></a
	>
</div>
