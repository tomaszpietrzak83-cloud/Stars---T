//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestRadiatingFreighterRejectsNewPassengersAndUnloadsExistingPeople(t *testing.T) {
	u := passengerUniverse(t)
	p, f, home := u.Player(1), u.Fleet("Transport"), u.Planet("Home")
	f.Spec.Radiating = true
	require.Nil(t, u.turn.findPassengerTransfer(p, f))
	require.Nil(t, u.turn.findAutomaticInvasion(p, f))
	before := home.Cargo.Colonists
	f.Cargo.Colonists = 100
	saveLogisticsWork(f, &logisticsJob{Kind: "passengers", TargetNum: home.Num, DeliveryNum: 2})
	saveFuelNetworkPlan(f, &fuelNetworkPlan{Leader: f.Num, Members: []int{f.Num}, Roles: map[int]string{f.Num: "freighter"}, Orders: map[int]FleetOrders{f.Num: f.FleetOrders}})
	f.SetTag(fuelNetworkMemberTag, strconv.Itoa(f.Num))
	u.Run((*turnGenerator).prepareRadiatingTransports)
	require.False(t, fuelNetworkManaged(f), "old passenger fuel reservations must not restore an illegal route")
	require.Equal(t, before+100, home.Cargo.Colonists)
	require.Zero(t, f.Cargo.Colonists)
	require.Nil(t, logisticsWork(f))
	require.True(t, passengerCarrier(p, f), "non-passenger logistics must remain enabled")
	p.Automation.DisableSalvage = false
	p.Automation.DisablePassengerTransport = false
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.NotNil(t, logisticsWork(f))
	require.Equal(t, "salvage", logisticsWork(f).Kind)
}

func TestRadiatingMineralPickupDoesNotFillHoldWithSettlers(t *testing.T) {
	u := passengerUniverse(t)
	f, home, dest := u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	f.Spec.Radiating = true
	home.Cargo.Ironium = 100
	saveLogisticsWork(f, &logisticsJob{Kind: "planet", TargetNum: home.Num, DeliveryNum: dest.Num, Position: home.Position, Load: Mineral{Ironium: 50}})
	before := home.Cargo.Colonists
	u.Run((*turnGenerator).resolveLogisticsAutomation)
	require.Zero(t, f.Cargo.Colonists)
	require.Equal(t, before, home.Cargo.Colonists)
}

func TestFuelMeetingMovesBothFleetsAtFreeSpeeds(t *testing.T) {
	u := logisticsUniverse(t)
	p, donor, client := u.Player(1), u.Fleet("Escort"), u.Fleet("Transport")
	p.Automation.FleetRoles[donor.Num] = "patrol"
	client.Position = Vector{20, 0}
	client.OrbitingPlanetNum = 0
	client.Fuel = 1
	client.SetTag(tankerAwaitTag, strconv.Itoa(donor.Num))
	addExpeditionTestDock(p, Vector{})
	job := &automaticTankerJob{TargetNum: client.Num, Members: []int{client.Num}, Position: client.Position, DeliveryFuel: 50, Urgent: true}
	beforeDonor, beforeClient := donor.Fuel, client.Fuel
	u.Run(func(tg *turnGenerator) { require.True(t, tg.routeTankerJob(p, donor, job)) })
	require.NotEqual(t, donor.Position, job.Position)
	require.NotEqual(t, client.Position, job.Position)
	require.Greater(t, len(donor.Waypoints), 1)
	require.Greater(t, len(client.Waypoints), 1)
	require.LessOrEqual(t, donor.Waypoints[1].WarpSpeed, freeRendezvousWarp(p, donor))
	require.LessOrEqual(t, client.Waypoints[1].WarpSpeed, freeRendezvousWarp(p, client))
	u.Run((*turnGenerator).fleetMove)
	require.Greater(t, donor.Position.X, 0)
	require.Less(t, client.Position.X, 20)
	require.GreaterOrEqual(t, donor.Fuel, beforeDonor)
	require.GreaterOrEqual(t, client.Fuel, beforeClient)
}

func TestDirectCruiseNetworkCannotLockPatrolToOldTarget(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "patrol"
	near := patrolTestEnemy(0)
	near.Num = 22
	near.Position = Vector{10, 0}
	far := patrolTestEnemy(0)
	far.Num = 23
	far.Position = Vector{140, 0}
	p.FleetIntels = []*Fleet{far, near}
	addExpeditionTestDock(p, Vector{})
	savePatrolJob(f, patrolJob{Leader: f.Num, Enemy: 2, Target: far.Num})
	saveFuelNetworkPlan(f, &fuelNetworkPlan{Leader: f.Num, Members: []int{f.Num}, Roles: map[int]string{f.Num: "patrol"}, Orders: map[int]FleetOrders{f.Num: f.FleetOrders}, Route: fuelNetworkRoute{Legs: []fuelNetworkLeg{{From: f.Position, To: far.Position}}}})
	f.SetTag(fuelNetworkMemberTag, strconv.Itoa(f.Num))
	u.Run((*turnGenerator).releaseCombatCruiseNetworks, (*turnGenerator).planPatrolAutomation)
	require.False(t, fuelNetworkManaged(f))
	require.Equal(t, near.Num, readPatrolJob(f).Target)
	require.Greater(t, len(f.Waypoints), 1)
}

