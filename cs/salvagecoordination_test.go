//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func battlePickupUniverse(t *testing.T) (*testUniverse, *Player, *Fleet, *Fleet, *Fleet) {
	u := logisticsUniverse(t)
	p, hunter, transport := u.Player(1), u.Fleet("Escort"), u.Fleet("Transport")
	p.SalvageIntels = nil
	u.Fleet("Combat").Position = Vector{500, 0}
	p.Automation.FleetRoles[hunter.Num] = "patrol"
	addExpeditionTestDock(p, Vector{})
	enemy := patrolTestEnemy(1)
	enemy.Position = Vector{25, 0}
	p.FleetIntels = []*Fleet{enemy}
	savePatrolJob(hunter, patrolJob{Leader: hunter.Num, Enemy: enemy.PlayerNum, Target: enemy.Num})
	u.Run(func(tg *turnGenerator) { require.True(t, tg.patrolRoute(p, hunter, enemy.Position, true)) })
	return u, p, hunter, transport, enemy
}

func TestTransportArrivesWithPatrolAndCollectsBeforeNextDecay(t *testing.T) {
	u, p, hunter, f, enemy := battlePickupUniverse(t)
	u.Run((*turnGenerator).planPatrolSalvage)
	require.NotEmpty(t, f.GetTag(anticipatedSalvageTag))
	u.Run((*turnGenerator).fleetMove)
	require.Equal(t, enemy.Position, hunter.Position)
	require.Equal(t, hunter.Position, f.Position, "collector must reach the battle in the same movement phase")
	// A small victorious battle produces less than the minimum annual decay.
	// Its collector is already here and can load at wp0 of the next turn.
	salvage := u.Game.getOrCreateSalvage(enemy.Position, p.Num, Cargo{Ironium: 5, Germanium: 1})
	p.SalvageIntels = nil
	f.Spec.Scanner = false
	hunter.Spec.Scanner = false
	scan := newPlayerScanner(u.Game.Universe, u.Game.Players, &u.Game.Rules, p)
	scan.scanSalvages(nil)
	require.NotNil(t, p.GetSalvageIntel(salvage.Num), "physical contact must work without scanner equipment")
	require.Nil(t, p.GetSalvageIntel(u.Game.Salvages[0].Num), "distant unseen debris stays unknown")
	p.FleetIntels = nil
	u.Game.Year++
	u.Run((*turnGenerator).planPatrolSalvage, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).decaySalvage)
	require.Equal(t, 5, f.Cargo.Ironium)
	require.Equal(t, 1, f.Cargo.Germanium)
	require.True(t, salvage.Delete)
}

func TestCollectorFollowsChangedBattleLocationAndReleasesCancelledBattle(t *testing.T) {
	u, p, hunter, f, enemy := battlePickupUniverse(t)
	u.Run((*turnGenerator).planPatrolSalvage)
	enemy.Position = Vector{30, 0}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.patrolRoute(p, hunter, enemy.Position, true)) }, (*turnGenerator).planPatrolSalvage)
	var pending anticipatedSalvage
	require.NoError(t, json.Unmarshal([]byte(f.GetTag(anticipatedSalvageTag)), &pending))
	require.Equal(t, enemy.Position, pending.Position)
	require.Equal(t, enemy.Position, f.Waypoints[len(f.Waypoints)-1].Position)
	p.FleetIntels = nil
	u.Run((*turnGenerator).planPatrolSalvage)
	require.Empty(t, f.GetTag(anticipatedSalvageTag))
}

func TestRealDebrisPreemptsSpeculativeBattleButNotDelivery(t *testing.T) {
	u, p, _, f, _ := battlePickupUniverse(t)
	u.Run((*turnGenerator).planPatrolSalvage)
	require.NotEmpty(t, f.GetTag(anticipatedSalvageTag))
	debris := *u.Game.Salvages[0]
	debris.Position = Vector{-50, 0}
	p.SalvageIntels = []*Salvage{&debris}
	u.Run((*turnGenerator).planPatrolSalvage)
	require.Empty(t, f.GetTag(anticipatedSalvageTag))
	require.Equal(t, "salvage", logisticsWork(f).Kind)
	saveLogisticsWork(f, &logisticsJob{Kind: "settle", TargetNum: 1, Position: Vector{99, 0}})
	f.Cargo.Colonists = 10
	u.Run((*turnGenerator).planPatrolSalvage)
	require.Equal(t, "settle", logisticsWork(f).Kind)
}

func TestCollectorTooFarToReachBattleIsNotReserved(t *testing.T) {
	u, _, _, f, _ := battlePickupUniverse(t)
	f.Position = Vector{1000, 0}
	u.Run((*turnGenerator).planPatrolSalvage)
	require.Empty(t, f.GetTag(anticipatedSalvageTag))
}

func TestNativeBattleCollectorAndOneShotPriority(t *testing.T) {
	s := TestScenario{
		Players: []ScenarioPlayer{
			{Designs: Designs(DesignStalwartDefender, DesignTeamster, DesignSpaceStation),
				Fleets: []ScenarioFleet{{Name: "Hunter", Design: DesignStalwartDefender.Name, At: "Home"}, {Name: "Collector", Design: DesignTeamster.Name, At: "Home"}}},
			{Designs: Designs(DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Victim", Design: DesignLongRangeScout.Name, Position: Vector{25, 0}}}},
		},
		Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500}}},
	}
	u := newTestUniverse(t, s)
	p, hunter, f, enemy := u.Player(1), u.Fleet("Hunter"), u.Fleet("Collector"), u.Fleet("Victim")
	p.Automation.FleetRoles = map[int]string{hunter.Num: "patrol", f.Num: "freighter"}
	p.Automation.PriorityFleets = map[string]bool{priorityFleetKey(enemy.PlayerNum, enemy.Num): true}
	p.discoverer.discoverDesign(enemy.Tokens[0].design, true)
	p.discoverer.discoverFleet(enemy, true)
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planPatrolAutomation, (*turnGenerator).planPatrolSalvage)
	require.NotEmpty(t, f.GetTag(anticipatedSalvageTag))
	u.Run((*turnGenerator).guardAutomaticFuel, (*turnGenerator).planAutomaticSafety, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetBattle)
	require.Equal(t, hunter.Position, f.Position)
	require.True(t, enemy.Delete, "the native battle must actually destroy the marked target")
	require.Empty(t, p.Automation.PriorityFleets)
	require.NotEmpty(t, u.Game.Salvages)
	salvage := u.Game.Salvages[0]
	before := salvage.Cargo.ToMineral().Total()
	require.Positive(t, before)
	p.SalvageIntels = nil
	scan := newPlayerScanner(u.Game.Universe, u.Game.Players, &u.Game.Rules, p)
	scan.scanSalvages(nil)
	p.FleetIntels = nil
	u.Game.Year++
	u.Run((*turnGenerator).planPatrolSalvage, (*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).decaySalvage)
	require.Equal(t, before, f.Cargo.ToMineral().Total(), "collector loads the real battle debris before decay, without instant battle salvage enabled")
}
