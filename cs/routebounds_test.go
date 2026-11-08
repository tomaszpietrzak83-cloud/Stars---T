package cs

import (
	"math/rand"
	"testing"
)

// Compare against the unfiltered geometry, including zero-length routes,
// exact boundary contact and both directions. A broad-phase false negative
// would change a safety decision, so test the resulting decision directly.
func TestRouteBoundsPreserveExactThreatGeometry(t *testing.T) {
	rng := rand.New(rand.NewSource(2494))
	point := func() Vector { return Vector{X: rng.Intn(4001) - 2000, Y: rng.Intn(4001) - 2000} }
	for i := 0; i < 50000; i++ {
		from, to := point(), point()
		threat := indexedRouteThreat{position: point(), end: point(), next: point(), reach: float64(rng.Intn(101))}
		if i%5 == 0 {
			to = from
		}
		if i%7 == 0 {
			threat.end = threat.position
		}
		if i%11 == 0 {
			to = threat.next.Add(Vector{X: 3})
		}
		for _, civilian := range []bool{false, true} {
			want := to.DistanceTo(threat.next) <= 3
			if got := threat.threatensRoute(from, to, civilian); got != want {
				t.Fatalf("case %d civilian=%v: got %v want %v: %+v %v %v", i, civilian, got, want, threat, from, to)
			}
		}
		want := segmentDistance(threat.position, from, to) <= threat.reach
		got := routeBoundsOverlap(from, to, threat.position, threat.position, threat.reach) && want
		if got != want {
			t.Fatalf("minefield broad phase rejected contact in case %d", i)
		}
	}
}
