//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func automaticBomberUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Bomber", Design: DesignTeamster.Name, At: "Home"}, {Name: "Cover", Design: DesignDestroyerDelta.Name, At: "Home"}}}, {}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Enemy", Owner: 2, Position: Vector{30, 0}, Cargo: Cargo{Colonists: 2001}}}})
	p := u.Player(1)
	p.PlanetIntels = []*Planet{u.Planet("Home"), u.Planet("Enemy")}
	for _, planet := range p.PlanetIntels {
		planet.ReportAge = 0
	}
	p.Automation.FleetRoles = map[int]string{u.Fleet("Bomber").Num: "bomber"}
	u.Fleet("Bomber").Spec.Bomber = true
	return u
}
func TestBomberScannerCoverageBudgetsStealthAndAWholeWarningTurn(t *testing.T) {
	scan := []scanner{{Position: Vector{}, Range: 1000, RangePen: 1000, CloakReductionFactor: 1}}
	require.False(t, bomberScannerCorridor(scan, Vector{}, Vector{50, 0}, 100, false), "98%% cloak defeats apparently broad coverage")
	scan[0].CloakReductionFactor = .5
	require.True(t, bomberScannerCorridor(scan, Vector{}, Vector{300, 0}, 100, false))
	require.False(t, bomberScannerCorridor(scan, Vector{}, Vector{450, 0}, 100, false), "reserve a turn before interception")
	scan[0].RangePen = 0
	require.False(t, bomberScannerCorridor(scan, Vector{300, 0}, Vector{300, 0}, 100, true), "orbit ambushes require penetrating coverage")
}
func TestAutomaticBomberChoosesEnemyWithoutEscortOnlyUnderAdequateCoverage(t *testing.T) {
	u := automaticBomberUniverse(t)
	home := u.Planet("Home")
	home.Scanner = true
	home.Spec.ScanRange = 10000
	home.Spec.ScanRangePen = 10000
	f := u.Fleet("Bomber")
	u.Run((*turnGenerator).planBomberEscorts)
	require.Greater(t, len(f.Waypoints), 1)
	require.Equal(t, u.Planet("Enemy").Position, f.Waypoints[len(f.Waypoints)-1].Position)
}
func TestBomberAttachesToProtectedMissionAndRestoresItsRole(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b, cover := u.Player(1), u.Fleet("Bomber"), u.Fleet("Cover")
	p.Automation.FleetRoles[cover.Num] = "combat"
	cover.Waypoints = []Waypoint{{Position: cover.Position}, NewPlanetWaypoint(u.Planet("Enemy").Position, u.Planet("Enemy").Num, u.Planet("Enemy").Name, 5)}
	u.Run((*turnGenerator).planBomberEscorts)
	require.Equal(t, "escort", p.Automation.FleetRoles[cover.Num])
	require.Equal(t, strconv.Itoa(b.Num), cover.GetTag(escortTag))
	require.Equal(t, b.Waypoints[1].Position, cover.Waypoints[1].Position)
	require.Equal(t, b.Waypoints[1].WarpSpeed, cover.Waypoints[1].WarpSpeed)
	u.Run(func(tg *turnGenerator) { tg.releaseEscorts(p, b) })
	require.Equal(t, "combat", p.Automation.FleetRoles[cover.Num])
	require.Empty(t, cover.GetTag(bomberBorrowTag))
}
func TestThreatenedBomberRequestsNearbyPatrolBeforeContact(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b, cover := u.Player(1), u.Fleet("Bomber"), u.Fleet("Cover")
	b.Position = Vector{30, 0}
	p.Automation.FleetRoles[cover.Num] = "patrol"
	p.FleetIntels = []*Fleet{{MapObject: MapObject{PlayerNum: 2, Num: 100, Position: Vector{200, 0}}, WarpSpeed: 10, Heading: VectorFloat64{-1, 0}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true, PowerRating: 1000}}}}
	require.Positive(t, bomberChaseThreat(p, b))
	u.Run((*turnGenerator).planAutomaticSafety)
	require.Equal(t, "escort", p.Automation.FleetRoles[cover.Num])
	require.Equal(t, strconv.Itoa(b.Num), cover.GetTag(escortTag))
	require.True(t, b.InAutomaticSafety())
	require.Greater(t, len(b.Waypoints), 1)
}

func TestBomberDoesNotInventWarpTenPursuitForDistantWarpFiveShip(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b := u.Player(1), u.Fleet("Bomber")
	p.FleetIntels = []*Fleet{{MapObject: MapObject{PlayerNum: 2, Num: 100, Position: Vector{130, 0}}, WarpSpeed: 5, Heading: VectorFloat64{X: -1}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true, PowerRating: 24}}}}
	require.Zero(t, bomberChaseThreat(p, b))
	p.FleetIntels[0].Position = Vector{40, 0}
	require.Positive(t, bomberChaseThreat(p, b))
	p.FleetIntels[0].WarpSpeed = 0
	p.FleetIntels[0].Heading = VectorFloat64{}
	require.Zero(t, bomberChaseThreat(p, b))
}
func TestStaleSmallPriorityColonyGetsBombardmentInsteadOfBeingIgnored(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b := u.Player(1), u.Fleet("Bomber")
	target := u.Planet("Enemy")
	p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
	target.Cargo.Colonists = 200
	target.ReportAge = 19
	require.True(t, priorityNeedsBombing(target))
	require.Same(t, target, priorityBombingTarget(p, b))
	target.ReportAge = 0
	require.False(t, priorityNeedsBombing(target))
	require.Same(t, target, priorityBombingTarget(p, b))
	transport := u.Fleet("Cover")
	p.Automation.FleetRoles[transport.Num] = "freighter"
	saveLogisticsWork(transport, &logisticsJob{Kind: "invade", DeliveryNum: target.Num})
	require.Nil(t, priorityBombingTarget(p, b, transport))
}
func TestCompletedBomberEvasionResumesMissionBeforePlanning(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b := u.Player(1), u.Fleet("Bomber")
	_ = p
	b.SetTag(safetyTag, `{"kind":"evade","orders":{"waypoints":[{"position":{"x":0,"y":0}}]}}`)
	u.Run((*turnGenerator).resumeCompletedEvades)
	require.False(t, b.InAutomaticSafety())
	u.Fleet("Cover").Spec.HasWeapons = true
	p.Automation.FleetRoles[u.Fleet("Cover").Num] = "escort"
	u.Run((*turnGenerator).planBomberEscorts)
	require.Greater(t, len(b.Waypoints), 1)
}
