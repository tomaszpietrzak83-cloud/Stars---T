//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPatrolWaitsForScoutEquipmentBeforeAttack(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta, DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Patrol", Design: DesignDestroyerDelta.Name, At: "Home"}, {Name: "Recon", Design: DesignLongRangeScout.Name, At: "Home"}}},
		{Designs: Designs(DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Unknown", Design: DesignLongRangeScout.Name, Quantity: 8, Position: Vector{20, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, f, s, e := u.Player(1), u.Fleet("Patrol"), u.Fleet("Recon"), u.Fleet("Unknown")
	p.Race.Spec.DiscoverDesignOnScan = false
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol", s.Num: "scout"}
	p.discoverer.discoverDesign(e.Tokens[0].design, false)
	p.discoverer.discoverFleet(e, false)
	addExpeditionTestDock(p, Vector{})
	require.True(t, unknownFleetEquipment(p, p.GetFleetIntel(2, e.Num)))
	p.Automation.PriorityFleets = map[string]bool{priorityFleetKey(2, e.Num): true}
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: e.Num})
	f.Waypoints = append(f.Waypoints, NewPositionWaypoint(e.Position, 6))
	require.False(t, patrolCanAttack(p, f))
	u.Run((*turnGenerator).enforceAutomaticDeparture)
	require.Len(t, f.Waypoints, 1, "saved automatic attack must be stopped too")
	u.Run((*turnGenerator).planScoutAutomation)
	require.True(t, scoutReconActive(p, s))
	require.False(t, readScoutRecon(s).Watching)
	require.Equal(t, e.Num, s.Waypoints[len(s.Waypoints)-1].TargetNum)
	s.Position = e.Position
	u.Run((*turnGenerator).scanScoutReconContacts, (*turnGenerator).planPatrolAutomation)
	require.False(t, unknownFleetEquipment(p, p.GetFleetIntel(2, e.Num)))
	require.True(t, patrolCanAttack(p, f), "known weak target becomes eligible only after real scout contact")
}

func TestUnknownConvergingFleetBlocksKnownTargetAttack(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	target := patrolTestEnemy(0, p)
	unknown := patrolTestEnemy(0)
	unknown.Tokens = []ShipToken{{DesignNum: 9999, Quantity: 8}}
	unknown.Num = 92
	unknown.Position = Vector{70, 0}
	unknown.WarpSpeed = 6
	unknown.Heading = VectorFloat64{-1, 0}
	p.FleetIntels = []*Fleet{target, unknown}
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: target.Num})
	require.False(t, patrolCanAttack(p, f))
	unknown.Heading = VectorFloat64{1, 0}
	require.True(t, patrolCanAttack(p, f), "a distant receding contact does not block a separate known encounter")
}
