//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func patrolTestEnemy(power int, knownTo ...*Player) *Fleet {
	f := &Fleet{MapObject: MapObject{Num: 91, PlayerNum: 2, Name: "Enemy scout", Position: Vector{30, 0}}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: power > 0, PowerRating: power}}}
	if len(knownTo) > 0 {
		p := knownTo[0]
		num := 10000 + len(p.ShipDesignIntels)
		d := &ShipDesign{PlayerNum: 2, Num: num, Hull: Scout.Name, Slots: []ShipDesignSlot{{HullSlotIndex: 1, HullComponent: RhinoScanner.Name, Quantity: 1}}, Spec: f.Spec.ShipDesignSpec}
		if power > 0 {
			d.Hull = Destroyer.Name
		}
		p.ShipDesignIntels = append(p.ShipDesignIntels, d)
		p.shipIntelIndex = nil
		f.Tokens = []ShipToken{{DesignNum: num, Quantity: 1}}
	}
	return f
}
func TestPatrolInterceptsWeakVisibleTargetWithReturnFuel(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	p.FleetIntels = []*Fleet{patrolTestEnemy(0, p)}
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planPatrolAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).planAutomaticSafety, (*turnGenerator).enforceAutomaticDeparture)
	require.True(t, patrolCanAttack(p, f))
	require.Len(t, f.Waypoints, 2)
	require.Equal(t, Vector{30, 0}, f.Waypoints[1].Position)
	require.False(t, f.InAutomaticSafety())
	require.NotNil(t, planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, Vector{30, 0}, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true}))
}
func TestPatrolGroupsThenSplitsSurvivorsAfterContactLost(t *testing.T) {
	u := logisticsUniverse(t)
	p, leader, follower := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	follower.Position = leader.Position
	p.Automation.FleetRoles = map[int]string{leader.Num: "patrol", follower.Num: "patrol"}
	p.FleetIntels = []*Fleet{patrolTestEnemy(leader.Spec.PowerRating, p)}
	addExpeditionTestDock(p, Vector{})
	ships := leader.Spec.TotalShips + follower.Spec.TotalShips
	fuel := leader.Fuel + follower.Fuel
	u.Run((*turnGenerator).planPatrolAutomation)
	require.True(t, follower.Delete)
	require.Equal(t, ships, leader.Spec.TotalShips)
	require.True(t, readPatrolJob(leader).Grouped)
	require.True(t, patrolCanAttack(p, leader))
	p.FleetIntels = nil
	u.Run((*turnGenerator).planPatrolAutomation)
	remaining, totalFuel := 0, 0
	for _, f := range u.Game.getFleets(p.Num) {
		if !f.Delete && p.Automation.FleetRoles[f.Num] == "patrol" {
			require.Equal(t, 1, f.Spec.TotalShips)
			require.Zero(t, readPatrolJob(f).Leader)
			remaining += f.Spec.TotalShips
			totalFuel += f.Fuel
		}
	}
	require.Equal(t, ships, remaining)
	require.Equal(t, fuel, totalFuel)
}
func TestPatrolRejectsOverwhelmingStaleAndUnknownArmedTargets(t *testing.T) {
	for _, kind := range []string{"strong", "stale", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			u := logisticsUniverse(t)
			p, f := u.Player(1), u.Fleet("Escort")
			p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
			enemy := patrolTestEnemy(f.Spec.PowerRating*10, p)
			if kind == "stale" {
				enemy.ReportAge = 1
			}
			if kind == "unknown" {
				enemy.Spec.PowerRating = 0
			}
			p.FleetIntels = []*Fleet{enemy}
			u.Run((*turnGenerator).planPatrolAutomation)
			require.Zero(t, readPatrolJob(f).Leader)
			require.False(t, patrolCanAttack(p, f))
			require.Equal(t, "patrol", p.Automation.FleetRoles[f.Num])
		})
	}
}
func TestPatrolDoesNotSendDamagedShipsIntoCombat(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	p.FleetIntels = []*Fleet{patrolTestEnemy(0, p)}
	f.Tokens[0].QuantityDamaged = 1
	f.Tokens[0].Damage = float64(f.Tokens[0].design.Spec.Armor) / 2
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Zero(t, readPatrolJob(f).Leader)
	require.Equal(t, "patrol", p.Automation.FleetRoles[f.Num])
}

