//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func rapidUpgradeUniverse(t *testing.T) *testUniverse {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	p := u.Player(1)
	old, next := p.Designs[0], p.Designs[1]
	old.Version, next.Version = 2, 3
	old.Spec.VersionCreatedYear, next.Spec.VersionCreatedYear = 2440, 2444
	next.Spec.PreviousVersionNum = old.Num
	p.Automation.DesignFamilies = map[int]int{old.Num: -1, next.Num: -1}
	return u
}

func TestRapidUpgradeRecoveryRequiresDatedConsecutiveFamilyVersions(t *testing.T) {
	cases := []struct {
		name     string
		change   func(*Player, *ShipDesign, *ShipDesign)
		eligible bool
	}{
		{"four years", func(p *Player, a, b *ShipDesign) {}, true},
		{"same year", func(p *Player, a, b *ShipDesign) { b.Spec.VersionCreatedYear = a.Spec.VersionCreatedYear }, true},
		{"five years", func(p *Player, a, b *ShipDesign) { b.Spec.VersionCreatedYear = 2445 }, false},
		{"unknown old date", func(p *Player, a, b *ShipDesign) { a.Spec.VersionCreatedYear = 0 }, false},
		{"unknown new date", func(p *Player, a, b *ShipDesign) { b.Spec.VersionCreatedYear = 0 }, false},
		{"reverse dates", func(p *Player, a, b *ShipDesign) { b.Spec.VersionCreatedYear = 2439 }, false},
		{"skipped version", func(p *Player, a, b *ShipDesign) { b.Version = 4 }, false},
		{"wrong predecessor", func(p *Player, a, b *ShipDesign) { b.Spec.PreviousVersionNum = 99 }, false},
		{"different family", func(p *Player, a, b *ShipDesign) { p.Automation.DesignFamilies[b.Num] = -2 }, false},
		{"no family", func(p *Player, a, b *ShipDesign) { p.Automation.DesignFamilies = nil }, false},
		{"different hull", func(p *Player, a, b *ShipDesign) { b.Hull = Cruiser.Name }, false},
		{"transferred", func(p *Player, a, b *ShipDesign) { a.OriginalPlayerNum = 2 }, false},
		{"foreign", func(p *Player, a, b *ShipDesign) { b.PlayerNum = 2 }, false},
		{"starbase", func(p *Player, a, b *ShipDesign) { a.Spec.Starbase = true; b.Spec.Starbase = true }, false},
		{"initial version", func(p *Player, a, b *ShipDesign) { a.Version = 0; b.Version = 2 }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := rapidUpgradeUniverse(t)
			p := u.Player(1)
			a, b := p.Designs[0], p.Designs[1]
			tc.change(p, a, b)
			credit, ok := rapidUpgradeRecovery(p, a, b, Cost{Ironium: 40, Boranium: 20, Germanium: 10, Resources: 100})
			require.Equal(t, tc.eligible, ok)
			if ok {
				require.Equal(t, Cost{Ironium: 36, Boranium: 18, Germanium: 9, Resources: 90}, credit)
			} else {
				require.Zero(t, credit)
			}
		})
	}
}

func TestRapidUpgradeScrapRecoversOldCostAndQueuesFullPriceSuccessor(t *testing.T) {
	u := rapidUpgradeUniverse(t)
	p, f, planet := u.Player(1), u.Fleet("Fleet"), u.Planet("Yard")
	old, next := p.Designs[0], p.Designs[1]
	// A later arrival after finishing a task does not change the project gap.
	u.Game.Year = 2455
	u.Run((*turnGenerator).fleetDesignMaintenance)
	available := Cost{}
	u.turn.maintainShipsAtPlanet(p, planet, &available)
	require.True(t, f.Delete)
	perShip, _ := rapidUpgradeRecovery(p, old, next, old.Spec.Cost)
	expected := MultiplyCost(perShip, 2)
	require.Equal(t, expected, available, "one 90% refund, with resources and every mineral")
	require.Len(t, planet.ProductionQueue, 2, "the normal distributor splits the two old ships")
	item := planet.ProductionQueue[0]
	require.Equal(t, next.Num, item.DesignNum)
	require.Equal(t, 1, item.Quantity)
	require.Zero(t, item.Allocated, "scrap is real yard stock, not a second paid allocation")
	producer := newProducer(testLogger, &u.Game.Rules, planet, p)
	cost, err := producer.getItemCost(&u.Game.Rules, p, planet, item)
	require.NoError(t, err)
	require.Equal(t, next.Spec.Cost, cost, "new ships retain their complete normal price")
}

