//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIndexedDiscoveryMatchesLinearReports(t *testing.T) {
	run := func(indexed bool) []byte {
		a := NewPlayer(1, NewRace().WithSpec(&rules)).withSpec(&rules).WithNum(1)
		b := NewPlayer(2, NewRace().WithSpec(&rules)).withSpec(&rules).WithNum(2)
		shared := PlayerRelationship{Relation: PlayerRelationFriend, ShareMap: true}
		a.Relations = []PlayerRelationship{shared, shared}
		b.Relations = []PlayerRelationship{shared, shared}
		players := []*Player{a, b}
		if indexed {
			for _, p := range players {
				p.buildFleetIntelIndex()
				p.buildShipIntelIndex()
			}
		}
		for year := 0; year < 2; year++ {
			for _, owner := range players {
				fleet := testLongRangeScout(owner)
				fleet.Cargo.Ironium = 12 + year
				fleet.Position = Vector{year * 10, 0}
				design := fleet.Tokens[0].design
				for _, observer := range players {
					d := newDiscovererWithAllies(testLogger, observer, players)
					d.discoverDesign(design, true)
					d.discoverFleet(fleet, true)
					d.discoverFleetCargo(fleet)
					d.discoverFleetScanner(fleet)
				}
			}
			for _, p := range players {
				require.Len(t, p.FleetIntels, 2, "same fleet numbers belonging to different empires remain distinct")
				require.Equal(t, 12+year, p.GetFleetIntel(2, 1).Cargo.Ironium)
				if year == 0 {
					p.clearTransientIntel()
					if indexed {
						p.buildFleetIntelIndex()
					}
				}
			}
		}
		raw, err := json.Marshal([]Intels{a.Intels, b.Intels})
		require.NoError(t, err)
		return raw
	}
	require.JSONEq(t, string(run(false)), string(run(true)))
}
