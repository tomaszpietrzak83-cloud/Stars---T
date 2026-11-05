//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
)

func TestRoutineRouteTradesOneExtraYearForSubstantialFuelSavings(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "escort"
	for _, token := range f.Tokens {
		token.design.Spec.Engine.FuelUsage = []int{0, 0, 0, 0, 1, 100, 200, 300, 400, 500, 600}
		token.design.Spec.Engine.MaxSafeSpeed = 9
	}
	f.Spec.Engine.MaxSafeSpeed = 9
	target := Vector{25, 0}
	economic := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, target, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true})
	fast := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, target, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true, Fast: true})
	require.NotNil(t, economic)
	require.NotNil(t, fast)
	require.Less(t, economic.Fuel, fast.Fuel)
	require.GreaterOrEqual(t, economic.ArrivalFuel[0], automaticReturnFuel(&u.Game.Rules, p, f, target, expeditionStations(p, f), true))
	t.Logf("Routine %d years/%d fuel; pursuit %d years/%d fuel", economic.Turns, economic.Fuel, fast.Turns, fast.Fuel)
}

func TestOrdinaryScoutDoesNotBlockRoutesOrAttractScoutRecon(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	enemy := patrolTestEnemy(0, p)
	enemy.Tokens = []ShipToken{{DesignNum: 77, Quantity: 1}}
	enemy.WarpSpeed = 5
	enemy.Heading = VectorFloat64{-1, 0}
	p.ShipDesignIntels = []*ShipDesign{{PlayerNum: 2, Num: 77, Hull: Scout.Name, Spec: ShipDesignSpec{ShipClass: ShipClassScout, Mass: 17}}}
	p.FleetIntels = []*Fleet{enemy}
	require.Zero(t, knownRouteThreat(p, Vector{}, Vector{60, 0}))
	require.False(t, reconCombatFormation(p, enemy))
	require.False(t, patrolReportedCombatant(p, enemy))
}

func TestUnknownEquipmentUsesWeaponAndGeneralCapacityTimesShipCount(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	enemy := patrolTestEnemy(0, p)
	enemy.Tokens = []ShipToken{{DesignNum: 77, Quantity: 3}}
	p.ShipDesignIntels = []*ShipDesign{{PlayerNum: 2, Num: 77, Hull: Destroyer.Name, Spec: ShipDesignSpec{Mass: 9999}}}
	slots := 0
	for _, slot := range Destroyer.Slots {
		if slot.Type&HullSlotTypeWeapon != 0 {
			slots += slot.Capacity
		}
	}
	require.Equal(t, slots*reportedWeaponUnit(p)*3, patrolReportedPower(p, enemy))
	enemy.Tokens[0].Quantity = 6
	require.Equal(t, slots*reportedWeaponUnit(p)*6, patrolReportedPower(p, enemy))
}

func TestRecoveryMovesPatrolEscortAndTransportDespiteEnemyProximity(t *testing.T) {
	for _, role := range []string{"patrol", "escort", "freighter"} {
		t.Run(role, func(t *testing.T) {
			u := logisticsUniverse(t)
			p, f := u.Player(1), u.Fleet("Escort")
			p.Automation.FleetRoles = map[int]string{f.Num: role}
			f.Position = Vector{100, 0}
			f.OrbitingPlanetNum = 0
			f.Fuel = f.Spec.FuelCapacity * 4 / 10
			enemy := patrolTestEnemy(100, p)
			enemy.Position = Vector{50, 0}
			p.FleetIntels = []*Fleet{enemy}
			u.Run((*turnGenerator).recoverIdleAutomaticFleets, (*turnGenerator).enforceAutomaticDeparture)
			require.True(t, fundedEmergencyReturn(&u.Game.Rules, p, f))
			require.Greater(t, len(f.Waypoints), 1)
			u.Run((*turnGenerator).fleetMove)
			require.Less(t, f.Position.X, 100)
			require.GreaterOrEqual(t, f.Fuel, 0)
		})
	}
}

func TestEmptyOffTerritoryFreighterWithdrawsInsteadOfWaitingForEscort(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	f.Position = Vector{40, 0}
	f.OrbitingPlanetNum = 0
	enemy := patrolTestEnemy(10000, p)
	enemy.Position = u.Game.Salvages[0].Position
	p.FleetIntels = []*Fleet{enemy}
	u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	require.Equal(t, "return", logisticsWork(f).Kind)
	require.Greater(t, len(f.Waypoints), 1)
	require.Equal(t, u.Planet("Home").Position, f.Waypoints[len(f.Waypoints)-1].Position)
}

