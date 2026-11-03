//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func frontierAdvanceUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Hunter", Design: DesignDestroyerDelta.Name, At: "Home"}}}, {Designs: Designs(DesignDestroyerDelta, DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Front", Design: DesignDestroyerDelta.Name, Quantity: 20, Position: Vector{600, 0}}, {Name: "Next", Design: DesignDestroyerDelta.Name, Quantity: 20, Position: Vector{900, 0}}, {Name: "Weak", Design: DesignLongRangeScout.Name, Position: Vector{20, 0}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Border", Owner: 1, Starbase: DesignSpaceStation.Name, Position: Vector{400, 0}}}})
	p, f := u.Player(1), u.Fleet("Hunter")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	p.discoverer.discoverFleet(u.Fleet("Front"), false)
	addExpeditionTestDock(p, Vector{})
	addExpeditionTestDock(p, Vector{400, 0})
	return u
}

func TestQuietSectorPatrolAdvancesToThreatAndThenFurther(t *testing.T) {
	u := frontierAdvanceUniverse(t)
	p, f := u.Player(1), u.Fleet("Hunter")
	f.SetTag(patrolSectorTag, "1")
	f.SetTag(frontierPatrolTag, "old local loop")
	u.Run(func(tg *turnGenerator) { require.True(t, tg.patrolFrontier(p, f)) }, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).planAutomaticSafety, (*turnGenerator).recoverIdleAutomaticFleets, (*turnGenerator).enforceAutomaticDeparture)
	require.NotEmpty(t, f.GetTag(patrolFrontTag))
	require.Len(t, f.Waypoints, 2)
	require.Greater(t, f.Waypoints[1].Position.X, 0)
	require.GreaterOrEqual(t, f.Waypoints[1].Position.DistanceTo(u.Fleet("Front").Position), 80.0)
	require.False(t, patrolCanAttack(p, f), "frontier concentration is not permission for an unknown-fleet attack")
	f.Position = f.Waypoints[1].Position
	f.Waypoints = []Waypoint{{Position: f.Position}}
	p.FleetIntels = nil
	p.discoverer.discoverFleet(u.Fleet("Next"), false)
	before := f.Position.X
	u.Run(func(tg *turnGenerator) { require.True(t, tg.patrolFrontier(p, f)) })
	require.Greater(t, f.Waypoints[1].Position.X, before, "clearing one front must allow further outward progress")
}

func TestLocalKillStillPrecedesFrontierRelocation(t *testing.T) {
	u := frontierAdvanceUniverse(t)
	p, f, e := u.Player(1), u.Fleet("Hunter"), u.Fleet("Weak")
	p.discoverer.discoverDesign(e.Tokens[0].design, true)
	p.discoverer.discoverFleet(e, true)
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Equal(t, e.Num, readPatrolJob(f).Target)
	require.True(t, patrolCanAttack(p, f))
	require.Equal(t, e.Position, f.Waypoints[1].Position)
}

func TestD188D193AdvanceOnCopied2453(t *testing.T) {
	path := os.Getenv("STARS_PATROL_FRONTIER_SNAPSHOT")
	if path == "" {
		t.Skip("optional isolated copy")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var game FullGame
	require.NoError(t, json.Unmarshal(raw, &game))
	game.TechStore = &StaticTechStore
	game.Rules.SetTechStore(&StaticTechStore)
	game.Universe.setLogger(testLogger)
	require.NoError(t, game.computeSpecs())
	tg := newTurnGenerator(&game)
	tg.ordersPlayerNum = 1
	year := game.Year
	for _, num := range []int{188, 193} {
		f := game.getFleet(1, num)
		require.NotNil(t, f)
		require.Len(t, f.Waypoints, 1)
	}
	tg.planPatrolAutomation()
	tg.guardAutomaticFuel()
	tg.planAutomaticSafety()
	tg.recoverIdleAutomaticFleets()
	tg.enforceAutomaticDeparture()
	for _, num := range []int{188, 193} {
		f := game.getFleet(1, num)
		if f.Delete {
			f = game.getFleet(1, readPatrolJob(f).Leader)
			require.NotNil(t, f)
		}
		require.Len(t, f.Waypoints, 2)
		require.True(t, f.GetTag(patrolFrontTag) != "" || readPatrolJob(f).Leader != 0, "must advance to frontier or intercept an encountered target")
		require.NotEqual(t, f.Position, f.Waypoints[1].Position)
		t.Logf("D%d: %v -> %v, warp %d; %s", num, f.Position, f.Waypoints[1].Position, f.Waypoints[1].WarpSpeed, f.GetTag(frontierPatrolTag))
	}
	require.Equal(t, year, game.Year)
	// Once the small local traffic is gone, the same quiet-sector ships must
	// advance to the remaining large formations rather than restart home loops.
	var cleared FullGame
	require.NoError(t, json.Unmarshal(raw, &cleared))
	cleared.TechStore = &StaticTechStore
	cleared.Rules.SetTechStore(&StaticTechStore)
	cleared.Universe.setLogger(testLogger)
	require.NoError(t, cleared.computeSpecs())
	cp := cleared.getPlayer(1)
	large := []*Fleet{}
	for _, enemy := range cp.FleetIntels {
		if reportedFleetShips(enemy) >= 8 {
			large = append(large, enemy)
		}
	}
	cp.FleetIntels = large
	ct := newTurnGenerator(&cleared)
	ct.ordersPlayerNum = 1
	ct.planPatrolAutomation()
	ct.guardAutomaticFuel()
	ct.planAutomaticSafety()
	ct.recoverIdleAutomaticFleets()
	ct.enforceAutomaticDeparture()
	for _, num := range []int{188, 193} {
		f := cleared.getFleet(1, num)
		require.False(t, f.Delete)
		require.NotEmpty(t, f.GetTag(patrolFrontTag))
		require.Len(t, f.Waypoints, 2)
		t.Logf("After local cleanup D%d advances %v -> %v: %s", num, f.Position, f.Waypoints[1].Position, f.GetTag(frontierPatrolTag))
	}
	require.Equal(t, year, cleared.Year)
}
