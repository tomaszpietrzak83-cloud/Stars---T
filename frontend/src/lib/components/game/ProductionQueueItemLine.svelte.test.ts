import '../../../css/app.css';
import { Universe } from '#lib/services/Universe.js';
import {
	ProductionQueueItemSchema,
	QueueItemCompletionEstimateSchema,
	QueueItemType
} from '#lib/types/cs-proto.js';
import { Infinite } from '#lib/types/Consts.js';
import { create } from '@bufbuild/protobuf';
import { writable } from 'svelte/store';
import { expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ProductionQueueItemLine from './ProductionQueueItemLine.svelte';

const state = vi.hoisted(() => ({ context: undefined as unknown }));
vi.mock('#lib/services/GameContext.js', () => ({ getGameContext: () => state.context }));

const classes = ['this-year', 'next-year', 'skipped', 'never'];

it.each([1, 2])('highlights selected ship completion in year %s', (year) => {
	state.context = { universe: writable(new Universe()) };
	const item = create(ProductionQueueItemSchema, {
		type: QueueItemType.SHIP_TOKEN,
		quantity: 5,
		queueItemCompletionEstimate: { yearsToBuildOne: 1, yearsToBuildAll: year }
	});
	const { container, unmount } = render(ProductionQueueItemLine, {
		index: 0,
		item,
		selected: true
	});
	expect(container.querySelector('button')!.style.backgroundColor).toBe(
		year === 1 ? 'var(--color-success)' : 'var(--color-info)'
	);
	unmount();
});

it('changes immediately from green to blue when the enlarged batch extends beyond this year', async () => {
	state.context = { universe: writable(new Universe()) };
	const item = create(ProductionQueueItemSchema, {
		type: QueueItemType.SHIP_TOKEN,
		quantity: 1,
		queueItemCompletionEstimate: { yearsToBuildOne: 1, yearsToBuildAll: 1 }
	});
	const view = render(ProductionQueueItemLine, { index: 0, item, selected: true });
	expect(view.container.querySelector('button')!.style.backgroundColor).toBe(
		'var(--color-success)'
	);
	await view.rerender({
		index: 0,
		item: create(ProductionQueueItemSchema, {
			...item,
			quantity: 2,
			queueItemCompletionEstimate: create(QueueItemCompletionEstimateSchema, {
				yearsToBuildOne: 1,
				yearsToBuildAll: 2
			})
		}),
		selected: true
	});
	expect(view.container.querySelector('button')!.style.backgroundColor).toBe('var(--color-info)');
	expect(
		view.container.querySelector('button')!.classList.contains('text-queue-item-next-year')
	).toBe(true);
	view.unmount();
});

// which of the queue item colors a line gets for these estimates
function colors(type: QueueItemType, yearsToBuildOne: number, yearsToBuildAll: number) {
	state.context = { universe: writable(new Universe()) };
	const item = create(ProductionQueueItemSchema, {
		type,
		quantity: 100,
		queueItemCompletionEstimate: { yearsToBuildOne, yearsToBuildAll, yearsToSkipAuto: Infinite }
	});
	const { container, unmount } = render(ProductionQueueItemLine, { index: 0, item });
	const button = container.querySelector('button')!;
	const found = classes.filter((c) => button.classList.contains(`text-queue-item-${c}`));
	unmount();
	return found;
}

// The entire ordered batch must finish this year to stay green.
it.each([QueueItemType.MINE, QueueItemType.AUTO_MINES])(
	'colors item type %s by how much of it is built this year',
	(type) => {
		expect(colors(type, 1, 1)).toEqual(['this-year']);
		expect(colors(type, 1, 3)).toEqual(['next-year']);
		expect(colors(type, 1, Infinite)).toEqual(['next-year']);
		expect(colors(type, 4, 6)).toEqual(['next-year']);
	}
);

it('colors items that can never be built', () => {
	expect(colors(QueueItemType.MINE, Infinite, Infinite)).toEqual(['never']);
	// an auto item with nothing to build is skipped
	expect(colors(QueueItemType.AUTO_MINES, Infinite, Infinite)).toEqual(['skipped']);
});

it('tells the queue about clicks and double clicks', () => {
	state.context = { universe: writable(new Universe()) };
	const item = create(ProductionQueueItemSchema, { type: QueueItemType.MINE, quantity: 5 });
	const onQueueItemClicked = vi.fn();
	const onQueueItemDoubleClicked = vi.fn();
	const { container, unmount } = render(ProductionQueueItemLine, {
		index: 2,
		item,
		selected: true,
		onQueueItemClicked,
		onQueueItemDoubleClicked
	});
	const button = container.querySelector('button')!;
	expect(button.style.backgroundColor).toBe('var(--color-info)');

	button.click();
	expect(onQueueItemClicked).toHaveBeenCalledWith(2, item);
	expect(onQueueItemDoubleClicked).not.toHaveBeenCalled();

	button.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
	expect(onQueueItemDoubleClicked).toHaveBeenCalledWith(2, item);
	unmount();
});
