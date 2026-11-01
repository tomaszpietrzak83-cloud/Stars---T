//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestManualFleetCannotBeBorrowedByBomber(t *testing.T) {
	u := logisticsUniverse(t)
	p, f, b := u.Player(1), u.Fleet("Escort"), u.Fleet("Transport")
	delete(p.Automation.FleetRoles, f.Num)
	f.Waypoints = append(f.Waypoints, NewPositionWaypoint(Vector{45, 0}, 6))
	before := f.FleetOrders
	u.Run(func(tg *turnGenerator) { tg.assignBomberProtection(p, b, Vector{45, 0}, true) })
	require.Empty(t, p.Automation.FleetRoles[f.Num])
	require.Empty(t, f.GetTag(bomberBorrowTag))
	require.Equal(t, before, f.FleetOrders)
}
func TestTakeManualControlPreservesOrdersAndRemovesResumableJobs(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles[f.Num] = "escort"
	f.SetTag(bomberBorrowTag, `{"role":"patrol"}`)
	f.SetTag(patrolTag, `{"leader":1}`)
	f.SetTag(safetyTag, `{"kind":"evade"}`)
	f.SetTag("note", "keep")
	f.Waypoints = append(f.Waypoints, NewPositionWaypoint(Vector{45, 0}, 6))
	before := f.FleetOrders
	p.TakeManualControl(f)
	require.Equal(t, before, f.FleetOrders)
	require.Equal(t, "keep", f.GetTag("note"))
	require.Empty(t, f.GetTag(bomberBorrowTag))
	require.Empty(t, f.GetTag(patrolTag))
	require.Empty(t, p.Automation.FleetRoles[f.Num])
}
func TestShipIntelIndexTracksDiscoveries(t *testing.T) {
	p := &Player{}
	a := &ShipDesign{PlayerNum: 2, Num: 1}
	p.ShipDesignIntels = []*ShipDesign{a}
	p.buildShipIntelIndex()
	require.Same(t, a, p.GetShipDesignIntel(2, 1))
	b := &ShipDesign{PlayerNum: 2, Num: 2}
	p.ShipDesignIntels = append(p.ShipDesignIntels, b)
	require.Same(t, b, p.GetShipDesignIntel(2, 2))
	require.Nil(t, p.GetShipDesignIntel(3, 1))
}

func TestReturnBudgetUnloadsOnlyAtKnownOwnDelivery(t *testing.T) {
	u := passengerUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	var dest *Planet
	for _, planet := range u.Game.getPlanets(p.Num) {
		if planet.Position != f.Position {
			dest = planet
			break
		}
	}
	require.NotNil(t, dest)
	dest.Spec.MaxPopulation = 1000000
	dest.Cargo.Colonists = 0
	p.PlanetIntels = []*Planet{dest}
	f.Cargo.Colonists = 10
	saveLogisticsWork(f, &logisticsJob{Kind: "settle", TargetNum: dest.Num, DeliveryNum: dest.Num, Position: dest.Position})
	budget := deliveryReturnBudget(p, f, dest.Position)
	require.Zero(t, budget.Cargo.Colonists)
	require.Zero(t, budget.Spec.CargoCapacity)
	require.Equal(t, 10, f.Cargo.Colonists)
	require.NotZero(t, deliveryReturnBudget(p, f, f.Position).Spec.CargoCapacity)
}

func TestAlreadyManualFleetKeepsItsNewerOrdersOverLegacyBorrow(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	delete(p.Automation.FleetRoles, f.Num)
	f.Waypoints = append(f.Waypoints, NewPositionWaypoint(Vector{45, 0}, 6))
	before := f.FleetOrders
	f.SetTag(bomberBorrowTag, `{"role":"","orders":{"waypoints":[{"position":{"x":0,"y":0}},{"position":{"x":100,"y":0}}]}}`)
	u.Run((*turnGenerator).restoreBorrowedManualFleets)
	require.Equal(t, before, f.FleetOrders)
	require.Empty(t, f.GetTag(bomberBorrowTag))
	require.Empty(t, p.Automation.FleetRoles[f.Num])
}
