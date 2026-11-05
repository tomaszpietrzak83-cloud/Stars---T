//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func colonizerUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignSantaMaria, DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Colony Ship", Design: DesignSantaMaria.Name, At: "Home", Cargo: Cargo{Colonists: 25}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500}}, {Name: "Near", Position: Vector{20, 0}, Hab: &Hab{60, 50, 50}}, {Name: "Far", Position: Vector{100, 0}, Hab: &Hab{50, 50, 50}}}})
	p := u.Player(1)
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("Near"), u.Planet("Far")}
	for _, planet := range p.PlanetIntels {
		planet.ReportAge = 0
	}
	p.Automation.FleetRoles = map[int]string{u.Fleet("Colony Ship").Num: "colonizer:auto"}
	return u
}

func TestClaimAdjusterColonizersRankByMaximumAvailableTerraforming(t *testing.T) {
	for _, mode := range []string{"auto", "best"} {
		t.Run(mode, func(t *testing.T) {
			u := colonizerUniverse(t)
			p, f := u.Player(1), u.Fleet("Colony Ship")
			p.Race = *NewRace().WithPRT(CA).WithSpec(&u.Game.Rules)
			p.WithTechLevels(TechLevel{10, 10, 10, 10, 10, 10})
			u.Recompute()
			p.Automation.ServeTerraformingWorlds = false
			p.Automation.FleetRoles[f.Num] = "colonizer:" + mode
			a, b := u.Planet("Near"), u.Planet("Far")
			a.Position, b.Position = Vector{40, 0}, Vector{0, 40}
			ability := NewTerraformer().GetTerraformAbility(p).Grav
			require.Positive(t, ability)
			// Reports may show a previous owner's environmental adjustments.
			// CA occupation uses the base environment, not that current scan.
			a.Hab, a.BaseHab = Hab{70, 50, 50}, Hab{50, 50, 50}
			b.Hab, b.BaseHab = Hab{50, 50, 50}, Hab{50 + ability + 10, 50, 50}
			require.Less(t, p.Race.GetPlanetHabitability(a.Hab), p.Race.GetPlanetHabitability(b.Hab))
			qualityA, qualityB := prospectivePlanetQuality(p, a), prospectivePlanetQuality(p, b)
			require.Equal(t, 100, qualityA)
			require.Positive(t, qualityB)
			require.Greater(t, qualityA, qualityB)
			u.Run((*turnGenerator).planColonizationAutomation)
			job := readColonizationJob(f)
			require.Equal(t, a.Num, job.TargetNum)
			require.Equal(t, qualityA, job.Quality)
			require.Equal(t, Hab{70, 50, 50}, a.Hab, "planning must not alter the planet")
			// Compare the forecast with the native occupation effect on this
			// isolated test universe, without advancing a turn or a saved game.
			a.PlayerNum, b.PlayerNum = p.Num, p.Num
			u.Run((*turnGenerator).instaform)
			require.Equal(t, qualityA, p.Race.GetPlanetHabitability(a.Hab))
			require.Equal(t, qualityB, p.Race.GetPlanetHabitability(b.Hab))
		})
	}
}
func TestAutomaticColonizationNearestFarthestBestModes(t *testing.T) {
	for _, test := range []struct{ mode, want string }{{"nearest", "Near"}, {"farthest", "Far"}, {"best", "Far"}} {
		t.Run(test.mode, func(t *testing.T) {
			u := colonizerUniverse(t)
			p, f := u.Player(1), u.Fleet("Colony Ship")
			p.Automation.FleetRoles[f.Num] = "colonizer:" + test.mode
			u.Run((*turnGenerator).planColonizationAutomation)
			job := readColonizationJob(f)
			require.NotNil(t, job)
			require.Equal(t, u.Planet(test.want).Num, job.TargetNum)
			require.Equal(t, WaypointTaskColonize, f.Waypoints[len(f.Waypoints)-1].Task)
		})
	}
}
func TestAutomaticColonizationAutoScoreUsesActualEngineTravelTime(t *testing.T) {
	for _, test := range []struct {
		maxWarp int
		want    string
	}{{4, "Near"}, {10, "Far"}} {
		t.Run(test.want, func(t *testing.T) {
			u := colonizerUniverse(t)
			f := u.Fleet("Colony Ship")
			f.Spec.Engine.MaxSafeSpeed = test.maxWarp
			for _, token := range f.Tokens {
				token.design.Spec.Engine.MaxSafeSpeed = test.maxWarp
				token.design.Spec.Engine.FreeSpeed = 0
				token.design.Spec.Engine.FuelUsage = []int{0, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10}
			}
			u.Run((*turnGenerator).planColonizationAutomation)
			job := readColonizationJob(f)
			require.Equal(t, u.Planet(test.want).Num, job.TargetNum)
			require.Positive(t, job.Score)
		})
	}
}
func TestAutomaticColonizationSplitsStacksAndReservesDifferentWorlds(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	f.Tokens[0].Quantity = 2
	f.Cargo.Colonists = 50
	f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
	u.Run((*turnGenerator).planColonizationAutomation)
	claimed := map[int]bool{}
	population := 0
	for _, fleet := range u.Game.Fleets {
		if fleet.Starbase {
			continue
		}
		require.Equal(t, 1, fleet.Spec.TotalShips)
		require.Equal(t, "colonizer:auto", p.Automation.FleetRoles[fleet.Num])
		job := readColonizationJob(fleet)
		require.NotZero(t, job.TargetNum)
		require.False(t, claimed[job.TargetNum])
		claimed[job.TargetNum] = true
		population += fleet.Cargo.Colonists
	}
	require.Len(t, claimed, 2)
	require.Equal(t, 50, population)
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Len(t, u.Game.Fleets, 2)
}
func TestAutomaticColonizationKeepsTargetAndReassignsOccupiedWorld(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.Automation.FleetRoles[f.Num] = "colonizer:nearest"
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, u.Planet("Near").Num, readColonizationJob(f).TargetNum)
	u.Planet("Far").Position = Vector{1, 0}
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, u.Planet("Near").Num, readColonizationJob(f).TargetNum, "do not change a valid assignment every turn")
	u.Planet("Near").PlayerNum = 2
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, u.Planet("Far").Num, readColonizationJob(f).TargetNum)
}
func TestAutomaticColonizationLoadsSettlersAndCompletesNormalColonization(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.Automation.FleetRoles[f.Num] = "colonizer:nearest"
	f.Cargo.Colonists = 0
	before := u.Planet("Home").Cargo.Colonists
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Positive(t, f.Cargo.Colonists)
	require.Equal(t, before, u.Planet("Home").Cargo.Colonists+f.Cargo.Colonists)
	require.GreaterOrEqual(t, u.Planet("Home").Cargo.Colonists, (before+1)/2)
	for range 8 {
		require.NoError(t, u.turn.generateTurn())
		if f.Delete {
			break
		}
	}
	require.True(t, f.Delete)
	require.Equal(t, p.Num, u.Planet("Near").PlayerNum)
	require.Positive(t, u.Planet("Near").Cargo.Colonists)
}
func TestAutomaticColonizationSkipsUnknownAndManualReservedWorlds(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	u.Planet("Far").ReportAge = ReportAgeUnexplored
	u.Planet("Near").ReportAge = ReportAgeUnexplored
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Zero(t, readColonizationJob(f).TargetNum)
	for _, wp := range f.Waypoints {
		require.NotEqual(t, WaypointTaskColonize, wp.Task)
	}
	u.Planet("Near").ReportAge = 0
	u.Planet("Far").ReportAge = 0
	other, err := u.turn.buildFleet(p, u.Planet("Home"), ShipToken{DesignNum: f.Tokens[0].DesignNum, Quantity: 1, design: f.Tokens[0].design}, nil, nil)
	require.NoError(t, err)
	other.Cargo.Colonists = 25
	other.Waypoints = []Waypoint{{Position: other.Position}, {Position: u.Planet("Near").Position, MapObjectTarget: u.Planet("Near").MapObject.ToTarget(), Task: WaypointTaskColonize, WarpSpeed: 5}}
	p.Automation.FleetRoles[f.Num] = "colonizer:nearest"
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, u.Planet("Far").Num, readColonizationJob(f).TargetNum)
}
func TestAutomaticColonizationRejectsMixedFleetAndAcceptsColonyModes(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	for _, mode := range []string{"auto", "nearest", "farthest", "best"} {
		require.NoError(t, p.ValidateLogisticsAutomation(LogisticsAutomation{FleetRoles: map[int]string{f.Num: "colonizer:" + mode}}, u.Game.Fleets))
	}
	scout := p.Designs[1]
	f.Tokens = append(f.Tokens, ShipToken{DesignNum: scout.Num, Quantity: 1, design: scout})
	f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
	require.NoError(t, p.ValidateLogisticsAutomation(LogisticsAutomation{FleetRoles: map[int]string{f.Num: "colonizer:auto"}}, u.Game.Fleets)) // existing role waits rather than turning off
	delete(p.Automation.FleetRoles, f.Num)
	require.Error(t, p.ValidateLogisticsAutomation(LogisticsAutomation{FleetRoles: map[int]string{f.Num: "colonizer:auto"}}, u.Game.Fleets))
	f.Tokens = slices.Delete(f.Tokens, 1, 2)
	forged := ProductionQueueItem{Type: QueueItemTypeShipToken, DesignNum: 1, Quantity: 1, Tags: Tags{colonizationTag: "forged"}}
	require.Error(t, validateMaintenanceQueueEdits(nil, []ProductionQueueItem{forged}))
}
func TestColonizerAtDestinationDoesNotWaitForUnnecessaryFuel(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.Automation.FleetRoles[f.Num] = "colonizer:nearest"
	f.Position = u.Planet("Near").Position
	f.OrbitingPlanetNum = u.Planet("Near").Num
	f.Fuel = 0
	u.Run((*turnGenerator).planColonizationAutomation, (*turnGenerator).guardAutomaticFuel)
	require.Equal(t, WaypointTaskColonize, f.Waypoints[0].Task)
	require.Nil(t, readAutomaticFuelRequest(f))
	u.Run((*turnGenerator).fleetColonize, (*turnGenerator).resolveColonistDrops)
	require.True(t, f.Delete)
	require.Equal(t, p.Num, u.Planet("Near").PlayerNum)
}

