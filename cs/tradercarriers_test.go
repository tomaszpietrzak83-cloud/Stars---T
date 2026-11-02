//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func traderCarrierUniverse(t *testing.T) (*testUniverse, *ShipDesign) {
	large := ShipDesign{Name: "Trader Large Freighter", Hull: LargeFreighter.Name,
		Slots: []ShipDesignSlot{{HullComponent: TransStar10.Name, HullSlotIndex: 1, Quantity: 2}}}
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, large, DesignSpaceStation), Fleets: []ScenarioFleet{
		{Name: "Small carriers", At: "Home", Tokens: []ScenarioShipToken{{Design: DesignTeamster.Name, Quantity: 40}}},
		{Name: "Large carrier", At: "Home", Design: large.Name},
	}}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name,
		Cargo: Cargo{Colonists: 10000, Ironium: 100000, Boranium: 100000, Germanium: 100000}}}})
	p := u.Player(1)
	p.Automation.FleetRoles = map[int]string{u.Fleet("Small carriers").Num: "freighter", u.Fleet("Large carrier").Num: "freighter"}
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	return u, p.GetDesignByName(large.Name)
}

func TestTraderPrefersLargeFreighterOverLargerSmallCarrierStack(t *testing.T) {
	u, _ := traderCarrierUniverse(t)
	p, large := u.Player(1), u.Fleet("Large carrier")
	require.Greater(t, u.Fleet("Small carriers").Spec.CargoCapacity, large.Spec.CargoCapacity)
	mt := &MysteryTrader{MapObject: MapObject{Num: 1, Position: Vector{5, 0}}, RequestedBoon: large.Spec.CargoCapacity, Destination: Vector{-100, 0}}
	u.Run(func(tg *turnGenerator) {
		members := tg.reserveTraderConvoy(p, mt)
		require.Len(t, members, 1)
		require.Same(t, large, members[0])
	})
	require.Empty(t, u.Fleet("Small carriers").GetTag(traderConvoyTag))
}

func TestTraderWaitsForActualNextTurnAutomaticLargeFreighter(t *testing.T) {
	u, design := traderCarrierUniverse(t)
	p, planet := u.Player(1), u.Planet("Home")
	delete(p.Automation.FleetRoles, u.Fleet("Large carrier").Num) // Manual stays manual.
	planet.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeShipToken, DesignNum: design.Num, Quantity: 1,
		Tags: Tags{replacementOrderTag: encodeMaintenance(&fleetMaintenance{OriginalRole: "freighter", Group: "queued-trader-test"})}}}
	before, err := json.Marshal([]any{p, planet})
	require.NoError(t, err)
	u.Game.Rules.ResetSeed(12345)
	control := NewRulesWithSeed(12345)
	u.Run(func(tg *turnGenerator) {
		capacity := tg.queuedTraderLargeFreighterCapacity(p)
		require.Equal(t, design.Spec.CargoCapacity, capacity)
		mt := &MysteryTrader{MapObject: MapObject{Num: 1, Position: Vector{5, 0}}, RequestedBoon: capacity, Destination: Vector{-100, 0}}
		require.Empty(t, tg.reserveTraderConvoy(p, mt))
	})
	require.Equal(t, control.random.Intn(1000000), u.Game.Rules.random.Intn(1000000), "forecast must not consume game RNG")
	after, err := json.Marshal([]any{p, planet})
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after), "checking queues must not spend minerals or alter orders")
	require.Empty(t, u.Fleet("Small carriers").GetTag(traderConvoyTag))
	require.Empty(t, p.Automation.FleetRoles[u.Fleet("Large carrier").Num])
	// The same queue loses eligibility if it belongs to a manual ship or a
	// fleet composition, or if an earlier order prevents next-turn completion.
	planet.ProductionQueue[0].Tags = nil
	u.Run(func(tg *turnGenerator) { require.Zero(t, tg.queuedTraderLargeFreighterCapacity(p)) })
	planet.ProductionQueue[0].Tags = Tags{replacementOrderTag: encodeMaintenance(&fleetMaintenance{OriginalRole: "freighter"}), assemblyTag: "7"}
	u.Run(func(tg *turnGenerator) { require.Zero(t, tg.queuedTraderLargeFreighterCapacity(p)) })
	delete(planet.ProductionQueue[0].Tags, assemblyTag)
	planet.ProductionQueue = append([]ProductionQueueItem{{Type: QueueItemTypeShipToken, DesignNum: p.GetDesignByName(DesignTeamster.Name).Num, Quantity: 100000}}, planet.ProductionQueue...)
	u.Run(func(tg *turnGenerator) { require.Zero(t, tg.queuedTraderLargeFreighterCapacity(p)) })
}