func TestPatrolDoesNotOverwriteMaintenanceRoute(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: 91})
	writeMaintenance(f, &fleetMaintenance{OriginalRole: "patrol"})
	f.Waypoints = append(f.Waypoints, NewPositionWaypoint(Vector{45, 0}, 5))
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Len(t, f.Waypoints, 2)
	require.Equal(t, Vector{45, 0}, f.Waypoints[1].Position)
	require.Zero(t, readPatrolJob(f).Leader)
}

func TestPatrolUsesReportedDesignsRatherThanPrivateEnemyPointers(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	enemy := patrolTestEnemy(0, p)
	enemy.Tokens = []ShipToken{{DesignNum: 10, Quantity: 1, design: f.Tokens[0].design}}
	require.Positive(t, patrolReportedPower(p, enemy), "unidentified hulls retain a finite conservative estimate")
	require.Less(t, patrolReportedPower(p, enemy), 1000000, "missing equipment is not an infinitely powerful enemy")
	p.ShipDesignIntels = []*ShipDesign{{PlayerNum: 2, Num: 10, Slots: []ShipDesignSlot{{Quantity: 1}}, Spec: ShipDesignSpec{PowerRating: 75}}}
	require.Equal(t, 75, patrolReportedPower(p, enemy))
}

func TestPatrolLightScoutReportsDoNotOutrankDestroyers(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	f.Spec.PowerRating = 10
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	enemy := patrolTestEnemy(0, p)
	enemy.Position = Vector{8, 0}
	enemy.Tokens = []ShipToken{{DesignNum: 10, Quantity: 1}}
	second := *enemy
	second.Num++
	second.Position = Vector{20, 0}
	p.FleetIntels = []*Fleet{enemy, &second}
	p.ShipDesignIntels = []*ShipDesign{{PlayerNum: 2, Num: 10, Hull: Scout.Name, Spec: ShipDesignSpec{Mass: 17, ShipClass: ShipClassScout}}}
	addExpeditionTestDock(p, Vector{})
	require.Equal(t, 2, patrolReportedPower(p, enemy))
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Zero(t, readPatrolJob(f).Leader, "a hull-only scout report still needs equipment reconnaissance")
	p.ShipDesignIntels[0].Slots = []ShipDesignSlot{{Quantity: 1}}
	p.ShipDesignIntels[0].Spec.PowerRating = 2
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Equal(t, enemy.Num, readPatrolJob(f).Target)
	require.True(t, patrolCanAttack(p, f))
}

func TestClosestPatrolGetsNearbyScoutInsteadOfDistantLowNumberFleet(t *testing.T) {
	u := logisticsUniverse(t)
	p, far, near := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	far.Position = Vector{}
	near.Position = Vector{45, 0}
	p.Automation.FleetRoles = map[int]string{far.Num: "patrol", near.Num: "patrol"}
	enemy := patrolTestEnemy(0, p)
	enemy.Position = Vector{50, 0}
	p.FleetIntels = []*Fleet{enemy}
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Equal(t, enemy.Num, readPatrolJob(near).Target)
	require.Zero(t, readPatrolJob(far).Target)
}

