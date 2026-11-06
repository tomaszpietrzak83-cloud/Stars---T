//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPatrolDispersesInsufficientPackAndHuntsInSamePass(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignJihadCruiser, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Pack", Design: DesignJihadCruiser.Name, Quantity: 3}}},
		{Designs: Designs(DesignJihadCruiser, DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Overwhelming", Design: DesignJihadCruiser.Name, Quantity: 100, Position: Vector{200, 0}}, {Name: "Prey", Design: DesignLongRangeScout.Name, Position: Vector{30, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, f, e := u.Player(1), u.Fleet("Pack"), u.Fleet("Overwhelming")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	for _, enemy := range u.Game.getFleets(2) {
		p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
		p.discoverer.discoverFleet(enemy, true)
	}
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: e.Num, Grouped: true})
	u.Run((*turnGenerator).startPatrolEvaluation, func(tg *turnGenerator) { tg.advancePatrol(p, f) })
	ships, hunters := 0, 0
	for _, ship := range u.Game.getFleets(1) {
		if ship.Starbase || ship.Delete {
			continue
		}
		require.Equal(t, 1, ship.Spec.TotalShips)
		require.Greater(t, len(ship.Waypoints), 1, "released ships receive work immediately")
		require.False(t, readPatrolJob(ship).Provisional)
		ships++
		if readPatrolJob(ship).Target == u.Fleet("Prey").Num {
			hunters++
		}
	}
	require.Equal(t, 3, ships)
	require.Equal(t, 1, hunters, "do not assign every hunter to the same single target")
}

func TestPatrolSamePositionAndRouteDoesNotMergeHunters(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{
		Designs: Designs(DesignDestroyerDelta),
		Fleets:  []ScenarioFleet{{Name: "A", Design: DesignDestroyerDelta.Name}, {Name: "B", Design: DesignDestroyerDelta.Name}},
	}}})
	p, a, b := u.Player(1), u.Fleet("A"), u.Fleet("B")
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol"}
	b.Position = a.Position
	a.Waypoints = []Waypoint{NewPositionWaypoint(a.Position, 0)}
	b.Waypoints = []Waypoint{NewPositionWaypoint(a.Position, 0)}
	u.Run((*turnGenerator).mergeCompatibleCombatFleets)
	require.False(t, a.Delete)
	require.False(t, b.Delete)
}

func TestPatrolUnreadyRendezvousDoesNotMerge(t *testing.T) {
	u := swarmUniverse(t)
	a, b := u.Fleet("Local A"), u.Fleet("Local B")
	u.Run((*turnGenerator).startPatrolEvaluation, (*turnGenerator).mergePatrolRendezvous)
	require.False(t, a.Delete)
	require.False(t, b.Delete)
}

func TestSalvageSmallSufficientHoldBeatsNearbyOversizedHold(t *testing.T) {
	require.True(t, betterSalvageCollector(70, 4, 2, 210, 1, 1, 12))
	require.False(t, betterSalvageCollector(70, 1, 2, 210, 4, 1, 180))
	require.True(t, betterSalvageCollector(210, 4, 2, 70, 1, 1, 500))
	require.True(t, betterSalvageCollector(70, 1, 2, 70, 4, 1, 12))
}

func TestSalvageDispatchPrefersSmallFreighterForSmallDebris(t *testing.T) {
	u := logisticsUniverse(t)
	p, small, large := u.Player(1), u.Fleet("Escort"), u.Fleet("Transport")
	p.Automation.FleetRoles[small.Num] = "freighter"
	small.Spec.CargoCapacity = 70
	large.Spec.CargoCapacity = 210
	small.Position = Vector{-40, 0}
	u.Game.Salvages[0].Cargo = Cargo{Ironium: 12}
	u.Run((*turnGenerator).planPatrolSalvage)
	require.NotNil(t, logisticsWork(small))
	require.Equal(t, "salvage", logisticsWork(small).Kind)
	require.Nil(t, logisticsWork(large))
	require.Greater(t, len(small.Waypoints), 1)
}

func TestArrivedPatrolCanHuntRepeatedlyBeforeRendezvous(t *testing.T) {
	hunter := campaignDesign("Free hunter", Cruiser.Name, "patrol")
	for i := range hunter.Slots {
		if StaticTechStore.GetHullComponent(hunter.Slots[i].HullComponent).HullSlotType&HullSlotTypeEngine != 0 {
			hunter.Slots[i].HullComponent = FuelMizer.Name
		}
	}
	for _, urgent := range []bool{false, true} {
		u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
			{Player: proposalTestPlayer(), Designs: Designs(hunter, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Gatherer", Design: hunter.Name}}},
			{Designs: Designs(DesignDestroyerDelta, DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Main force", Design: DesignDestroyerDelta.Name, Quantity: 20, Position: Vector{200, 0}}, {Name: "On the way", Design: DesignLongRangeScout.Name, Position: Vector{16, 0}}}},
		}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
		p, f, e := u.Player(1), u.Fleet("Gatherer"), u.Fleet("Main force")
		f.Fuel = 0
		p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
		for _, enemy := range u.Game.getFleets(2) {
			p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
			p.discoverer.discoverFleet(enemy, true)
		}
		if urgent {
			r := p.GetFleetIntel(2, e.Num)
			r.WarpSpeed = 10
			r.Heading = VectorFloat64{X: -1}
		}
		u.Run((*turnGenerator).startPatrolEvaluation)
		job := patrolJob{Leader: f.Num, Enemy: 2, Target: e.Num, Provisional: true, Rally: Vector{0, 0}, RendezvousBy: u.Game.Year + 3, CleanupUsed: true, GatheringSince: u.Game.Year - 1}
		savePatrolJob(f, job)
		f.Waypoints = []Waypoint{NewPositionWaypoint(f.Position, 0)}
		u.Run(func(tg *turnGenerator) { tg.patrolGatheringCleanup(p, f, p.GetFleetIntel(2, e.Num), &job, []*Fleet{f}) })
		if urgent {
			require.Zero(t, readPatrolJob(f).CleanupTarget)
			continue
		}
		require.NotZero(t, readPatrolJob(f).CleanupTarget)
		require.Equal(t, job.RendezvousBy, readPatrolJob(f).RendezvousBy)
		require.Equal(t, u.Fleet("On the way").Num, patrolTacticalEnemy(p, f).Num)
		require.True(t, patrolCanAttack(p, f))
		require.True(t, readPatrolJob(f).CleanupUsed)
		t.Logf("CLEANUP funded local kill: out=%v, main deadline=%d retained", f.Waypoints, job.RendezvousBy)
	}
}

func TestSmallDebrisDoesNotDivertLargeFreighterWhileSmallIsBusy(t *testing.T) {
	u := logisticsUniverse(t)
	p, small, large := u.Player(1), u.Fleet("Escort"), u.Fleet("Transport")
	p.Automation.FleetRoles[small.Num] = "freighter"
	small.Spec.CargoCapacity = 70
	large.Spec.CargoCapacity = 210
	small.Cargo = Cargo{Colonists: 70}
	u.Game.Salvages[0].Cargo = Cargo{Ironium: 5}
	u.Run((*turnGenerator).planPatrolSalvage, (*turnGenerator).planLogisticsAutomation)
	if job := logisticsWork(large); job != nil {
		require.NotEqual(t, "salvage", job.Kind)
	}
	require.Equal(t, 70, small.Cargo.Colonists, "never interrupt loaded passenger delivery")
}
