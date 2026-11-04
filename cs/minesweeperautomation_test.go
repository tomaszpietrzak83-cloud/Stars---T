//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func testSweeperDesign() ShipDesign {
	return ShipDesign{Name: "Equipment decides the role", Hull: Destroyer.Name, Slots: []ShipDesignSlot{{HullSlotIndex: 1, HullComponent: FuelMizer.Name, Quantity: 1}, {HullSlotIndex: 2, HullComponent: GatlingGun.Name, Quantity: 1}, {HullSlotIndex: 3, HullComponent: GatlingGun.Name, Quantity: 1}}}
}

func minesweeperUniverse(t *testing.T) *testUniverse {
	d := testSweeperDesign()
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(d, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Sweeper", Design: d.Name}}},
		{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Enemy", Design: DesignDestroyerDelta.Name, Position: Vector{500, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	u.Player(1).Automation.FleetRoles = map[int]string{u.Fleet("Sweeper").Num: "minesweeper"}
	return u
}

func TestGatlingOnlyMinesweeperClassificationAndConstruction(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f := u.Player(1), u.Fleet("Sweeper")
	d := p.GetDesign(f.Tokens[0].DesignNum)
	require.True(t, dedicatedMinesweeperSpec(d.Spec))
	require.Equal(t, "minesweeper", effectiveDesignAutoRole(d.Spec))
	built, err := u.turn.buildFleet(p, u.Planet("Home"), ShipToken{DesignNum: d.Num, Quantity: 2, design: d}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "minesweeper", p.Automation.FleetRoles[built.Num])
	for _, role := range []string{"patrol", "escort", "combat"} {
		require.Error(t, ValidateDesignAutoRole(role, d.Spec))
		require.Error(t, p.ValidateLogisticsAutomation(LogisticsAutomation{FleetRoles: map[int]string{f.Num: role}}, u.Game.getFleets(1)))
	}
	require.NoError(t, ValidateDesignAutoRole("minesweeper", d.Spec))
	require.NoError(t, ValidateDesignAutoRole("manual", d.Spec))
	mixed := d.Spec
	mixed.WeaponSlots = append(append([]ShipDesignSlot(nil), d.Spec.WeaponSlots...), ShipDesignSlot{HullComponent: XRayLaser.Name, Quantity: 1})
	require.False(t, dedicatedMinesweeperSpec(mixed))
	gunless := d.Spec
	gunless.WeaponSlots = nil
	require.False(t, dedicatedMinesweeperSpec(gunless))
}

func TestMinesweeperCannotBeBorrowedForCombatAndRestoresManualPlan(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f := u.Player(1), u.Fleet("Sweeper")
	p.Automation.FleetRoles[f.Num] = "patrol"
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: u.Fleet("Enemy").Num})
	require.False(t, patrolAvailable(p, f))
	require.False(t, combatAutonomy(p, f))
	require.False(t, patrolCanAttack(p, f))
	u.Run(func(tg *turnGenerator) { tg.assignBomberProtection(p, u.Fleet("Enemy"), Vector{}, true) })
	require.NotEqual(t, "escort", p.Automation.FleetRoles[f.Num])
	u.Run((*turnGenerator).prepareDedicatedMinesweepers)
	require.Equal(t, "minesweeper", p.Automation.FleetRoles[f.Num])
	require.Zero(t, readPatrolJob(f).Leader)
	old := f.BattlePlanNum
	setMinesweeperBattlePlan(p, f)
	require.Equal(t, BattleTacticDisengage, patrolPlanForFleet(p, f).Tactic)
	p.TakeManualControl(f)
	require.Equal(t, old, f.BattlePlanNum)
	require.Empty(t, f.GetTag(minesweeperBattlePlanTag))
}

