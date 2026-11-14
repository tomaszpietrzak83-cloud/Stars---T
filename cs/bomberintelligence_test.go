//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
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

func TestAutomaticBomberLeavesBlockedPriorityForReachableColony(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, bomber := u.Player(1), u.Fleet("Bomber")
	home, priority := u.Planet("Home"), u.Planet("Enemy")
	home.Scanner = true
	home.Spec.ScanRange = 10000
	home.Spec.ScanRangePen = 10000
	p.Automation.PriorityPlanets = map[int]bool{priority.Num: true}
	priority.Spec.HasStarbase = true

	mission := bomberOrders{Orders: bomber.FleetOrders}
	mission.Orders.Waypoints = []Waypoint{{Position: bomber.Position}, NewPlanetWaypoint(priority.Position, priority.Num, priority.Name, 6)}
	raw, err := json.Marshal(mission)
	require.NoError(t, err)
	bomber.SetTag(bomberTag, string(raw))
	rally, err := json.Marshal(bombingRally{Leader: u.Fleet("Cover").Num, Target: priority.Num, Until: u.Game.Year + 1})
	require.NoError(t, err)
	bomber.SetTag(bombingRallyTag, string(rally))

	u.Run((*turnGenerator).planBomberEscorts)
	require.Empty(t, bomber.GetTag(bomberTag), "an old mission must not bring back an orbital-base target")
	require.Empty(t, bomber.GetTag(bombingRallyTag))
	require.Len(t, bomber.Waypoints, 1)

	alternative := *priority
	alternative.Num++
	alternative.Name = "Alternative"
	alternative.Position = Vector{40, 0}
	alternative.Spec.HasStarbase = false
	p.PlanetIntels = append(p.PlanetIntels, &alternative)
	bomber.SetTag(bomberTag, string(raw))

	u.Run((*turnGenerator).planBomberEscorts)
	require.Equal(t, alternative.Num, bomberGoal(bomber))
	require.Greater(t, len(bomber.Waypoints), 1)
	require.Equal(t, alternative.Position, bomber.Waypoints[len(bomber.Waypoints)-1].Position)

	priority.Spec.HasStarbase = false
	u.Run((*turnGenerator).planBomberEscorts)
	require.Equal(t, priority.Num, bomberGoal(bomber), "an eligible priority colony takes precedence again")
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

func TestBomberWaitsForFreshOrbitalReportWithoutBorrowingPatrol(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b, cover := u.Player(1), u.Fleet("Bomber"), u.Fleet("Cover")
	target := u.Planet("Enemy")
	target.ReportAge = 2
	p.Automation.FleetRoles[cover.Num] = "patrol"
	u.Run((*turnGenerator).planBomberEscorts, (*turnGenerator).planBombingWolfPacks)
	require.Equal(t, "recon", readBomberOrders(b).Stage)
	require.Equal(t, target.Num, bomberGoal(b))
	require.Len(t, b.Waypoints, 1)
	require.Equal(t, "patrol", p.Automation.FleetRoles[cover.Num])
	require.False(t, cover.Delete)
}

func TestPercentageBombingHandsOffAtOnePercentOnlyWithEnoughLandingTroops(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, bomber, transport := u.Player(1), u.Fleet("Bomber"), u.Fleet("Cover")
	target := u.Planet("Enemy")
	target.Hab = u.Planet("Home").Hab
	target.Spec.MaxPopulation = 12000000
	target.Cargo.Colonists = 1100
	p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
	p.Automation.FleetRoles[transport.Num] = "freighter"
	bomber.Spec.SmartBombs = []Bomb{{KillRate: 10, Quantity: 1}}
	transport.Waypoints = []Waypoint{{Position: transport.Position}, NewPlanetWaypoint(target.Position, target.Num, target.Name, 6)}
	saveLogisticsWork(transport, &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position})
	require.True(t, priorityNeedsBombing(target), "above the ordinary population cutoff")
	require.True(t, percentageBombingThreshold(bomber, target))
	transport.Cargo.Colonists = 250
	require.False(t, bomberHandsOffToLanding(&u.Game.Rules, p, bomber, target, []*Fleet{transport}))
	transport.Cargo.Colonists = 10000
	require.True(t, bomberHandsOffToLanding(&u.Game.Rules, p, bomber, target, []*Fleet{transport}))
	bomber.Spec.Bombs = []Bomb{{MinKillRate: 100, Quantity: 1}}
	require.False(t, bomberHandsOffToLanding(&u.Game.Rules, p, bomber, target, []*Fleet{transport}))
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
	require.Same(t, target, priorityBombingTarget(&u.Game.Rules, p, b))
	target.ReportAge = 0
	require.False(t, priorityNeedsBombing(target))
	require.Same(t, target, priorityBombingTarget(&u.Game.Rules, p, b))
	transport := u.Fleet("Cover")
	p.Automation.FleetRoles[transport.Num] = "freighter"
	transport.Cargo.Colonists = 500
	transport.Waypoints = []Waypoint{{Position: transport.Position}, {Position: target.Position, MapObjectTarget: target.ToTarget(), WarpSpeed: 6}}
	saveLogisticsWork(transport, &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position})
	require.Nil(t, priorityBombingTarget(&u.Game.Rules, p, b, transport))
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
