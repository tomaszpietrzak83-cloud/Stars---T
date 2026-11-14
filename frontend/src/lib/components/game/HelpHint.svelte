<script lang="ts">
	import { autoUpdate, computePosition, flip, offset, shift } from '@floating-ui/dom';

	let { label, text, example }: { label: string; text: string; example?: string } = $props();
	const id = $props.id();
	let trigger: HTMLButtonElement;
	let bubble: HTMLDivElement;
	let timer: ReturnType<typeof setTimeout> | undefined;
	let stopPositioning: (() => void) | undefined;

	function show() {
		if (bubble.matches(':popover-open')) return;
		bubble.showPopover();
		stopPositioning = autoUpdate(trigger, bubble, async () => {
			const { x, y } = await computePosition(trigger, bubble, {
				strategy: 'fixed',
				placement: 'bottom-start',
				middleware: [offset(6), flip({ padding: 8 }), shift({ padding: 8 })]
			});
			bubble.style.left = `${x}px`;
			bubble.style.top = `${y}px`;
		});
	}

	function schedule(event?: PointerEvent) {
		if (event?.pointerType === 'touch') return;
		clearTimeout(timer);
		timer = setTimeout(show, 1000);
	}

	function hide() {
		clearTimeout(timer);
		bubble?.hidePopover();
	}

	function click(event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();
		clearTimeout(timer);
		if (bubble.matches(':popover-open')) hide();
		else show();
	}

	$effect(() => () => {
		clearTimeout(timer);
		stopPositioning?.();
	});
</script>

<button
	bind:this={trigger}
	type="button"
	class="inline-grid size-4 shrink-0 place-items-center rounded-full border border-current align-middle text-[11px] font-semibold leading-none text-base-content/75 hover:text-base-content focus-visible:outline-2"
	aria-label={`Help: ${label}`}
	aria-describedby={id}
	onpointerenter={schedule}
	onpointerleave={hide}
	onfocus={() => schedule()}
	onblur={hide}
	onclick={click}
>?</button>
<div
	bind:this={bubble}
	{id}
	role="tooltip"
	popover="auto"
	class="pointer-events-none fixed inset-auto m-0 w-max max-w-[min(20rem,calc(100vw-1rem))] rounded-md border border-base-content/25 bg-base-100 px-3 py-2 text-left text-sm font-normal normal-case leading-snug text-base-content shadow-lg"
	ontoggle={() => {
		if (!bubble.matches(':popover-open')) {
			stopPositioning?.();
			stopPositioning = undefined;
		}
	}}
>
	<p>{text}</p>
	{#if example}<p class="mt-2 text-base-content/80"><strong>Example</strong> {example}</p>{/if}
</div>
