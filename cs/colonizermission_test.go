//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchivedColonizerCommittedRoutes(t *testing.T) {
	path := os.Getenv("STARS_COLONIZATION_MISSION_ARCHIVE")
	if path == "" {
		t.Skip("optional private copied-archive check")
	}
	absolute, err := filepath.Abs(path)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/")
	file, err := os.Open(absolute)
	require.NoError(t, err)
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer zipped.Close()
	var archive struct{ Game FullGame }
	require.NoError(t, json.NewDecoder(zipped).Decode(&archive))
	g := &archive.Game
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	tg := newTurnGenerator(g)
	require.NoError(t, g.computeSpecs())
	tg.enablePlanningThreatIndex()
	defer tg.disablePlanningThreatIndex()
	tg.refreshAutomaticFuelIntel()
	p := g.getPlayer(1)
	for _, num := range []int{247, 283} {
		f := g.getFleet(1, num)
		require.NotNil(t, f)
		job := readColonizationJob(f)
		require.NotNil(t, job)
		target := job.TargetNum
		cargo, fuel := f.Cargo, f.Fuel
		tg.continueColonizerMission(p, f, job, p.GetPlanetIntel(target))
		require.Equal(t, target, readColonizationJob(f).TargetNum)
		require.Equal(t, cargo, f.Cargo)
		require.Equal(t, fuel, f.Fuel)
		t.Logf("fleet%d target%d: %s", num, target, readColonizationJob(f).Status)
	}
	f := g.getFleet(1, 190)
	require.NotNil(t, f)
	job := readColonizationJob(f)
	job.Kind = "deliver-retire"
	tg.deliverRetiringColonizer(p, f, job)
	require.Equal(t, 250, f.Cargo.Colonists)
	require.True(t, len(f.Waypoints) > 1 || readAutomaticFuelRequest(f) != nil)
	t.Logf("fleet190: %s; route=%v", readColonizationJob(f).Status, f.Waypoints)
}

func TestColonizerFundedRouteSurvivesFuelGuard(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	u.Run((*turnGenerator).planColonizationAutomation)
	require.True(t, fundedColonizerWaypoints(&u.Game.Rules, p, f))
	original := append([]Waypoint(nil), f.Waypoints...)
	u.Run((*turnGenerator).guardAutomaticFuel)
	require.Equal(t, original, f.Waypoints)
}

func TestLandingRestoresColonizerRoleAfterMergedTransportDrop(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	f.Tokens[0].Quantity = 2
	f.Cargo = Cargo{}
	f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
	p.Automation.FleetRoles[f.Num] = "freighter"
	raw, _ := json.Marshal(map[int]string{f.Tokens[0].DesignNum: "colonizer:best"})
	f.SetTag("landing-colonizer-roles", string(raw))
	u.Run(func(tg *turnGenerator) { tg.disperseEmptyLandingTransports(p, f) })
	count := 0
	for _, ship := range u.Game.getFleets(p.Num) {
		if ship.Starbase || ship.Delete {
			continue
		}
		count++
		require.Equal(t, "colonizer:best", p.Automation.FleetRoles[ship.Num])
		require.Equal(t, "passengers", readColonizationJob(ship).Kind)
	}
	require.Equal(t, 2, count)
}

func TestColonizerMissionIgnoresModePriorityAndTemporaryFuelLoss(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.Automation.FleetRoles[f.Num] = "colonizer:best"
	u.Run((*turnGenerator).planColonizationAutomation)
	target := readColonizationJob(f).TargetNum
	p.Automation.FleetRoles[f.Num] = "colonizer:nearest"
	p.Automation.PriorityPlanets = map[int]bool{u.Planet("Near").Num: true}
	f.Position, f.Fuel = Vector{5, 0}, 0
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, target, readColonizationJob(f).TargetNum)
}

func TestColonizerPendingUpgradeDoesNotInterruptMission(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.DesignMaintenance = map[int]int{f.Tokens[0].DesignNum: RetireDesign}
	u.Run((*turnGenerator).planColonizationAutomation)
	target := readColonizationJob(f).TargetNum
	u.Run((*turnGenerator).fleetDesignMaintenance, (*turnGenerator).planColonizationAutomation)
	require.False(t, f.InDesignMaintenance())
	require.Equal(t, target, readColonizationJob(f).TargetNum)
	require.Equal(t, WaypointTaskColonize, f.Waypoints[len(f.Waypoints)-1].Task)
}

