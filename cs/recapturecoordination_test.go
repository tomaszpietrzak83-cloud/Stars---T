//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPriorityLandingAcceptsHabitableExplicitTarget(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, target := u.Player(1), u.Planet("Enemy")
	target.Hab = u.Planet("Home").Hab
	for grav := 0; grav <= 100; grav++ {
		target.Hab.Grav = grav
		target.BaseHab = target.Hab
		if q := prospectivePlanetQuality(p, target); q > 0 && q < 50 && terraformedPlanetQuality(p, target) <= 75 {
			break
		}
	}
	require.Greater(t, prospectivePlanetQuality(p, target), 0)
	require.Less(t, prospectivePlanetQuality(p, target), 50)
	require.LessOrEqual(t, terraformedPlanetQuality(p, target), 75, "this fixture tests the manual override for an ordinary-potential world")
	require.False(t, invasionColony(p, target))
	p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
	require.True(t, invasionColony(p, target), "an explicitly selected habitable colony must not retain the ordinary 50-percent cutoff")
}

func TestPendingBomberCanFinishSavedMission(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b, home, target := u.Player(1), u.Fleet("Bomber"), u.Planet("Home"), u.Planet("Enemy")
	home.Scanner = true
	home.Spec.ScanRange = 10000
	home.Spec.ScanRangePen = 10000
	saved := bomberOrders{Orders: FleetOrders{Waypoints: []Waypoint{{Position: b.Position}, {Position: target.Position, MapObjectTarget: target.ToTarget(), WarpSpeed: 5}}}}
	raw, _ := json.Marshal(saved)
	b.SetTag(bomberTag, string(raw))
	b.SetTag("maintenance-pending", "Finish current mission")
	u.Run((*turnGenerator).planBomberEscorts)
	require.Greater(t, len(b.Waypoints), 1)
	require.Equal(t, target.Position, b.Waypoints[1].Position)
	require.Equal(t, "bomber", p.Automation.FleetRoles[b.Num])
}

func TestPriorityReconRefreshesStaleUnownedReport(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Scout", Design: DesignLongRangeScout.Name, At: "Home"}}}, {}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Lost", Owner: 2, Position: Vector{40, 0}}}})
	p, f := u.Player(1), u.Fleet("Scout")
	intel := *u.Planet("Lost")
	intel.PlayerNum = 0
	intel.ReportAge = 10
	p.PlanetIntels = []*Planet{u.Planet("Home"), &intel}
	p.Automation.PriorityPlanets = map[int]bool{intel.Num: true}
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.planPriorityColonyRecon(p, f)) })
	require.Equal(t, 0, intel.PlayerNum, "planning must not reveal the hidden owner")
}

func TestBomberDoesNotDelegateToInsufficientOrStalledLanding(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, f, target := u.Player(1), u.Fleet("Cover"), u.Planet("Enemy")
	p.Automation.FleetRoles[f.Num] = "freighter"
	p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
	target.Cargo.Colonists = 200
	f.Cargo.Colonists = 100
	saveLogisticsWork(f, &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position, Status: "Waiting for enough escort strength"})
	require.False(t, bombingDelegatedToLanding(&u.Game.Rules, p, target, []*Fleet{f}), "the bomber cannot leave a 20k defender to a stalled 10k transport")
}

func TestLandingClearanceSurvivesReportAgeButNotNewBase(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, f, target := u.Player(1), u.Fleet("Cover"), u.Planet("Enemy")
	p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
	target.Cargo.Colonists = 400
	j := &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num}
	u.turn.invasionDestination(p, f, j, target)
	require.True(t, j.LandingCleared)
	require.False(t, j.BombingStage)
	target.ReportAge = 5
	u.turn.invasionDestination(p, f, j, target)
	require.False(t, j.BombingStage, "ordinary report ageing must not turn a loaded delivery back")
	require.Equal(t, target.Position, j.Position)
	target.ReportAge = 0
	target.Spec.HasStarbase = true
	u.turn.invasionDestination(p, f, j, target)
	require.True(t, j.BombingStage)
	require.False(t, j.LandingCleared)
	require.NotEqual(t, target.Position, j.Position)
}

func TestLandingClearanceDoesNotTransferAcrossTargetsOwnersOrMissions(t *testing.T) {
	for _, change := range []string{"target", "owner", "mission", "uncleared", "population", "defenses"} {
		t.Run(change, func(t *testing.T) {
			u := automaticBomberUniverse(t)
			p, f, target := u.Player(1), u.Fleet("Cover"), u.Planet("Enemy")
			p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
			target.Cargo.Colonists = 400
			j := &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num}
			u.turn.invasionDestination(p, f, j, target)
			target.ReportAge = 5
			switch change {
			case "target":
				j.LandingTarget++
			case "owner":
				j.LandingOwner++
			case "mission":
				j.Kind = "settle"
			case "uncleared":
				j.LandingCleared = false
			case "population":
				target.ReportAge = 0
				target.Cargo.Colonists = 1001
			case "defenses":
				target.ReportAge = 0
				target.Spec.DefenseCoverage = .1
			}
			u.turn.invasionDestination(p, f, j, target)
			require.False(t, j.LandingCleared)
			require.True(t, j.BombingStage)
		})
	}
}

func TestBomberHandoffBudgetsUnknownGroundDefenseBonus(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, f, target := u.Player(1), u.Fleet("Cover"), u.Planet("Enemy")
	p.Automation.PriorityPlanets = map[int]bool{target.Num: true}
	p.Automation.FleetRoles[f.Num] = "freighter"
	target.Cargo.Colonists = 1000
	f.Cargo.Colonists = 1505
	f.Position = target.Position
	saveLogisticsWork(f, &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position})
	require.GreaterOrEqual(t, reportedInvasionDefense(&u.Game.Rules), 2.0)
	require.False(t, bombingDelegatedToLanding(&u.Game.Rules, p, target, []*Fleet{f}), "150k invaders cannot certify victory over possible 200k defensive strength")
	before := u.turn.priorityLandingRequirement(p, target, 0)
	u.Player(2).Race.Spec.InvasionDefendBonus = 99
	require.Equal(t, before, u.turn.priorityLandingRequirement(p, target, 0), "private racial information must not affect the plan")
}
