//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDesignAutoOrdersBuildUpgradeAndReplacement(t *testing.T) {
	u := maintenanceUniverse(t)
	p, planet := u.Player(1), u.Planet("Yard")
	source, next := p.Designs[0], p.Designs[1]
	source.Spec.DefaultAutoRole = "scout"
	require.NoError(t, p.RegisterDesignUpgrade(source, next))
	require.Equal(t, "scout", next.Spec.DefaultAutoRole)
	spec, err := ComputeShipDesignSpec(&u.Game.Rules, p.TechLevels, p.Race.Spec, next)
	require.NoError(t, err)
	require.Equal(t, "scout", spec.DefaultAutoRole)
	next.Spec = spec
	raw, err := json.Marshal(next)
	require.NoError(t, err)
	var reloaded ShipDesign
	require.NoError(t, json.Unmarshal(raw, &reloaded))
	require.Equal(t, "scout", reloaded.Spec.DefaultAutoRole)
	fleet, err := u.turn.buildFleet(p, planet, ShipToken{DesignNum: next.Num, Quantity: 1, design: next}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "scout", p.Automation.FleetRoles[fleet.Num], "role enabled as soon as production creates the fleet")
	for _, original := range []string{"", "miner", "scout"} {
		work := &fleetMaintenance{OriginalRole: original, Group: "default-vs-existing-" + original, BaseName: "Existing", RallyPlanetNum: planet.Num}
		rebuilt, err := u.turn.addReplacementShips(p, planet, ShipToken{DesignNum: next.Num, Quantity: 1, design: next}, work)
		require.NoError(t, err)
		require.Equal(t, original, p.Automation.FleetRoles[rebuilt.Num], "existing manual/automatic orders override blueprint defaults")
	}
}

func TestDesignAutoOrdersValidatesActualEquipment(t *testing.T) {
	u := maintenanceUniverse(t)
	p := u.Player(1)
	d := p.Designs[0]
	d.Spec.DefaultAutoRole = "scout"
	require.NoError(t, d.Validate(&u.Game.Rules, p))
	for _, role := range []string{"patrol", "bomber", "unknown-role"} {
		d.Spec.DefaultAutoRole = role
		require.Error(t, d.Validate(&u.Game.Rules, p))
	}
	d.Spec.DefaultAutoRole = "manual"
	require.NoError(t, d.Validate(&u.Game.Rules, p))
	require.Error(t, ValidateDesignAutoRole("patrol", ShipDesignSpec{Starbase: true, HasWeapons: true}))
}
