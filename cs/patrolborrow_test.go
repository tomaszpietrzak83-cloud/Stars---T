//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEmergencyCoverDoesNotTakeEveryHunter(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{
		{Name: "Bomber", Design: DesignTeamster.Name, At: "Home"},
		{Name: "A", Design: DesignDestroyerDelta.Name, At: "Home"},
		{Name: "B", Design: DesignDestroyerDelta.Name, At: "Home"},
		{Name: "C", Design: DesignDestroyerDelta.Name, At: "Home"},
	}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, bomber := u.Player(1), u.Fleet("Bomber")
	p.Automation.FleetRoles = map[int]string{bomber.Num: "bomber"}
	for _, name := range []string{"A", "B", "C"} {
		p.Automation.FleetRoles[u.Fleet(name).Num] = "patrol"
	}
	p.FleetIntels = []*Fleet{{MapObject: MapObject{PlayerNum: 2, Num: 1, Position: Vector{30, 0}}, WarpSpeed: 5, Heading: VectorFloat64{X: -1}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true, PowerRating: 1}}}}
	u.Run(func(tg *turnGenerator) { tg.assignBomberProtection(p, bomber, bomber.Position, true) })
	borrowed := 0
	for _, name := range []string{"A", "B", "C"} {
		if u.Fleet(name).GetTag(bomberBorrowTag) != "" {
			borrowed++
		}
	}
	require.Equal(t, 1, borrowed, "one adequate local guard must leave the other hunters free")
}

func TestUnreachableBorrowedPatrolReturnsToHunting(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, bomber, f := u.Player(1), u.Fleet("Bomber"), u.Fleet("Cover")
	p.Automation.FleetRoles[f.Num] = "patrol"
	f.Position, f.Fuel = Vector{500, 500}, 0
	borrowBomberProtection(p, f, bomber)
	saveAutomaticFuelRequest(f, automaticFuelRequest{Destination: bomber.Position, Urgent: true})
	u.Run((*turnGenerator).releaseUnreachablePatrolSupport)
	require.Equal(t, "patrol", p.Automation.FleetRoles[f.Num])
	require.Empty(t, f.GetTag(escortTag))
	require.Empty(t, f.GetTag(bomberBorrowTag))
	require.Nil(t, readAutomaticFuelRequest(f))
}