func TestBestColonizerReplacesUnreachableReservationWithReachableWorld(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.Automation.FleetRoles[f.Num] = "colonizer:best"
	f.Position = Vector{20, 0}
	f.OrbitingPlanetNum = 0
	f.Fuel = 5
	for i := range f.Tokens {
		f.Tokens[i].design.Spec.Engine.FreeSpeed = 0
		f.Tokens[i].design.Spec.Engine.FuelUsage = []int{0, 1000000, 1000000, 1000000, 1000000, 1000000, 1000000, 1000000, 1000000, 1000000, 1000000}
	}
	far := u.Planet("Far")
	far.Position = Vector{5000, 0}
	writeColonizationJob(f, &colonizationJob{TargetNum: far.Num, Mode: "best", Kind: "colonize"})
	near := u.Planet("Near")
	near.Position = f.Position
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, near.Num, readColonizationJob(f).TargetNum)
	require.Equal(t, WaypointTaskColonize, f.Waypoints[len(f.Waypoints)-1].Task)
	require.Equal(t, "colonizer:best", p.Automation.FleetRoles[f.Num])
}

func TestColonizationRejectsRoutesLongerThanFiftyTurns(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.Automation.FleetRoles[f.Num] = "colonizer:best"
	f.Fuel = 100000
	f.Spec.FuelCapacity = 100000
	for i := range f.Tokens {
		f.Tokens[i].design.Spec.Engine.MaxSafeSpeed = 5
		f.Tokens[i].design.Spec.Engine.IdealSpeed = 5
		f.Tokens[i].design.Spec.Engine.FuelUsage = []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	}
	f.Spec.Engine.MaxSafeSpeed = 5
	f.Spec.Engine.IdealSpeed = 5
	far := u.Planet("Far")
	far.Position = Vector{1275, 0}
	require.Nil(t, planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, far.Position, far.MapObject.ToTarget(), automaticRoutePolicy{OneWay: true}))
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, u.Planet("Near").Num, readColonizationJob(f).TargetNum)
}

