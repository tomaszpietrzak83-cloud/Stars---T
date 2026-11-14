package cs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHullStatisticsSnapshotAndHistoricalTotals(t *testing.T) {
	u := logisticsUniverse(t)
	player := u.Player(1)
	design := &ShipDesign{Hull: "Frigate"}
	counters := player.accounting(u.Game.Year)
	counters.ShipsBuilt = 10
	counters.recordHull(design, HullStatistics{ShipsBuilt: 2, BuiltTonnage: 40})

	u.Run(func(tg *turnGenerator) {
		tg.beginStatistics()
		counters.ShipsBuilt++
		counters.recordHull(design, HullStatistics{ShipsBuilt: 1, BuiltTonnage: 20})
		tg.finishStatistics()
	})

	require.Equal(t, int64(11), counters.ShipsBuilt)
	require.Equal(t, HullStatistics{ShipsBuilt: 3, BuiltTonnage: 60}, counters.Hulls["Frigate"])
	require.Equal(t, HullStatistics{ShipsBuilt: 1, BuiltTonnage: 20},
		player.Stats.Accounting.History[0].Counters.Hulls["Frigate"])
}

func TestBattleDebrisCreditsOnlyRecoveredBattleMinerals(t *testing.T) {
	u := logisticsUniverse(t)
	player := u.Player(1)
	transport := u.Fleet("Transport")
	transport.Cargo = Cargo{}
	transport.Spec.CargoCapacity = 30

	// This field mixes a cargo dump with battle debris and survives a save and reload.
	source := newSalvage(transport.Position, 1, player.Num, Cargo{Ironium: 80, Boranium: 50})
	source.setBattleMinerals(Mineral{Ironium: 20, Boranium: 30})
	data, err := json.Marshal(source)
	require.NoError(t, err)
	source = &Salvage{}
	require.NoError(t, json.Unmarshal(data, source))
	require.Equal(t, Mineral{Ironium: 20, Boranium: 30}, source.battleMinerals())

	u.Run(func(tg *turnGenerator) {
		tg.beginStatistics()
		tg.recoverSalvage(player, transport, source)
		require.Equal(t, Mineral{Ironium: 30}, transport.Cargo.ToMineral())
		require.Equal(t, Mineral{Boranium: 30}, source.battleMinerals())
		transport.Cargo = Cargo{}
		transferer := newCargoTransferer(tg.log, tg.game)
		require.Equal(t, CargoTransferStatusNone,
			transferer.transferToDest(transport, source, Boranium, -10))
		tg.finishStatistics()
	})

	for _, counters := range []StatisticsCounters{player.Stats.Accounting.Counters, player.Stats.Accounting.History[0].Counters} {
		require.Equal(t, int64(30), counters.MineralsSalvaged)
		require.Equal(t, int64(20), counters.IroniumSalvaged)
		require.Equal(t, int64(10), counters.BoraniumSalvaged)
		require.Zero(t, counters.GermaniumSalvaged)
	}
	require.Equal(t, Mineral{Boranium: 20}, source.battleMinerals())

	source.decay(&u.Game.Rules)
	require.Equal(t, source.Cargo.Boranium/2, source.battleMinerals().Boranium)
}
