<script lang="ts">
	import HelpHint from '#lib/components/game/HelpHint.svelte';
	import {
		WaypointTaskTransportAction,
		type WaypointTransportTasks,
		WaypointTransportTaskSchema
	} from '#lib/types/cs-proto.js';
	import { create } from '@bufbuild/protobuf';
	import TransportTask from './TransportTask.svelte';

	type Props = {
		transportTasks: WaypointTransportTasks;
	};

	let { transportTasks = $bindable() }: Props = $props();

	const resources = [
		{ key: 'fuel', title: 'Fuel', textClass: 'text-fuel' },
		{ key: 'ironium', title: 'Ironium', textClass: 'text-ironium' },
		{ key: 'boranium', title: 'Boranium', textClass: 'text-boranium' },
		{ key: 'germanium', title: 'Germanium', textClass: 'text-germanium' },
		{ key: 'colonists', title: 'Colonists', textClass: 'text-colonists' }
	] as const;
</script>

<div class="grid grid-cols-4 mt-2">
	<!-- headers -->
	<div></div>
	<div class="col-span-2 flex items-center justify-center gap-2 font-semibold">Action <HelpHint label="Transport actions" text="Choose a separate action for each cargo type. Use Load Dunnage to fill leftover space after the other load orders." /></div>
	<div class="flex items-center justify-center gap-2 font-semibold">Amount <HelpHint label="Transport amount" text="Depending on the action, the number is a fixed quantity, a cargo target, or a percentage of fleet capacity." /></div>

	{#each resources as { key, title, textClass } (key)}
		<TransportTask
			{title}
			{textClass}
			bind:action={
				() => transportTasks[key]?.action ?? WaypointTaskTransportAction.UNSPECIFIED,
				(action) => {
					const task = (transportTasks[key] ??= create(WaypointTransportTaskSchema));
					task.action = action ?? WaypointTaskTransportAction.UNSPECIFIED;
				}
			}
			bind:amount={
				() => transportTasks[key]?.amount ?? 0,
				(amount) => {
					const task = (transportTasks[key] ??= create(WaypointTransportTaskSchema));
					task.amount = amount ?? 0;
				}
			}
		/>
	{/each}
</div>
