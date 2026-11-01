package ai

import (
	"github.com/sirgwain/craig-stars/cs"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func TestPlanetIndexMatchesLinearWithClaimedTargetsAndTies(t *testing.T) {
	rng := rand.New(rand.NewSource(72))
	active := map[int]*cs.Planet{}
	for n := 1; n <= 1200; n++ {
		active[n] = &cs.Planet{MapObject: cs.MapObject{Num: n, Position: cs.Vector{X: rng.Intn(10000) - 5000, Y: rng.Intn(10000) - 5000}}}
	}
	index := newPlanetIntelIndex(active)
	computer := &aiPlayer{}
	for n := 0; n < 1500; n++ {
		at := cs.Vector{X: rng.Intn(10000) - 5000, Y: rng.Intn(10000) - 5000}
		expected := computer.getClosestPlanetIntel(at, active)
		require.Same(t, expected, index.nearest(at, active))
		if expected != nil && n%2 == 0 {
			delete(active, expected.Num)
		}
	}
	tied := map[int]*cs.Planet{2: {MapObject: cs.MapObject{Num: 2, Position: cs.Vector{X: 1}}}, 1: {MapObject: cs.MapObject{Num: 1, Position: cs.Vector{X: -1}}}}
	require.Equal(t, 1, newPlanetIntelIndex(tied).nearest(cs.Vector{}, tied).Num)
	for n := range active {
		delete(active, n)
	}
	require.Nil(t, index.nearest(cs.Vector{}, active))
}
