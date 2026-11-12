<script lang="ts">
	import HelpHint from '#lib/components/game/HelpHint.svelte';
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';

	type Props = {
		title?: string;
		sectionKey?: string;
		help?: string;
		// small buttons right after the title
		actions?: Snippet;
		children?: Snippet;
	};

	let { title = '', sectionKey, help, actions, children }: Props = $props();
	let collapsed = $state(false);
	const storageKey = $derived('tomek-stars.command-section.' + (sectionKey ?? title));
	onMount(() => {
		try { collapsed = localStorage.getItem(storageKey) === 'collapsed'; } catch { /* Storage may be disabled. */ }
	});
	function toggle() {
		collapsed = !collapsed;
		try { localStorage.setItem(storageKey, collapsed ? 'collapsed' : 'expanded'); } catch { /* Keep the current UI usable. */ }
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="desk:w-56 phone-landscape:w-56 card bg-base-200 shadow-sm rounded-xs border-2 border-base-300 select-none"
	data-type="command-tile"
	data-id={title}
	oncontextmenu={(e) => e.preventDefault()}
>
	<div class="card-body p-3 gap-0">
		<!-- actions sit right beside the title, and the two are centered together -->
		<div class="flex flex-row items-center justify-center gap-1.5 mb-1">
			<button type="button" class="min-w-0 cursor-pointer" class:w-full={!actions && !help} onclick={toggle} aria-expanded={!collapsed} aria-label={`${collapsed ? 'Expand' : 'Collapse'} ${title || sectionKey || 'section'}`}>
				<div class="truncate text-center text-lg font-semibold text-secondary" {title}>
					<span aria-hidden="true">{collapsed ? '▸' : '▾'}</span> {title || 'Overview'}
				</div>
			</button>
			{#if help}<HelpHint label={title} text={help} />{/if}
			{#if actions}
				<div class="flex-none flex flex-row items-center">
					{@render actions()}
				</div>
			{/if}
		</div>
		<div hidden={collapsed}>
			{@render children?.()}
		</div>
	</div>
</div>
