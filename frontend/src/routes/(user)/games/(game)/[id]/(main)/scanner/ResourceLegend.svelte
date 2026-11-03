<script lang="ts">
	import { getGameContext } from '#lib/services/GameContext.js';
	const { settings } = getGameContext();
</script>

<div
	aria-label="Resource view legend"
	class="absolute bottom-2 left-2 right-2 sm:right-auto z-20 pointer-events-none rounded border border-white/25 bg-black/85 px-2 py-1 text-[10px] leading-relaxed text-white"
>
	<div>
		<span class="text-[#87cefa] font-bold">● R</span> annual production · largest planet = 100%
	</div>
	<div>
		Station ↑ · surface stocks → <span style="color:var(--scanner-resource-ironium)">I</span>
		<span style="color:var(--scanner-resource-boranium)">B</span>
		<span style="color:var(--scanner-resource-germanium)">G</span>
		· full bar: {Math.max(1, $settings.mineralScale).toLocaleString('en-US')} kT
	</div>
	<div class="text-white/70">I: Ironium · B: Boranium · G: Germanium · + above scale</div>
</div>