func TestColonizerReleasesLegacyRefitWithoutDroppingMission(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	writeColonizationJob(f, &colonizationJob{Kind: "colonize", Mode: "best", TargetNum: u.Planet("Far").Num})
	writeMaintenance(f, &fleetMaintenance{OriginalRole: "colonizer:best", Group: "legacy", PlanetNum: u.Planet("Home").Num})
	p.DesignMaintenance = map[int]int{f.Tokens[0].DesignNum: RetireDesign}
	u.Run((*turnGenerator).fleetDesignMaintenance, (*turnGenerator).planColonizationAutomation)
	require.False(t, f.InDesignMaintenance())
	require.Equal(t, u.Planet("Far").Num, readColonizationJob(f).TargetNum)
}

func TestColonizerReservationRequiresRealVoyage(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	u.Run((*turnGenerator).planColonizationAutomation)
	job := readColonizationJob(f)
	require.True(t, colonizerTravelling(p, f, job))
	f.Waypoints = []Waypoint{{Position: f.Position}}
	require.False(t, colonizerTravelling(p, f, job))
}

func TestColonizerUnexpectedOwnerReceivesGroundDropAndHullSurvives(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignSantaMaria, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Colony", Design: DesignSantaMaria.Name, At: "Target", Cargo: Cargo{Colonists: 25}}}}, {Designs: Designs(DesignSpaceStation)}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Target", Owner: 2, Position: Vector{20, 0}, Cargo: Cargo{Colonists: 1}}}})
	p, f := u.Player(1), u.Fleet("Colony")
	target := u.Planet("Target")
	p.Automation.FleetRoles = map[int]string{f.Num: "colonizer:best"}
	p.PlanetIntels = []*Planet{u.Planet("Home"), target}
	f.Waypoints = []Waypoint{{Position: target.Position, MapObjectTarget: target.ToTarget(), Task: WaypointTaskColonize}}
	writeColonizationJob(f, &colonizationJob{Kind: "colonize", TargetNum: target.Num, Mode: "best"})
	u.Run((*turnGenerator).fleetColonize, (*turnGenerator).resolveColonistDrops)
	require.False(t, f.Delete)
	require.Zero(t, f.Cargo.Colonists)
	require.True(t, target.OwnedBy(p.Num))
	require.Equal(t, "passengers", readColonizationJob(f).Kind)
}

func TestColonizerRetirementUsesFundedReturnTime(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	f.Position = Vector{100, 0}
	f.Fuel = 0
	f.Cargo.Colonists = 0
	f.Spec.Engine.MaxSafeSpeed = 1
	for i := range f.Tokens {
		f.Tokens[i].design.Spec.Engine.MaxSafeSpeed = 1
		f.Tokens[i].design.Spec.Engine.FreeSpeed = 1
	}
	u.Run(func(tg *turnGenerator) { tg.completedColonizerTask(p, f, &colonizationJob{Mode: "best"}) })
	require.Equal(t, "retire", readColonizationJob(f).Kind)
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, WaypointTaskScrapFleet, f.Waypoints[len(f.Waypoints)-1].Task)
	require.Equal(t, u.Planet("Home").Num, f.Waypoints[len(f.Waypoints)-1].TargetNum)
}

func TestColonizerPassengersCanJoinLandingButCommittedShipCannot(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	writeColonizationJob(f, &colonizationJob{Mode: "best", Kind: "passengers"})
	require.True(t, landingTransport(p, f))
	u.Run(func(tg *turnGenerator) {
		choices := tg.landingPickups(p, u.Planet("Far"), f.Position, map[int]int{}, 1)
		require.Len(t, choices, 1)
		require.Same(t, f, choices[0].fleet)
	})
	writeColonizationJob(f, &colonizationJob{Mode: "best", Kind: "colonize", TargetNum: u.Planet("Far").Num})
	require.False(t, landingTransport(p, f))
}
