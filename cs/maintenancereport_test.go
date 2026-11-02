//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func maintenanceMessages(p *Player) []PlayerMessage {
	result := []PlayerMessage{}
	for _, m := range p.Messages {
		if strings.HasPrefix(m.Text, "Ship upgrades and retirements (") {
			result = append(result, m)
		}
	}
	return result
}

func TestMaintenanceReportsAllFleetsOncePerTurn(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	p, old := u.Player(1), u.Fleet("Fleet")
	extra, err := u.turn.buildFleet(p, u.Planet("Yard"), ShipToken{DesignNum: old.Tokens[0].DesignNum, Quantity: 3, design: old.Tokens[0].design}, nil, nil)
	require.NoError(t, err)
	p.Automation.RebuildFleets[extra.Num] = true
	require.NoError(t, u.turn.generateTurn())
	messages := maintenanceMessages(p)
	require.Len(t, messages, 1)
	require.Contains(t, messages[0].Text, "Scrapped 5 ships (5 x Sword1)")
	require.Contains(t, messages[0].Text, "Built 5 replacement ships (5 x Sword2)")
	for _, m := range p.Messages {
		require.NotEqual(t, PlayerMessageFleetScrapped, m.Type)
		require.NotContains(t, m.Text, "100% of the rebuilt fleet")
	}
	require.NoError(t, u.turn.generateTurn())
	for _, m := range maintenanceMessages(p) {
		require.NotContains(t, m.Text, "Scrapped")
		require.NotContains(t, m.Text, "Built 5 replacement ships")
	}
}

func TestMaintenanceSummaryDoesNotClaimUnfinishedReplacements(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	p, planet := u.Player(1), u.Planet("Yard")
	planet.Spec.ResourcesPerYearAvailable = 1
	u.Run((*turnGenerator).fleetDesignMaintenance)
	producer := newProducer(testLogger, &u.Game.Rules, planet, p)
	producer.maintainShips = func(available *Cost) { u.turn.maintainShipsAtPlanet(p, planet, available) }
	result, err := producer.produce()
	require.NoError(t, err)
	require.Empty(t, result.tokens)
	u.turn.reportShipMaintenance()
	messages := maintenanceMessages(p)
	require.Len(t, messages, 1)
	require.Contains(t, messages[0].Text, "Scrapped 2 ships")
	require.NotContains(t, messages[0].Text, "Built")
	u.turn.reportShipMaintenance()
	require.Len(t, maintenanceMessages(p), 1, "flushing twice cannot duplicate the summary")
}

func TestManualScrappingRetainsItsOwnMessage(t *testing.T) {
	u := maintenanceUniverse(t)
	f := u.Fleet("Fleet")
	f.Waypoints[0].Task = WaypointTaskScrapFleet
	u.Run((*turnGenerator).fleetScrap)
	require.True(t, f.Delete)
	found := false
	for _, m := range u.Player(1).Messages {
		found = found || m.Type == PlayerMessageFleetScrapped
	}
	require.True(t, found)
	require.Empty(t, maintenanceMessages(u.Player(1)))
}
