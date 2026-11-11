package cs

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatrolFuelDeliveryRequiresSingleShipsAndClearArea(t *testing.T) {
	u := tankerUniverse(t)
	p, donor, receiver := u.Player(1), u.Fleet("Tanker"), u.Fleet("Customer")
	p.Automation.FleetRoles[donor.Num] = "patrol"
	request := func() {
		saveAutomaticFuelRequest(receiver, automaticFuelRequest{Destination: receiver.Position, Members: []int{receiver.Num}, Urgent: true})
	}

	// Recover a legacy delivery already assigned to a merged combat fleet.
	donor.Tokens[0].Quantity = 2
	donor.Spec = ComputeFleetSpec(&u.Game.Rules, p, donor)
	donor.Fuel = donor.Spec.FuelCapacity
	receiver.SetTag(tankerAwaitTag, strconv.Itoa(donor.Num))
	writeTankerJob(donor, &automaticTankerJob{TargetNum: receiver.Num, Members: []int{receiver.Num}, Position: receiver.Position, DeliveryFuel: 30})
	u.Run((*turnGenerator).clearInvalidPatrolFuelDeliveries)
	require.Nil(t, readTankerJob(donor))
	require.Empty(t, receiver.GetTag(tankerAwaitTag))
	request()
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Nil(t, readTankerJob(donor))

	donor.Tokens[0].Quantity = 1
	donor.Spec = ComputeFleetSpec(&u.Game.Rules, p, donor)
	donor.Fuel = donor.Spec.FuelCapacity
	receiver.Tokens[0].Quantity = 2
	receiver.Spec = ComputeFleetSpec(&u.Game.Rules, p, receiver)
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Nil(t, readTankerJob(donor), "a single patrol cannot refuel a stack")

	receiver.Tokens[0].Quantity = 1
	receiver.Spec = ComputeFleetSpec(&u.Game.Rules, p, receiver)
	p.FleetIntels = []*Fleet{{MapObject: MapObject{PlayerNum: 2, Num: 1, Position: receiver.Position}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{Bomber: true}}}}
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Nil(t, readTankerJob(donor), "known bombers rule out the rendezvous")

	p.FleetIntels = nil
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Equal(t, strconv.Itoa(donor.Num), receiver.GetTag(tankerAwaitTag))
	require.Equal(t, []int{receiver.Num}, readTankerJob(donor).Members)
}

func TestDispersedPatrolFillsTanksWithConservedFuel(t *testing.T) {
	u := tankerUniverse(t)
	p, stack := u.Player(1), u.Fleet("Tanker")
	p.Automation.FleetRoles[stack.Num] = "patrol"
	stack.Tokens[0].Quantity = 3
	stack.Spec = ComputeFleetSpec(&u.Game.Rules, p, stack)
	stack.Fuel = stack.Spec.FuelCapacity / 2
	fuel := stack.Fuel
	savePatrolJob(stack, patrolJob{Leader: stack.Num, Grouped: true})
	u.Run(func(tg *turnGenerator) { tg.releasePatrol(p, stack) })
	remaining, full, ships := 0, 0, 0
	for _, f := range u.Game.getFleets(p.Num) {
		if p.Automation.FleetRoles[f.Num] != "patrol" {
			continue
		}
		require.Equal(t, 1, f.Spec.TotalShips)
		remaining += f.Fuel
		ships++
		if f.Fuel == f.Spec.FuelCapacity {
			full++
		}
	}
	require.Equal(t, 3, ships)
	require.Equal(t, fuel, remaining)
	require.Equal(t, 1, full)
}

func TestPatrolCruiseUsesLaterEnginesFreeWarp(t *testing.T) {
	for _, tt := range []struct {
		engine string
		warp   int
	}{
		{RadiatingHydroRamScoop.Name, 6},
		{TransGalacticSuperScoop.Name, 7},
		{TransGalacticMizerScoop.Name, 8},
		{GalaxyScoop.Name, 9},
	} {
		t.Run(tt.engine, func(t *testing.T) {
			d := Designs(DesignDestroyerDelta)[0]
			d.Slots[0].HullComponent = tt.engine
			u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(d), Fleets: []ScenarioFleet{{Name: "Hunter", Design: d.Name, EmptyFuel: true}}}}})
			p, f := u.Player(1), u.Fleet("Hunter")
			require.Equal(t, tt.warp, economicalPatrolMaxWarp(p, f))
			require.Equal(t, 0, f.GetFuelCost(p, tt.warp, f.StrategicSpeed(tt.warp)))
		})
	}
}

func TestPatrolFuelMeetingRejectsOrbitAndAllowsNearbyOpenSpace(t *testing.T) {
	u := tankerUniverse(t)
	p, donor, receiver := u.Player(1), u.Fleet("Tanker"), u.Fleet("Customer")
	p.Automation.FleetRoles[donor.Num] = "patrol"
	colony := u.Planet("Colony")
	receiver.Position = colony.Position
	require.False(t, patrolFuelMeetingAllowed(p, donor, []*Fleet{receiver}, colony.Position))
	require.True(t, patrolFuelMeetingAllowed(p, donor, []*Fleet{receiver}, colony.Position.Add(Vector{1, 0})))
}

func TestPatrolOrbitCoverageRequiresArrivalThisTurn(t *testing.T) {
	u := tankerUniverse(t)
	p, f := u.Player(1), u.Fleet("Tanker")
	enemy := &Fleet{MapObject: MapObject{Num: 77, PlayerNum: 2, Position: Vector{30, 0}}}
	job := patrolJob{Leader: f.Num, Enemy: 2, Target: 77, Provisional: true}
	f.Waypoints = []Waypoint{{Position: f.Position}, {Position: enemy.Position, WarpSpeed: 6}}
	savePatrolJob(f, job)
	require.False(t, patrolAttackArrivesThisTurn(p, []*Fleet{f}, enemy))
	job.Provisional = false
	savePatrolJob(f, job)
	f.Waypoints[1].WarpSpeed = 5
	require.False(t, patrolAttackArrivesThisTurn(p, []*Fleet{f}, enemy))
	f.Waypoints[1].WarpSpeed = 6
	require.True(t, patrolAttackArrivesThisTurn(p, []*Fleet{f}, enemy))
}
