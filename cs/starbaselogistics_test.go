//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGoodColonyPassengerTripPrefetchesTwentyPercentMineralsWithoutGermanium(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	dest.Spec.HasStarbase = false
	dest.Spec.Habitability = 100
	dest.Hab = Hab{50, 50, 50}
	dest.BaseHab = dest.Hab
	dest.Mines = 0
	dest.Cargo = Cargo{Colonists: 1}
	source.Cargo = Cargo{Colonists: 9500, Ironium: 1000, Boranium: 1000, Germanium: 1000}
	before := source.Cargo
	u.Run(func(tg *turnGenerator) {
		tg.loadAutomaticPassengers(p, f, &logisticsJob{Kind: "passengers", TargetNum: source.Num, DeliveryNum: dest.Num, Position: source.Position, Passengers: f.Spec.CargoCapacity})
	})
	require.Equal(t, f.Spec.CargoCapacity, f.CargoMass())
	require.Equal(t, f.Spec.CargoCapacity/5, f.Cargo.Ironium+f.Cargo.Boranium)
	require.Zero(t, f.Cargo.Germanium)
	require.Equal(t, f.Spec.CargoCapacity-f.Spec.CargoCapacity/5, f.Cargo.Colonists)
	require.Equal(t, before, source.Cargo.Add(f.Cargo))
}

func TestFutureStarbaseReceivesMineralsBeforeThereIsADock(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	dest.Spec.HasStarbase = false
	dest.Spec.DockCapacity = 0
	dest.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeStarbase, DesignNum: p.Designs[2].Num, Quantity: 1}}
	require.NoError(t, dest.PopulateProductionQueueDesigns(p))
	source.Cargo.Ironium = 500
	source.Cargo.Boranium = 500
	source.Cargo.Germanium = 500
	dest.Cargo.Germanium = 0
	dest.Spec.MiningOutput = Mineral{}
	dest.Mines = 0
	var job *logisticsJob
	u.Run(func(tg *turnGenerator) { job = tg.findShipyardSupply(p, f, map[string]bool{}) })
	require.NotNil(t, job)
	require.Equal(t, dest.Num, job.DeliveryNum)
	f.Position = dest.Position
	f.Cargo = Cargo{Germanium: 100}
	saveLogisticsWork(f, &logisticsJob{Kind: "return", TargetNum: dest.Num, DeliveryNum: dest.Num, Position: dest.Position})
	u.Run((*turnGenerator).resolveLogisticsAutomation)
	require.Equal(t, 100, dest.Cargo.Germanium)
	require.Zero(t, f.CargoMass())
}

func TestAdvanceConstructionStockDoesNotDisplacePassengerShare(t *testing.T) {
	u := passengerUniverse(t)
	p, f, source, dest := u.Player(1), u.Fleet("Transport"), u.Planet("Home"), u.Planet("Young colony")
	p.Automation.ManualPassengerThresholds = false
	dest.Hab = Hab{50, 50, 50}
	dest.Spec.Habitability = 100
	dest.Cargo = Cargo{Colonists: 1}
	dest.Mines = 0
	source.Cargo = Cargo{Colonists: sourcePopulationReserve(p, source) + f.Spec.CargoCapacity/2, Ironium: 1000, Boranium: 1000, Germanium: 1000}
	before := source.Cargo
	u.Run(func(tg *turnGenerator) {
		tg.loadAutomaticPassengers(p, f, &logisticsJob{Kind: "passengers", TargetNum: source.Num, DeliveryNum: dest.Num, Position: source.Position})
	})
	require.Zero(t, f.CargoMass())
	require.Equal(t, before, source.Cargo)
}