func TestPatrolUsesNativeInterceptionWhenScoutStopsBeforePredictedPoint(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Patrol", Design: DesignDestroyerDelta.Name, At: "Home"}}},
		{Designs: Designs(DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Scout", Design: DesignLongRangeScout.Name, Position: Vector{10, 0}, Waypoints: []ScenarioWaypoint{{To: "Stop", Warp: 5}}}}},
	},
		Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Stop", Position: Vector{20, 0}}}})
	p, f, enemy := u.Player(1), u.Fleet("Patrol"), u.Fleet("Scout")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	enemy.Waypoints = []Waypoint{{Position: enemy.Position}, NewPositionWaypoint(Vector{20, 0}, 5)}
	intel := *enemy
	intel.Tags = nil
	intel.Heading = VectorFloat64{X: 1}
	intel.WarpSpeed = 5
	p.FleetIntels = []*Fleet{&intel}
	knownDesign := *enemy.Tokens[0].design
	p.ShipDesignIntels = []*ShipDesign{&knownDesign}
	p.ShipDesignIntels[0].PlayerNum = 2
	p.ShipDesignIntels[0].Num = enemy.Tokens[0].DesignNum
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planPatrolAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).preparePatrolPursuits, (*turnGenerator).enforceAutomaticDeparture)
	require.Len(t, f.Waypoints, 2)
	require.Equal(t, MapObjectTypeFleet, f.Waypoints[1].TargetType)
	u.Run((*turnGenerator).fleetMove)
	require.Equal(t, Vector{20, 0}, enemy.Position)
	require.Equal(t, enemy.Position, f.Position, "interception must follow the target's real arrival, not overshoot to the predicted position")
	require.GreaterOrEqual(t, f.Fuel, 0)
}

func TestPatrolReservesFuelForCruiseReturnInsteadOfFreeWarpOne(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	addExpeditionTestDock(p, Vector{})
	position := Vector{60, 0}
	reserve := automaticReturnFuel(&u.Game.Rules, p, f, position, expeditionStations(p, f), true)
	require.Greater(t, reserve, 0)
	route := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, position, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true})
	require.NotNil(t, route)
	require.GreaterOrEqual(t, route.ArrivalFuel[0], reserve)
	virtual := *f
	virtual.Position = position
	virtual.Fuel = route.ArrivalFuel[0]
	back := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{&virtual}, Vector{}, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true})
	require.NotNil(t, back)
	require.LessOrEqual(t, back.Turns, 8)
	for _, wp := range back.Waypoints[1:] {
		if virtual.Position.DistanceTo(wp.Position) > virtual.StrategicSpeed(wp.WarpSpeed) {
			require.GreaterOrEqual(t, wp.WarpSpeed, automaticCruiseWarp(p, &virtual))
		}
	}
}

func TestPatrolDoesNotFollowChangedTargetBeyondFundedRange(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	addExpeditionTestDock(p, Vector{})
	f.Waypoints = append(f.Waypoints, NewPositionWaypoint(Vector{10, 0}, 6))
	u.Run(func(tg *turnGenerator) {
		require.False(t, tg.patrolPursuitSafe(f, Vector{1000, 0}, 36))
		f.Fuel = 0
		require.False(t, tg.patrolPursuitSafe(f, Vector{30, 0}, 36))
	})
}

func TestFullTurnPatrolUsesThePlayersVisibleReports(t *testing.T) {
	u := newTestUniverse(t, TestScenario{
		Players: []ScenarioPlayer{
			{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Patrol", Design: DesignDestroyerDelta.Name, At: "Home"}}},
			{Designs: Designs(DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Scout", Design: DesignLongRangeScout.Name, Position: Vector{10, 0}}}},
		},
		Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500}}, {Name: "Enemy home", Owner: 2, Position: Vector{500, 0}, Cargo: Cargo{Colonists: 2500}}},
	})
	p, f, enemy := u.Player(1), u.Fleet("Patrol"), u.Fleet("Scout")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	known := *enemy.Tokens[0].design
	p.ShipDesignIntels = []*ShipDesign{&known}
	report := *enemy
	p.FleetIntels = []*Fleet{&report}
	require.NoError(t, u.turn.generateTurn())
	require.Contains(t, f.GetTag(patrolTag), "Intercepting", "the full turn must not erase visible fleet reports before automatic orders run")
	require.Equal(t, "patrol", p.Automation.FleetRoles[f.Num])
}