func TestMinesweeperClearsRealRemoteFieldWithSafeEntryAndReload(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f := u.Player(1), u.Fleet("Sweeper")
	field := newMinefield(u.Player(2), MinefieldTypeStandard, 100, 1, Vector{140, 0})
	u.Game.Universe.addMinefield(field)
	report := *field
	p.MinefieldIntels = []*Minefield{&report}
	u.Run((*turnGenerator).planMinesweeperAutomation)
	require.GreaterOrEqual(t, len(f.Waypoints), 3, "cruise and safe mine-entry legs are distinct")
	safe := u.Game.Rules.MinefieldStatsByType[field.MinefieldType].MaxSpeed + p.Race.Spec.MinefieldSafeWarpBonus
	require.Greater(t, f.Waypoints[1].WarpSpeed, safe)
	require.LessOrEqual(t, f.Waypoints[len(f.Waypoints)-1].WarpSpeed, safe)
	for step := 0; step < 16 && !field.Delete && field.NumMines > 0; step++ {
		u.Run((*turnGenerator).planMinesweeperAutomation, (*turnGenerator).planAutomaticSafety, (*turnGenerator).recoverIdleAutomaticFleets, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetSweepMines)
		t.Logf("SWEEP step=%d position=%v mines=%d role=%s", step, f.Position, field.NumMines, p.Automation.FleetRoles[f.Num])
		if step == 2 {
			raw, err := json.Marshal(u.Game)
			require.NoError(t, err)
			var restored FullGame
			require.NoError(t, json.Unmarshal(raw, &restored))
			restored.TechStore = &StaticTechStore
			restored.Rules.SetTechStore(&StaticTechStore)
			restored.Universe.setLogger(testLogger)
			require.NoError(t, restored.computeSpecs())
			u.Game = &restored
			tg := newTurnGenerator(u.Game)
			u.turn = &tg
			p = u.Player(1)
			f = u.Fleet("Sweeper")
			field = u.Game.Minefields[0]
		}
	}
	require.True(t, field.Delete || field.NumMines == 0, "native mine-sweep phase must actually clear the field")
	require.Equal(t, "minesweeper", p.Automation.FleetRoles[f.Num])
	require.GreaterOrEqual(t, f.Fuel, 0)
}

func TestMinesweeperRetreatsEvenWithAggressiveFleetPlan(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f := u.Player(1), u.Fleet("Sweeper")
	d := p.GetDesign(f.Tokens[0].DesignNum)
	plan := designBattlePlan(&u.Game.Rules, p, d, BattlePlan{Num: 88, Tactic: BattleTacticMaximizeDamage, PrimaryTarget: BattleTargetAny, SecondaryTarget: BattleTargetAny})
	require.Equal(t, BattleTacticDisengage, plan.Tactic)
	require.Equal(t, BattleTargetNone, plan.PrimaryTarget)
	require.Equal(t, BattleTargetNone, plan.SecondaryTarget)
	enemyModel := &Player{Num: 2, AIControlled: true}
	plan = designBattlePlan(&u.Game.Rules, enemyModel, d, BattlePlan{Num: 88, Tactic: BattleTacticMaximizeDamage})
	require.Equal(t, BattleTacticMaximizeDamage, plan.Tactic, "do not infer an enemy's private automation from Gatling equipment")
}

func TestMinesweeperDoesNotStartBattleOrFireWhenIntercepted(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f, e := u.Player(1), u.Fleet("Sweeper"), u.Fleet("Enemy")
	p.Relations = []PlayerRelationship{{Relation: PlayerRelationFriend}, {Relation: PlayerRelationEnemy}}
	u.Player(2).Relations = []PlayerRelationship{{Relation: PlayerRelationEnemy}, {Relation: PlayerRelationFriend}}
	aggressive := BattlePlan{Num: 88, PrimaryTarget: BattleTargetAny, SecondaryTarget: BattleTargetAny, Tactic: BattleTacticMaximizeDamage, AttackWho: BattleAttackWhoEnemies}
	f.battlePlan = &aggressive
	players := map[int]*Player{1: p, 2: u.Player(2)}
	_, starts := getBattleAttackOrders(players, []*Fleet{f})
	require.False(t, starts, "Gatling-only equipment cannot initiate combat even before its first automation pass")
	e.battlePlan = &aggressive
	e.Position = f.Position
	u.Game.Rules.ResetSeed(17)
	b := newBattler(testLogger, &u.Game.Rules, 1, players, []*Fleet{f, e}, nil)
	require.NotNil(t, b)
	record := b.runBattle()
	own := map[int]bool{}
	for _, token := range record.Tokens {
		if token.PlayerNum == p.Num {
			own[token.Num] = true
			require.Equal(t, BattleTacticDisengage, token.Tactic)
			require.Equal(t, BattleTargetNone, token.PrimaryTarget)
		}
	}
	require.NotEmpty(t, own)
	for _, round := range record.ActionsPerRound {
		for _, action := range round {
			if own[action.TokenNum] {
				require.NotEqual(t, TokenActionBeamFire, action.Type)
				require.NotEqual(t, TokenActionTorpedoFire, action.Type)
			}
		}
	}
}