func TestGatheringPatrolLeaderAndFollowerMoveTowardSharedPoint(t *testing.T) {
	u := logisticsUniverse(t)
	p, a, b := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	b.Position = Vector{80, 0}
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol"}
	addExpeditionTestDock(p, Vector{})
	savePatrolJob(a, patrolJob{Leader: a.Num})
	savePatrolJob(b, patrolJob{Leader: a.Num})
	job := readPatrolJob(a)
	u.Run(func(tg *turnGenerator) { require.True(t, tg.movingPatrolRally(p, a, &job)) })
	require.Equal(t, a.Waypoints[1].Position, b.Waypoints[1].Position)
	require.NotEqual(t, a.Position, a.Waypoints[1].Position)
	require.NotEqual(t, b.Position, b.Waypoints[1].Position)
	u.Run((*turnGenerator).fleetMove)
	require.Greater(t, a.Position.X, 0)
	require.Less(t, b.Position.X, 80)
}

func TestStatisticsCreditActualShooterAndRecordOwnLosses(t *testing.T) {
	u := logisticsUniverse(t)
	u.Game.Players = append(u.Game.Players, &Player{Num: 2}, &Player{Num: 3})
	record := &BattleRecord{Tokens: []BattleRecordToken{{Num: 1, PlayerNum: 1, StartingQuantity: 1, Mass: 20}, {Num: 2, PlayerNum: 2, StartingQuantity: 2, Mass: 30}, {Num: 3, PlayerNum: 3, StartingQuantity: 1, Mass: 20}}, DestroyedTokens: []BattleRecordDestroyedToken{{Num: 2, PlayerNum: 2, Quantity: 2}}, ActionsPerRound: [][]BattleRecordTokenAction{{{TokenNum: 1, TargetNum: 2, TokensDestroyed: 2, Type: TokenActionBeamFire}}}}
	u.Run(func(tg *turnGenerator) {
		tg.beginStatistics()
		tg.recordBattleStatistics(record)
		tg.finishStatistics()
	})
	c := u.Player(1).Stats.Accounting.Counters
	require.Equal(t, int64(2), c.ShipsDestroyed)
	require.Equal(t, int64(60), c.DestroyedTonnage)
	require.Equal(t, int64(0), u.Player(3).Stats.Accounting.Counters.ShipsDestroyed)
	require.Equal(t, int64(2), u.Player(2).Stats.Accounting.Counters.ShipsLost)
	require.Len(t, u.Player(1).Stats.Accounting.History, 1)
	require.Equal(t, int64(2), u.Player(1).Stats.Accounting.History[0].Counters.ShipsDestroyed)
}

func TestProductionStatisticsDoNotRecountOldPartialAllocation(t *testing.T) {
	u := logisticsUniverse(t)
	p, home := u.Player(1), u.Planet("Home")
	home.Spec.ResourcesPerYearAvailable = 1
	home.Cargo.Germanium = 100
	home.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeFactory, Quantity: 1}}
	var counters StatisticsCounters
	for range 3 {
		producer := newProducer(u.turn.log, &u.Game.Rules, home, p)
		producer.onSpend = func(delta Cost) { recordProductionSpending(&counters, delta) }
		_, err := producer.produce()
		require.NoError(t, err)
	}
	require.Equal(t, int64(3), counters.ProductionResourcesSpent)
	before := counters
	estimator := newProducer(u.turn.log, &u.Game.Rules, home, p)
	estimator.estimating = true
	_, err := estimator.produce()
	require.NoError(t, err)
	require.Equal(t, before, counters)
}

func TestRadiatingEvacuationDoesNotCancelAnUnrelatedReusedDonorNumber(t *testing.T) {
	u := logisticsUniverse(t)
	f, donor, other := u.Fleet("Transport"), u.Fleet("Escort"), u.Fleet("Combat")
	f.Spec.Radiating = true
	f.Cargo.Colonists = 100
	writeTankerJob(donor, &automaticTankerJob{TargetNum: other.Num, Members: []int{other.Num}})
	saveFuelNetworkPlan(donor, &fuelNetworkPlan{Leader: donor.Num, Members: []int{other.Num}})
	f.SetTag(tankerAwaitTag, strconv.Itoa(donor.Num))
	f.SetTag(fuelNetworkMemberTag, strconv.Itoa(donor.Num))
	u.Run((*turnGenerator).prepareRadiatingTransports)
	require.NotNil(t, readTankerJob(donor))
	require.Equal(t, other.Num, readTankerJob(donor).TargetNum)
	require.NotNil(t, readFuelNetworkPlan(donor))
	require.Empty(t, f.GetTag(tankerAwaitTag))
	require.Empty(t, f.GetTag(fuelNetworkMemberTag))
}