func TestRescuerApproachesSlowScoutAndPreservesFuelConservation(t *testing.T) {
	u := tankerUniverse(t)
	p, customer, donor := u.Player(1), u.Fleet("Customer"), u.Fleet("Tanker")
	p.Automation.FleetRoles[customer.Num] = "scout"
	p.Automation.FleetRoles[donor.Num] = "escort"
	customer.Position = Vector{180, 0}
	saveAutomaticFuelRequest(customer, automaticFuelRequest{Destination: customer.Position, Members: []int{customer.Num}, Urgent: true})
	before := customer.Fuel + donor.Fuel
	u.Run((*turnGenerator).planAutomaticTankers)
	require.NotNil(t, readTankerJob(donor))
	require.Greater(t, len(donor.Waypoints), 1)
	require.NotEqual(t, donor.Position, readTankerJob(donor).Position, "the funded donor must close distance, not send a warp-3 scout all the way to itself")
	require.Equal(t, before, customer.Fuel+donor.Fuel)
}

func TestPatrolAndFreeEscortsActuallyMergeSevenDestroyers(t *testing.T) {
	fs := []ScenarioFleet{}
	for i := range 7 {
		fs = append(fs, ScenarioFleet{Name: fmt.Sprintf("Guard %d", i), Design: DesignDestroyerDelta.Name, At: "Home"})
	}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: fs}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}}})
	p := u.Player(1)
	p.Automation.FleetRoles = map[int]string{}
	for i := range 7 {
		role := "patrol"
		if i >= 4 {
			role = "escort"
		}
		p.Automation.FleetRoles[u.Fleet(fmt.Sprintf("Guard %d", i)).Num] = role
	}
	enemy := patrolTestEnemy(u.Fleet("Guard 0").Spec.PowerRating*4, p)
	p.FleetIntels = []*Fleet{enemy}
	u.Run((*turnGenerator).planPatrolAutomation)
	var leader *Fleet
	count := 0
	for _, f := range u.Game.getFleets(p.Num) {
		if !f.Delete && !f.Starbase {
			count++
			leader = f
		}
	}
	require.Equal(t, 1, count)
	require.Equal(t, 7, leader.Spec.TotalShips)
	require.Len(t, leader.Tokens, 1)
	require.Equal(t, 7, leader.Tokens[0].Quantity)
	require.True(t, patrolCanAttack(p, leader))
	p.FleetIntels = nil
	u.Run((*turnGenerator).planPatrolAutomation)
	roles := map[string]int{}
	for _, f := range u.Game.getFleets(p.Num) {
		if !f.Delete && !f.Starbase {
			roles[p.Automation.FleetRoles[f.Num]] += f.Spec.TotalShips
		}
	}
	require.Equal(t, 4, roles["patrol"])
	require.Equal(t, 3, roles["escort"])
}

func TestQueuedReplacementFollowsNewFamilyGenerationWithoutLosingPaidWork(t *testing.T) {
	u, p, planet, d := manufacturingTestWorld(t)
	next := *d
	next.Num = 99
	next.Name = "Newest generation"
	p.Designs = append(p.Designs, &next)
	p.DesignMaintenance = DesignMaintenance{d.Num: next.Num}
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeShipToken, DesignNum: d.Num, Quantity: 2, Allocated: Cost{Resources: 1}, Tags: Tags{replacementOrderTag: encodeMaintenance(&fleetMaintenance{Group: "test"})}, design: d}}
	planet.Cargo = Cargo{}
	planet.Spec.ResourcesPerYearAvailable = 0
	producer := newProducer(slog.Default(), &u.Game.Rules, planet, p)
	_, err := producer.produce()
	require.NoError(t, err)
	require.Len(t, planet.ProductionQueue, 1)
	require.Equal(t, next.Num, planet.ProductionQueue[0].DesignNum)
	require.Equal(t, 2, planet.ProductionQueue[0].Quantity)
	require.Equal(t, Cost{Resources: 1}, planet.ProductionQueue[0].Allocated)
}

func TestUrgentRendezvousKeepsTheSameThreatPolicyAtDeparture(t *testing.T) {
	u := logisticsUniverse(t)
	p, client, donor := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{client.Num: "scout", donor.Num: "patrol"}
	client.Position = Vector{80, 0}
	client.OrbitingPlanetNum = 0
	client.Fuel = 100
	client.Waypoints = []Waypoint{{Position: client.Position}, {Position: donor.Position, WarpSpeed: 4}}
	client.SetTag(tankerAwaitTag, fmt.Sprint(donor.Num))
	writeTankerJob(donor, &automaticTankerJob{Urgent: true, TargetNum: client.Num, Members: []int{client.Num}, Position: donor.Position, DeliveryFuel: 20})
	enemy := patrolTestEnemy(1000, p)
	enemy.Position = Vector{40, 0}
	p.FleetIntels = []*Fleet{enemy}
	u.Run((*turnGenerator).enforceAutomaticDeparture)
	require.Greater(t, len(client.Waypoints), 1, "the same funded rendezvous cannot be rejected later solely for reported proximity")
	require.Equal(t, donor.Position, client.Waypoints[1].Position)
}

