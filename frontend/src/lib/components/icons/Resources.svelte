<script lang="ts">
	import type { SVGAttributes } from 'svelte/elements';
	let props: SVGAttributes<SVGElement> = $props();
</script>

<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true" {...props}>
	<text
		x="12"
		y="20"
		text-anchor="middle"
		font-family="sans-serif"
		font-size="23"
		font-weight="700"
		fill="currentColor">R</text
	>
</svg>
