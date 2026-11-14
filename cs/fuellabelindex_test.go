package cs

import (
	"math/rand"
	"slices"
	"testing"
)

func originalFuelDominates(a, b *fuelNetworkLabel) bool {
	if a.turns > b.turns || a.burned > b.burned {
		return false
	}
	for i, fuel := range a.fuel {
		if fuel < b.fuel[i] {
			return false
		}
	}
	for _, node := range a.visited {
		if !slices.Contains(b.visited, node) {
			return false
		}
	}
	return true
}

func TestFuelLabelIndexMatchesOriginalIncludingMaskCollisions(t *testing.T) {
	rng := rand.New(rand.NewSource(1113498))
	index := fuelLabelIndex{}
	var original []*fuelNetworkLabel
	for n := 0; n < 4000; n++ {
		label := &fuelNetworkLabel{turns: rng.Intn(20), burned: rng.Intn(50), fuel: []int{rng.Intn(50), rng.Intn(50)}, visitedBits: make([]uint64, 4)}
		for i := 0; i < 1+rng.Intn(12); i++ {
			v := rng.Intn(256)
			label.visited = append(label.visited, v)
			label.visitedMask |= uint64(1) << (v % 64)
			label.visitedBits[v/64] |= uint64(1) << (v % 64)
		}
		want := false
		for _, old := range original {
			if fuelLabelDominates(old, label) != originalFuelDominates(old, label) {
				t.Fatal("bitset dominance differs")
			}
			if !old.discarded && originalFuelDominates(old, label) {
				want = true
			}
		}
		if index.dominates(label) != want {
			t.Fatalf("dominance differs at %d", n)
		}
		if want {
			continue
		}
		for _, old := range original {
			if !old.discarded && originalFuelDominates(label, old) {
				old.discarded = true
			}
		}
		copy := *label
		original = append(original, &copy)
		index.insert(label)
		if n%100 == 0 {
			actual := 0
			for _, bucket := range index {
				for _, l := range bucket {
					if l.discarded {
						t.Fatal("discarded label retained")
					}
					actual++
				}
			}
			expected := 0
			for _, l := range original {
				if !l.discarded {
					expected++
				}
			}
			if actual != expected {
				t.Fatalf("active labels differ: %d != %d", actual, expected)
			}
		}
	}
}
