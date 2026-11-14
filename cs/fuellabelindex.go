package cs

import "math/bits"

// A dominating route must have visited a subset of the candidate's nodes.
// Group by a 64-bit projection to avoid comparing unrelated routes. Collisions
// only cause extra exact comparisons; they can never discard a valid label.
// Bucket traversal changes no heap operations, tie breaks or expansion counts.
type fuelLabelIndex map[uint64][]*fuelNetworkLabel

func (index fuelLabelIndex) dominates(candidate *fuelNetworkLabel) bool {
	check := func(bucket []*fuelNetworkLabel) bool {
		for _, old := range bucket {
			if !old.discarded && fuelLabelDominates(old, candidate) {
				return true
			}
		}
		return false
	}
	mask := candidate.visitedMask
	if bits.OnesCount64(mask) <= 10 {
		for subset := mask; ; subset = (subset - 1) & mask {
			if check(index[subset]) {
				return true
			}
			if subset == 0 {
				break
			}
		}
	} else {
		for key, bucket := range index {
			if key&^mask == 0 && check(bucket) {
				return true
			}
		}
	}
	return false
}

func (index fuelLabelIndex) insert(candidate *fuelNetworkLabel) {
	for mask, bucket := range index {
		if candidate.visitedMask&^mask != 0 {
			continue
		}
		active := bucket[:0]
		for _, old := range bucket {
			if fuelLabelDominates(candidate, old) {
				old.discarded = true
			} else {
				active = append(active, old)
			}
		}
		if len(active) == 0 {
			delete(index, mask)
		} else {
			index[mask] = active
		}
	}
	index[candidate.visitedMask] = append(index[candidate.visitedMask], candidate)
}