func TestPassingCombatShipSharesOnlyGenuinelySpareFuel(t *testing.T) {
	u := logisticsUniverse(t)
	p, client, donor := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{client.Num: "scout", donor.Num: "patrol"}
	client.Position = Vector{50, 0}
	donor.Position = client.Position
	client.Fuel = 0
	before := client.Fuel + donor.Fuel
	u.Run((*turnGenerator).shareOpportunisticFuel)
	require.Positive(t, client.Fuel)
	require.Equal(t, before, client.Fuel+donor.Fuel)
	require.GreaterOrEqual(t, donor.Fuel, automaticReturnFuel(&u.Game.Rules, p, donor, donor.Position, expeditionStations(p, donor), true))
}

func TestRecoveryPreservesPurposefulWaitForMovingRallyMembers(t *testing.T) {
	u := logisticsUniverse(t)
	p, leader, member := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	p.Automation.FleetRoles = map[int]string{leader.Num: "patrol", member.Num: "patrol"}
	leader.Position = Vector{40, 0}
	member.Position = Vector{60, 0}
	leader.Waypoints = []Waypoint{{Position: leader.Position}}
	member.Waypoints = []Waypoint{{Position: member.Position}, {Position: leader.Position, WarpSpeed: 4}}
	savePatrolJob(leader, patrolJob{Leader: leader.Num})
	savePatrolJob(member, patrolJob{Leader: leader.Num})
	u.Run((*turnGenerator).recoverIdleAutomaticFleets)
	require.Len(t, leader.Waypoints, 1, "a leader must not run away from members actually approaching its rally")
}

func TestExistingPatrolPackCanRecruitANewFreeEscort(t *testing.T) {
	u := logisticsUniverse(t)
	p, leader, support := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	support.Position = leader.Position
	p.Automation.FleetRoles = map[int]string{leader.Num: "patrol", support.Num: "escort"}
	enemy := patrolTestEnemy(patrolPower(leader), p)
	p.FleetIntels = []*Fleet{enemy}
	savePatrolJob(leader, patrolJob{Leader: leader.Num, Enemy: enemy.PlayerNum, Target: enemy.Num})
	u.Run((*turnGenerator).planPatrolAutomation)
	require.True(t, support.Delete)
	require.Equal(t, 3, leader.Spec.TotalShips)
	require.True(t, patrolCanAttack(p, leader))
	require.Equal(t, 1, readPatrolJob(leader).SupportDesigns[support.Tokens[0].DesignNum])
}

func TestRecoveryDoesNotInterruptBombingOrAnActiveMobileRepair(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b := u.Player(1), u.Fleet("Bomber")
	b.Position = u.Planet("Enemy").Position
	b.Waypoints = []Waypoint{{Position: b.Position}}
	u.Run((*turnGenerator).recoverIdleAutomaticFleets)
	require.Len(t, b.Waypoints, 1)
	v := tankerUniverse(t)
	q, patient, doctor := v.Player(1), v.Fleet("Customer"), v.Fleet("Tanker")
	patient.Position = Vector{40, 0}
	doctor.Position = patient.Position
	patient.Waypoints = []Waypoint{{Position: patient.Position}}
	patient.SetTag(safetyTag, `{"kind":"repair"}`)
	patient.SetTag(repairTankerTag, fmt.Sprint(doctor.Num))
	v.Run((*turnGenerator).recoverIdleAutomaticFleets)
	require.Len(t, patient.Waypoints, 1)
	require.Equal(t, "freighter", q.Automation.FleetRoles[patient.Num])
	require.Equal(t, "bomber", p.Automation.FleetRoles[b.Num])
}

func TestRetreatDoesNotDissolveAMergedPatrolIntoSeparateBattleStacks(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	f.Tokens[0].Quantity = 7
	f.Spec = ComputeFleetSpec(&u.Game.Rules, p, f)
	f.Fuel = f.Spec.FuelCapacity
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	savePatrolJob(f, patrolJob{Leader: f.Num, Grouped: true, Enemy: 2, Target: 91})
	f.SetTag(safetyTag, `{"kind":"evade"}`)
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Equal(t, 7, f.Spec.TotalShips)
	require.Equal(t, 7, f.Tokens[0].Quantity)
	require.True(t, readPatrolJob(f).Grouped)
}
