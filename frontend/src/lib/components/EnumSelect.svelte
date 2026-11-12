<script lang="ts">
	import HelpHint from '#lib/components/game/HelpHint.svelte';
	import { enumToString } from '#lib/types/Enums.js';
	import { startCase } from 'lodash-es';
	import type { HTMLSelectAttributes } from 'svelte/elements';
	import { $enum as eu } from 'ts-enum-util';
	import type { StringKeyOf } from 'ts-enum-util/dist/types/types';

	// enums are strings or numbers
	type T = $$Generic<Record<StringKeyOf<E>, number | string>>;
	type TT = T[Extract<keyof T, string>];

	type Props<T> = {
		name: string;
		title?: string | undefined;
		tooltip?: string | undefined;
		description?: string;
		helpExample?: string;
		enumType: T;
		titleClass?: string;
		typeTitle?: (type: TT) => string;
		typeFilter?: (type: TT) => boolean;
		showEmpty?: boolean;
	} & HTMLSelectAttributes;

	let {
		name,
		value = $bindable(),
		tooltip,
		description,
		helpExample,
		enumType,
		titleClass = 'cs-label-text w-32 text-right',
		typeFilter = (_: TT) => true,
		showEmpty = false,
		...rest
	}: Props<T> = $props();

	const defaultTypeTitleFunc = (type: TT) => startCase(enumToString(enumType, type));

	let title = $derived(rest.title ?? startCase(name));
	let typeTitle = $derived(rest.typeTitle ?? defaultTypeTitleFunc);
</script>

<div class="flex w-full grow items-center gap-2">
	<label class="cs-form-label min-w-0 grow"
		><span class={titleClass}>{title}</span>
		<select class="select ml-2 grow" {name} bind:value {...rest}>
			{#each eu(enumType).getValues() as type (type)}
				{#if typeFilter(type) && (showEmpty || type !== 0)}
					<option value={type}>{typeTitle(type)}</option>
				{/if}
			{/each}
		</select>
	</label>
	{#if tooltip || description}
		<HelpHint label={title} text={tooltip ?? description ?? ''} example={helpExample} />
	{/if}
</div>
