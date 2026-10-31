//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func patrolTestEnemy(power int) *Fleet {
	return &Fleet{MapObject: MapObject{Num: 91, PlayerNum: 2, Name: "Enemy scout", Position: Vector{30, 0}}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: power > 0, PowerRating: power}}}
}
func TestPatrolInterceptsWeakVisibleTargetWithReturnFuel(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	p.FleetIntels = []*Fleet{patrolTestEnemy(0)}
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
	p.FleetIntels = []*Fleet{patrolTestEnemy(leader.Spec.PowerRating)}
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
			enemy := patrolTestEnemy(f.Spec.PowerRating * 10)
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
	p.FleetIntels = []*Fleet{patrolTestEnemy(0)}
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
	enemy := patrolTestEnemy(0)
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
	enemy := patrolTestEnemy(0)
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
	require.Equal(t, enemy.Num, readPatrolJob(f).Target)
	require.True(t, patrolCanAttack(p, f))
}

func TestClosestPatrolGetsNearbyScoutInsteadOfDistantLowNumberFleet(t *testing.T) {
	u := logisticsUniverse(t)
	p, far, near := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	far.Position = Vector{}
	near.Position = Vector{45, 0}
	p.Automation.FleetRoles = map[int]string{far.Num: "patrol", near.Num: "patrol"}
	enemy := patrolTestEnemy(0)
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