func TestPatrolFollowersMergeBeforeBattleAtIntermediateRendezvous(t *testing.T) {
	u := logisticsUniverse(t)
	p, a, b := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol"}
	a.Position = Vector{10, 0}
	b.Position = a.Position
	savePatrolJob(a, patrolJob{Leader: 999, Enemy: 2, Target: 91})
	savePatrolJob(b, patrolJob{Leader: 999, Enemy: 2, Target: 91})
	total := a.Spec.TotalShips + b.Spec.TotalShips
	u.Run((*turnGenerator).mergePatrolRendezvous)
	require.True(t, b.Delete)
	require.Equal(t, total, a.Spec.TotalShips)
	require.True(t, readPatrolJob(a).Grouped)
}
func TestPatrolMobilizesForDistantIncomingCombatFleet(t *testing.T) {
	u := logisticsUniverse(t)
	p, a, b := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	b.Position = a.Position
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol"}
	addExpeditionTestDock(p, Vector{})
	enemy := patrolTestEnemy(10000, p)
	enemy.Position = Vector{200, 0}
	enemy.WarpSpeed = 5
	enemy.Heading = VectorFloat64{-1, 0}
	enemy.Name = "31-ship invasion"
	p.FleetIntels = []*Fleet{enemy}
	u.Run((*turnGenerator).planPatrolAutomation)
	require.True(t, b.Delete)
	require.Greater(t, a.Spec.TotalShips, 1)
	require.Contains(t, readPatrolJob(a).Status, "Mobilizing")
	require.False(t, patrolCanAttack(p, a), "outmatched defenders must gather instead of charge")
}

func TestIntermediatePatrolStacksSplitWhenPackIsReleased(t *testing.T) {
	u := logisticsUniverse(t)
	p, a, b, leader := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat"), u.Fleet("Transport")
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol", leader.Num: "patrol"}
	a.Position = Vector{10, 0}
	b.Position = a.Position
	for _, f := range []*Fleet{a, b, leader} {
		savePatrolJob(f, patrolJob{Leader: leader.Num, Enemy: 2, Target: 91})
	}
	u.Run((*turnGenerator).mergePatrolRendezvous)
	require.Greater(t, a.Spec.TotalShips, 1)
	u.Run(func(tg *turnGenerator) { tg.releasePatrol(p, leader) })
	for _, f := range u.Game.getFleets(p.Num) {
		if !f.Delete {
			require.LessOrEqual(t, f.Spec.TotalShips, 1)
			require.Zero(t, readPatrolJob(f).Leader)
		}
	}
}

func TestStalledDistantPackDispersesAndDefersItsImpossibleTarget(t *testing.T) {
	u := logisticsUniverse(t)
	p, a, b := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	b.Position = Vector{15, 0}
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol"}
	e := patrolTestEnemy(100000, p)
	e.Position = Vector{130, 0}
	p.FleetIntels = []*Fleet{e}
	savePatrolJob(a, patrolJob{Leader: a.Num, Enemy: e.PlayerNum, Target: e.Num, GatheringSince: u.Game.Year - 3})
	savePatrolJob(b, patrolJob{Leader: a.Num, Enemy: e.PlayerNum, Target: e.Num})
	u.Run(func(tg *turnGenerator) { tg.advancePatrol(p, a) })
	require.Zero(t, readPatrolJob(a).Leader)
	require.Zero(t, readPatrolJob(b).Leader)
	require.True(t, declinedPatrolTarget(a, e, u.Game.Year))
	require.True(t, declinedPatrolTarget(b, e, u.Game.Year))
}

func TestPatrolDoesNotCountPlanetaryBombsAsAntiShipWeapons(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	known := &ShipDesign{PlayerNum: 2, Num: 90, Slots: []ShipDesignSlot{{Quantity: 4}}, Spec: ShipDesignSpec{Bomber: true, PowerRating: 10000}}
	p.ShipDesignIntels = []*ShipDesign{known}
	e := &Fleet{MapObject: MapObject{PlayerNum: 2}, Tokens: []ShipToken{{DesignNum: 90, Quantity: 8}}}
	require.Equal(t, 8, patrolReportedPower(p, e))
	known.Spec.HasWeapons = true
	known.Spec.PowerRating = 110
	known.Spec.Bombs = []Bomb{{Quantity: 1, KillRate: 5}}
	require.Equal(t, 80, patrolReportedPower(p, e), "ten anti-ship rating per ship, excluding one hundred bombing rating")
}