func TestRapidUpgradeDoesNotChangeRetirementOrManualScrap(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		u := rapidUpgradeUniverse(t)
		p, f, planet := u.Player(1), u.Fleet("Fleet"), u.Planet("Yard")
		normal := f.getScrapAmount(&u.Game.Rules, p, planet, false)
		if automatic {
			p.DesignMaintenance[p.Designs[0].Num] = RetireDesign
			u.Run((*turnGenerator).fleetDesignMaintenance)
			available := Cost{}
			u.turn.maintainShipsAtPlanet(p, planet, &available)
			require.Equal(t, normal, available)
		} else {
			f.Waypoints[0].Task = WaypointTaskScrapFleet
			before := planet.Cargo
			u.Run((*turnGenerator).fleetScrap)
			require.True(t, f.Delete)
			require.Equal(t, normal.ToMineral(), planet.Cargo.Subtract(before).ToMineral())
		}
	}
}

func TestRapidUpgradeRecoveryIsNotIncreasedByRecyclingTraits(t *testing.T) {
	u := rapidUpgradeUniverse(t)
	p := u.Player(1)
	p.Race.Spec.ScrapMineralOffsetStarbase = 1
	p.Race.Spec.ScrapResourcesOffsetStarbase = 1
	credit, ok := rapidUpgradeRecovery(p, p.Designs[0], p.Designs[1], Cost{Ironium: 11, Boranium: 21, Germanium: 31, Resources: 101})
	require.True(t, ok)
	require.Equal(t, Cost{Ironium: 9, Boranium: 18, Germanium: 27, Resources: 90}, credit)
}

func TestRapidUpgradeHistorySurvivesSpecRecomputationAndPlanning(t *testing.T) {
	u := rapidUpgradeUniverse(t)
	p, f, planet := u.Player(1), u.Fleet("Fleet"), u.Planet("Yard")
	u.Recompute()
	old, next := p.Designs[0], p.Designs[1]
	require.Equal(t, 2440, old.Spec.VersionCreatedYear)
	require.Equal(t, 2444, next.Spec.VersionCreatedYear)
	require.Equal(t, old.Num, next.Spec.PreviousVersionNum)
	writeMaintenance(f, &fleetMaintenance{Group: "incoming", Stage: "scrap", PlanetNum: planet.Num})
	credit, ok := rapidUpgradeRecovery(p, old, next, old.Spec.Cost)
	require.True(t, ok)
	need := u.turn.incomingRebuildMinerals(p, planet)
	require.Equal(t, MultiplyCost(next.Spec.Cost.Subtract(credit).MinZero(), 2).ToMineral(), need)
	batch, _ := rapidUpgradeRecovery(p, old, next, MultiplyCost(old.Spec.Cost, 2))
	require.Equal(t, float64(max(0, 2*next.Spec.Cost.Resources-batch.Resources)), u.turn.rebuildYardLoads(p, "")[planet.Num])
}

func TestDesignUpgradeFromOlderDraftRecordsActualLatestPredecessor(t *testing.T) {
	u := rapidUpgradeUniverse(t)
	p := u.Player(1)
	old, previous := p.Designs[0], p.Designs[1]
	next := *old
	next.Num = 10
	next.Spec.VersionCreatedYear = 2445
	require.NoError(t, p.RegisterDesignUpgrade(old, &next))
	require.Equal(t, previous.Num, next.Spec.PreviousVersionNum)
	require.Equal(t, 4, next.Version)
	_, ok := rapidUpgradeRecovery(p, previous, &next, previous.Spec.Cost)
	require.True(t, ok)
	_, ok = rapidUpgradeRecovery(p, old, &next, old.Spec.Cost)
	require.False(t, ok, "skipping the intermediate version gets ordinary scrap")
}
