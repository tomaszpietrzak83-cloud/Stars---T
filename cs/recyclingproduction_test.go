//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchivedRecyclingProduction(t *testing.T) {
	path := os.Getenv("STARS_RECYCLING_ARCHIVE")
	if path == "" {
		t.Skip("optional copied recycling archive regression")
	}
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(abs)), "/audit-results/")
	f, err := os.Open(abs)
	require.NoError(t, err)
	defer f.Close()
	z, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer z.Close()
	var archive struct{ Game *FullGame }
	require.NoError(t, json.NewDecoder(z).Decode(&archive))
	g := archive.Game
	require.NotNil(t, g)
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	g.Rules.ResetSeed(g.Seed)
	turn := newTurnGenerator(g)
	require.NoError(t, g.computeSpecs())
	yard := g.getPlanet(225)
	require.Equal(t, "Riverbend", yard.Name)
	before := yard.Factories
	// Replay the affected phases only; archives omit unrelated AI report caches.
	turn.fleetScrap()
	require.Equal(t, 126, yard.bonusResources)
	turn.planetMine()
	require.NoError(t, turn.planetProduction())
	require.Equal(t, 12, yard.Factories-before, "126 recovered points fund ten extra factories at cost 12")
	require.Zero(t, yard.bonusResources)
	t.Logf("Copied Riverbend: built %d factories, total %d; live save untouched", yard.Factories-before, yard.Factories)
}

func TestManualRecyclingFundsFactoriesOnce(t *testing.T) {
	u := maintenanceUniverse(t)
	p, yard, fleet := u.Player(1), u.Planet("Yard"), u.Fleet("Fleet")
	p.Race.WithLRT(UR)
	p.Race.FactoryCost = 12
	u.Recompute()
	// Isolate a 120-resource refund from ordinary annual production.
	fleet.Spec.Cost.Resources = 150
	u.turn.scrapFleet(fleet, false)
	require.Equal(t, 120, yard.bonusResources)
	yard.Spec.ResourcesPerYearAvailable = 36
	yard.Spec.MaxFactories = 1000
	yard.Spec.MaxPossibleFactories = 1000
	yard.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeAutoFactories, Quantity: 100}}
	producer := newProducer(testLogger, &u.Game.Rules, yard, p)
	result, err := producer.produce()
	require.NoError(t, err)
	require.Equal(t, 13, result.factories, "three ordinary factories plus ten funded by recycling")
	require.Zero(t, yard.bonusResources)
	require.Equal(t, 36, yard.Spec.ResourcesPerYearAvailable, "annual output does not include the one-time credit")
	result, err = producer.produce()
	require.NoError(t, err)
	require.Equal(t, 3, result.factories, "the refund cannot be spent again next year")
}

func TestRecyclingAddsAcrossFleetsAndUnusedCreditReachesResearch(t *testing.T) {
	u := maintenanceUniverse(t)
	p, yard, fleet := u.Player(1), u.Planet("Yard"), u.Fleet("Fleet")
	p.Race.WithLRT(UR)
	u.Recompute()
	extra, err := u.turn.buildFleet(p, yard, ShipToken{DesignNum: p.Designs[0].Num, Quantity: 1, design: p.Designs[0]}, nil, nil)
	require.NoError(t, err)
	fleet.Spec.Cost.Resources, extra.Spec.Cost.Resources = 100, 50
	u.turn.scrapFleet(fleet, false)
	u.turn.scrapFleet(extra, false)
	require.Equal(t, 120, yard.bonusResources)
	yard.ProductionQueue = nil
	yard.Spec.ResourcesPerYearAvailable = 36
	producer := newProducer(testLogger, &u.Game.Rules, yard, p)
	result, err := producer.produce()
	require.NoError(t, err)
	require.Equal(t, 156, result.leftoverResources)
	require.Zero(t, yard.bonusResources)
}
