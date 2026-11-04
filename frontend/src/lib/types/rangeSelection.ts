// Use displayed order and stable identities, including after a sort change.
export function rangeSelection(
	selected: Record<number, boolean>,
	order: number[],
	anchor: number | undefined,
	target: number,
	checked: boolean,
	shift: boolean
) {
	const next = { ...selected };
	const start = anchor === undefined ? -1 : order.indexOf(anchor);
	const end = order.indexOf(target);
	const ids =
		shift && start >= 0 && end >= 0
			? order.slice(Math.min(start, end), Math.max(start, end) + 1)
			: [target];
	for (const id of ids) {
		if (checked) next[id] = true;
		else delete next[id];
	}
	return next;
}
