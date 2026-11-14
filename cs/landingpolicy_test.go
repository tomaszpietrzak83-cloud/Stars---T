//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoundedLandingReleasesMillionPersonArmyWithoutLosingCargo(t *testing.T) {
	u := landingOperationUniverse(t)
	p, leader := u.Player(1), u.Fleet("Transport0")
	leader.Tokens[0].Quantity = 100
	leader.Spec = ComputeFleetSpec(&u.Game.Rules, p, leader)
	leader.Cargo.Colonists, leader.Fuel = 20000, leader.Spec.FuelCapacity
	op := &landingOperation{Target: u.Planet("Target").Num, Owner: 2, Need: 20000, Started: u.Game.Year, Rally: leader.Position, Phase: "gather"}
	saveLandingOperation(leader, op)
	saveLogisticsWork(leader, &logisticsJob{Kind: "landing-rally", DeliveryNum: op.Target})
	beforePeople, beforeFuel, beforeShips := 0, 0, 0
	for _, f := range u.Game.getFleets(1) {
		beforePeople += f.Cargo.Colonists
		beforeFuel += f.Fuel
		beforeShips += f.Spec.TotalShips
	}
	u.Run((*turnGenerator).releaseBlockedLandings)
	people, fuel, ships := 0, 0, 0
	for _, f := range u.Game.getFleets(1) {
		if f.Delete {
			continue
		}
		people += f.Cargo.Colonists
		fuel += f.Fuel
		ships += f.Spec.TotalShips
		require.False(t, landingManaged(f))
		if f.Cargo.Colonists > 0 {
			require.LessOrEqual(t, f.Spec.TotalShips, 1)
			require.Nil(t, logisticsWork(f))
		}
	}
	require.Equal(t, beforePeople, people)
	require.Equal(t, beforeFuel, fuel)
	require.Equal(t, beforeShips, ships)
}

func TestBoundedLandingWaitsForBombingAndCapsReservations(t *testing.T) {
	u := landingOperationUniverse(t)
	p := u.Player(1)
	target := p.GetPlanetIntel(u.Planet("Target").Num)
	for _, base := range []bool{true, false} {
		target.Spec.HasStarbase = base
		target.Cargo.Colonists = 20000
		u.Run((*turnGenerator).planLandingOperations)
		require.Nil(t, landingLeader(u))
		require.NotEmpty(t, target.GetTag(landingPreparationTag))
	}
	target.Cargo.Colonists, target.ReportAge = 500, 0
	u.Run((*turnGenerator).planLandingOperations)
	leader := landingLeader(u)
	require.NotNil(t, leader)
	reserved := u.turn.landingReservedTroops(p, leader)
	require.GreaterOrEqual(t, reserved, 2000)
	require.LessOrEqual(t, reserved, landingArmyLimit)
}

func TestBoundedLandingStallReleasesArmyAndDoesNotImmediatelyRecruitAgain(t *testing.T) {
	u := landingOperationUniverse(t)
	p, leader := u.Player(1), u.Fleet("Transport0")
	leader.Cargo.Colonists = 500
	op := &landingOperation{Target: u.Planet("Target").Num, Owner: 2, Need: 2000, Started: u.Game.Year, Rally: leader.Position, Phase: "gather"}
	saveLandingOperation(leader, op)
	u.Run((*turnGenerator).releaseBlockedLandings)
	for range 4 {
		u.Game.Year++
		u.Run((*turnGenerator).releaseBlockedLandings)
	}
	require.Nil(t, readLandingOperation(leader))
	u.Run((*turnGenerator).planLandingOperations)
	require.Nil(t, landingLeader(u))
	require.Equal(t, fmt.Sprint(u.Game.Year+5), p.GetPlanetIntel(op.Target).GetTag(landingRetryTag))
}
