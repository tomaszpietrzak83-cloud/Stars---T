<script lang="ts">
	import HelpHint from '#lib/components/game/HelpHint.svelte';
	import { startCase } from 'lodash-es';

	type Props = {
		name: string;
		checked: boolean | undefined;
		title?: string | undefined;
		titleClass?: string;
		description?: string;
	};

	let {
		name,
		checked = $bindable(),
		titleClass = 'cs-label-text w-32 text-right',
		description,
		...rest
	}: Props = $props();

	let title = $derived(rest.title ?? startCase(name));
</script>

<div class="w-full grow">
	<div class="cs-form-control flex flex-row items-center gap-2">
		<label class="cs-form-label"
			><span class={titleClass}>{title}</span>
			<input class="checkbox ml-2" type="checkbox" {name} bind:checked />
		</label>
		{#if description}<HelpHint label={title} text={description} />{/if}
	</div>
</div>
