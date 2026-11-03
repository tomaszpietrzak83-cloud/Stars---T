//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestFifteenDestroyersOnCopied2451Orders(t *testing.T) {
	path := os.Getenv("STARS_PATROL_ALLOCATION_SNAPSHOT")
	if path == "" {
		t.Skip("optional copied archive")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var game FullGame
	require.NoError(t, json.Unmarshal(raw, &game))
	game.TechStore = &StaticTechStore
	game.Rules.SetTechStore(&StaticTechStore)
	game.Universe.setLogger(testLogger)
	require.NoError(t, game.computeSpecs())
	p, f := game.getPlayer(1), game.getFleet(1, 66)
	require.Equal(t, 15, f.Spec.TotalShips)
	enemy := p.GetFleetIntel(3, 47)
	require.Equal(t, 3, reportedFleetShips(enemy))
	require.False(t, patrolNeedsRecon(p, enemy))
	require.False(t, patrolReportedCombatant(p, enemy))
	fuel := f.Fuel
	year := game.Year
	tg := newTurnGenerator(&game)
	tg.ordersPlayerNum = 1
	tg.advancePatrol(p, f)
	t.Logf("Fleet #66: %d destroyers retained for 3:47, original fuel %d, year unchanged %d", f.Spec.TotalShips, fuel, game.Year)
	require.Equal(t, 3, f.Spec.TotalShips)
	require.Equal(t, year, game.Year)
}

func TestUnknownSmallPatrolContactsRemainAttackable(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	enemy := patrolTestEnemy(0)
	enemy.Tokens = []ShipToken{{DesignNum: 999, Quantity: 3}}
	p.ShipDesignIntels = []*ShipDesign{{PlayerNum: 2, Num: 999, Hull: SmallFreighter.Name}}
	p.FleetIntels = []*Fleet{enemy}
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: enemy.Num})
	require.False(t, patrolNeedsRecon(p, enemy))
	require.True(t, patrolCanAttack(p, f))
	enemy.Tokens[0].Quantity = 7
	require.False(t, patrolNeedsRecon(p, enemy))
	enemy.Tokens[0].Quantity = 8
	require.True(t, patrolNeedsRecon(p, enemy))
	require.False(t, patrolCanAttack(p, f))
	enemy.Tokens[0].Quantity = 4
	other := *enemy
	other.Num++
	p.FleetIntels = append(p.FleetIntels, &other)
	require.True(t, patrolNeedsRecon(p, enemy), "two nearby unknown groups of four are one large encounter")
}

func TestOversizedPatrolReleasesSurplusForThreeWeakShips(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(map[bool]string{false: "same design", true: "mixed designs"}[mixed], func(t *testing.T) {
			tokens := []ScenarioShipToken{{Design: DesignStalwartDefender.Name, Quantity: 15}}
			if mixed {
				tokens = []ScenarioShipToken{{Design: DesignStalwartDefender.Name, Quantity: 6}, {Design: DesignDestroyerDelta.Name, Quantity: 9}}
			}
			u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignStalwartDefender, DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Pack", Tokens: tokens, At: "Home"}}}, {Designs: Designs(DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Weak", Design: DesignLongRangeScout.Name, Quantity: 3, Position: Vector{25, 0}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
			p, f, e := u.Player(1), u.Fleet("Pack"), u.Fleet("Weak")
			p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
			p.discoverer.discoverDesign(e.Tokens[0].design, true)
			p.discoverer.discoverFleet(e, true)
			addExpeditionTestDock(p, Vector{})
			savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: e.Num, RequiredShips: 3, Grouped: true})
			fuel := f.Fuel
			u.Run(func(tg *turnGenerator) { tg.advancePatrol(p, f) })
			require.Equal(t, 3, f.Spec.TotalShips)
			require.True(t, patrolCanAttack(p, f))
			require.Len(t, f.Waypoints, 2)
			ships, totalFuel, spares := 0, 0, 0
			for _, other := range u.Game.getFleets(1) {
				if !other.Delete {
					ships += other.Spec.TotalShips
					totalFuel += other.Fuel
					if other != f && !other.Starbase {
						spares += other.Spec.TotalShips
						require.Zero(t, readPatrolJob(other).Leader)
						require.Equal(t, "patrol", p.Automation.FleetRoles[other.Num])
					}
				}
			}
			require.Equal(t, 12, spares)
			require.Equal(t, 15, ships)
			require.Equal(t, fuel, totalFuel)
		})
	}
}

func TestRadiatingPatrolCanBurstAboveFreeCruiseWarp(t *testing.T) {
	d := DesignDestroyerDelta
	d.Slots = append([]ShipDesignSlot(nil), d.Slots...)
	d.Slots[0].HullComponent = RadiatingHydroRamScoop.Name
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(d, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Hunter", Design: d.Name, At: "Home"}}}, {Designs: Designs(DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Target", Design: DesignLongRangeScout.Name, Position: Vector{55, 0}}}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p, f, e := u.Player(1), u.Fleet("Hunter"), u.Fleet("Target")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	p.discoverer.discoverDesign(e.Tokens[0].design, true)
	p.discoverer.discoverFleet(e, true)
	addExpeditionTestDock(p, Vector{})
	require.Equal(t, 6, economicalContainmentPolicy(p, f).MinWarp)
	u.Run((*turnGenerator).planPatrolAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	require.True(t, patrolCanAttack(p, f))
	require.Len(t, f.Waypoints, 2)
	require.Greater(t, f.Waypoints[1].WarpSpeed, 6, "funded pursuit can leave free cruise speed")
}
