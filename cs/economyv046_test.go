//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSmallMineralShortageLoadsAFullHoldAndKeepsYardStock(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	p.Automation.DisablePassengerTransport = true
	p.Automation.DisableMineralSupply = false
	source.Cargo.Ironium = 10000
	source.Cargo.Germanium = 10000
	dest.Spec.HasStarbase = true
	dest.Spec.DockCapacity = -1
	dest.Cargo.Ironium = 238
	// One full hold is useful even when the current stock floor is short by 62.
	saveLogisticsWork(f, &logisticsJob{Kind: "planet", TargetNum: source.Num, DeliveryNum: dest.Num, Position: source.Position, Load: Mineral{Ironium: 62}})
	before := source.Cargo.Ironium
	u.Run((*turnGenerator).resolveLogisticsAutomation)
	require.Equal(t, f.Spec.CargoCapacity, f.Cargo.Ironium)
	require.Equal(t, before-f.Spec.CargoCapacity, source.Cargo.Ironium)
	require.Zero(t, f.Cargo.Colonists)
	// An idle shipyard still keeps stock for future ship production.
	reserve := manufacturingStockTarget(&u.Game.Rules, p, dest)
	require.GreaterOrEqual(t, reserve.Ironium, 300)
	require.GreaterOrEqual(t, reserve.Boranium, 300)
	require.GreaterOrEqual(t, reserve.Germanium, 300)
}
func TestStaleEmptyPickupIsReplannedInsteadOfHoldingTheSourceForever(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	saveLogisticsWork(f, &logisticsJob{Kind: "planet", TargetNum: source.Num, DeliveryNum: dest.Num, Position: source.Position, Load: Mineral{Germanium: 2}, Status: "Waiting empty at source for a full safe load (0/210 kT available)"})
	require.Nil(t, u.turn.currentLogisticsWork(p, f))
	require.Equal(t, "freighter", p.Automation.FleetRoles[f.Num])
}
func TestLandingTransportBeatsScoutAndOtherTargetsInPatrolPriority(t *testing.T) {
	p := &Player{Num: 1, PlanetIntels: []*Planet{{MapObject: MapObject{PlayerNum: 1, Position: Vector{}}}}}
	patrol := &Fleet{MapObject: MapObject{Position: Vector{}}}
	transport := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 1, Position: Vector{80, 0}}, WarpSpeed: 4, Heading: VectorFloat64{X: -1}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HullType: TechHullTypeFreighter}}}
	scout := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 2, Position: Vector{10, 0}}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HullType: TechHullTypeScout}}}
	p.FleetIntels = []*Fleet{scout, transport}
	require.Same(t, transport, sortedPatrolTargets(p, patrol)[0])
	require.Less(t, patrolOpportunityScore(p, patrol, transport), patrolOpportunityScore(p, patrol, scout))
}

// Exported snapshot of an independently copied 2483 database; no turn or save.
func TestPetra2483MineralPickupOnExportedCopy(t *testing.T) {
	source := os.Getenv("STARS_ECONOMY_SNAPSHOT")
	if source == "" {
		t.Skip("optional private economy snapshot")
	}
	absolute, err := filepath.Abs(source)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/")
	raw, err := os.ReadFile(absolute)
	require.NoError(t, err)
	var g FullGame
	require.NoError(t, json.Unmarshal(raw, &g))
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	tg := newTurnGenerator(&g)
	require.NoError(t, g.computeSpecs())
	tg.enablePlanningThreatIndex()
	defer tg.disablePlanningThreatIndex()
	p := g.getPlayer(1)
	f := g.getFleet(1, 32)
	require.NotNil(t, f)
	year := g.Year
	job := logisticsWork(f)
	require.NotNil(t, job)
	origin := g.getPlanet(job.TargetNum)
	dest := g.getPlanet(job.DeliveryNum)
	require.Equal(t, "Petra", origin.Name)
	require.Equal(t, 62, job.Load.Ironium)
	require.NotNil(t, tg.currentLogisticsWork(p, f), "a fillable delivery must survive rebuild-pending stale-job recovery")
	roles := p.Automation.FleetRoles
	p.Automation.FleetRoles = map[int]string{f.Num: "freighter"}
	tg.resolveLogisticsAutomation()
	tg.dispatchReadyLogistics()
	p.Automation.FleetRoles = roles
	require.Equal(t, 210, f.Cargo.Ironium, "MF32 must actually load the full cargo")
	require.Greater(t, len(f.Waypoints), 1, "MF32 must actually depart on its existing delivery before rebuilding")
	probe, stock := *f, *origin
	probe.Cargo = Cargo{}
	tg.topUpShipyardBuffer(p, &probe, &stock, dest)
	require.Equal(t, 210, probe.Cargo.Ironium)
	require.Equal(t, origin.Cargo.Ironium-210, stock.Cargo.Ironium)
	t.Logf("MF32 at Petra: old pickup %d kT, full cargo candidate %d/%d kT; source ironium %d", job.Load.Ironium, probe.Cargo.Ironium, probe.Spec.CargoCapacity, origin.Cargo.Ironium)
	require.Equal(t, year, g.Year)
}

func TestSeveralFullFreightersCanSupplyOneBlockedYard(t *testing.T) {
	scenario := maintenanceScenario()
	scenario.Players[0].Designs = append(scenario.Players[0].Designs, DesignTeamster)
	scenario.Players[0].Fleets = append(scenario.Players[0].Fleets, ScenarioFleet{Name: "Supplier A", Design: DesignTeamster.Name, At: "Yard"}, ScenarioFleet{Name: "Supplier B", Design: DesignTeamster.Name, At: "Yard"})
	second := scenario.Planets[0]
	second.Name = "Second yard"
	second.Position = Vector{10, 0}
	scenario.Planets = append(scenario.Planets, second)
	u := newTestUniverse(t, scenario)
	p, a, b, yard := u.Player(1), u.Fleet("Supplier A"), u.Fleet("Supplier B"), u.Planet("Second yard")
	p.DesignMaintenance = DesignMaintenance{p.Designs[0].Num: p.Designs[1].Num}
	p.Designs[1].Spec.Cost.Ironium = 2000
	writeMaintenance(u.Fleet("Fleet"), &fleetMaintenance{Group: "supply", Stage: "scrap", PlanetNum: yard.Num})
	yard.Cargo.Ironium = 0
	u.Planet("Yard").Cargo.Ironium = 10000
	p.Automation.FleetRoles = map[int]string{a.Num: "freighter", b.Num: "freighter"}
	p.Automation.DisablePassengerTransport = true
	p.PlanetIntels = []*Planet{u.Planet("Yard"), yard}
	first := u.turn.findShipyardSupply(p, a, map[string]bool{})
	require.NotNil(t, first)
	saveLogisticsWork(a, first)
	next := u.turn.findShipyardSupply(p, b, map[string]bool{"yard:2": true})
	require.NotNil(t, next)
	require.Equal(t, yard.Num, next.DeliveryNum)
	require.Equal(t, b.Spec.CargoCapacity, next.Load.Ironium)
	saveLogisticsWork(b, next)
	require.Equal(t, a.Spec.CargoCapacity+b.Spec.CargoCapacity, u.turn.pendingSupplyMinerals(p, yard.Num, false, 0).Ironium)
}
func TestIdleYardStockDoesNotPreemptPassengerService(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	p.Automation.DisableMineralSupply = false
	source.Cargo.Ironium = 10000
	dest.Spec.HasStarbase = true
	dest.Spec.DockCapacity = -1
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.NotNil(t, logisticsWork(f))
	require.Equal(t, "passengers", logisticsWork(f).Kind)
}
