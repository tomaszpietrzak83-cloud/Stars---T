import { expect, it } from 'vitest';
import { rangeSelection } from './rangeSelection';
it('uses the current sorted order, preserves unrelated picks and supports unchecking', () => {
	expect(rangeSelection({ 99: true }, [4, 3, 2, 1], 3, 1, true, true)).toEqual({
		99: true,
		3: true,
		2: true,
		1: true
	});
	expect(
		rangeSelection({ 1: true, 2: true, 3: true, 99: true }, [3, 2, 1], 1, 3, false, true)
	).toEqual({ 99: true });
	expect(rangeSelection({ 99: true }, [4, 2], 3, 2, true, true)).toEqual({ 99: true, 2: true });
});