func TestColonizerUnloadsAtAlreadyOwnedTargetAndKeepsRole(t *testing.T) {
	for _, retire := range []bool{false, true} {
		t.Run(fmt.Sprint(retire), func(t *testing.T) {
			u := colonizerUniverse(t)
			p, f, planet := u.Player(1), u.Fleet("Colony Ship"), u.Planet("Home")
			before := planet.Cargo.Colonists
			load := f.Cargo.Colonists
			writeColonizationJob(f, &colonizationJob{TargetNum: planet.Num, Kind: "colonize", Mode: "auto"})
			if retire {
				p.DesignMaintenance = DesignMaintenance{f.Tokens[0].DesignNum: RetireDesign}
			}
			u.Run((*turnGenerator).unloadArrivedColonizers)
			require.Zero(t, f.Cargo.Colonists)
			require.Equal(t, before+load, planet.Cargo.Colonists)
			require.Equal(t, "colonizer:auto", p.Automation.FleetRoles[f.Num])
			require.Equal(t, "load", readColonizationJob(f).Kind)
			if retire {
				tg := newTurnGenerator(u.Game)
				require.True(t, tg.maintenanceNeeded(p, f))
			}
		})
	}
}

func TestAutomaticColonizersUseRedWorldsOnlyAsFallback(t *testing.T) {
	u := colonizerUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Colony Ship")
	red, green := u.Planet("Near"), u.Planet("Far")
	red.Hab, red.BaseHab = Hab{100, 100, 100}, Hab{100, 100, 100}
	p.Automation.FleetRoles[fleet.Num] = "colonizer:nearest"
	p.Automation.PriorityPlanets = map[int]bool{red.Num: true}

	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, green.Num, readColonizationJob(fleet).TargetNum)

	green.PlayerNum = 2
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, red.Num, readColonizationJob(fleet).TargetNum)
	fleet.Position = red.Position
	fleet.Waypoints = fleet.Waypoints[len(fleet.Waypoints)-1:]
	u.Run((*turnGenerator).fleetColonize, (*turnGenerator).resolveColonistDrops)
	require.True(t, red.OwnedBy(p.Num))
	require.False(t, suitableColony(p, red))
	require.Empty(t, red.ProductionQueue)
}

func TestAutomaticRedColonyStartsPaidTerraforming(t *testing.T) {
	u := colonizerUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Colony Ship")
	p.WithTechLevels(TechLevel{10, 10, 10, 10, 10, 10})
	red := u.Planet("Near")
	found := false
	for grav := 0; grav <= 100; grav++ {
		red.Hab, red.BaseHab = Hab{grav, 50, 50}, Hab{grav, 50, 50}
		if p.Race.GetPlanetHabitability(red.Hab) < 0 && terraformedPlanetQuality(p, red) > 0 {
			found = true
			break
		}
	}
	require.True(t, found)
	u.Planet("Far").PlayerNum = 2
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, red.Num, readColonizationJob(fleet).TargetNum)
	fleet.Position = red.Position
	fleet.Waypoints = fleet.Waypoints[len(fleet.Waypoints)-1:]
	u.Run((*turnGenerator).fleetColonize, (*turnGenerator).resolveColonistDrops)
	require.True(t, red.OwnedBy(p.Num))
	require.False(t, suitableColony(p, red))
	require.NotEmpty(t, red.ProductionQueue)
	require.Equal(t, QueueItemTypeAutoMinTerraform, red.ProductionQueue[0].Type)
}
