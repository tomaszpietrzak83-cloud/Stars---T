package db

import (
	"github.com/sirgwain/craig-stars/cs"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDesignMaintenancePersistence(t *testing.T) {
	c := connectTestDB()
	game := c.createTestGame(t.Context())
	p := &cs.Player{GameDBObject: cs.GameDBObject{GameID: game.ID}, UserID: 1, Num: 1, Name: "Maintenance",
		PlayerOrders: cs.PlayerOrders{DesignMaintenance: cs.DesignMaintenance{1: 2}}}
	require.NoError(t, c.SavePlayer(t.Context(), p))
	p.Automation = cs.LogisticsAutomation{FleetRoles: map[int]string{1: "freighter", 2: "escort"}, BattleSalvage: true, OffloadCombatCargo: true}
	p.Automation.FleetAssemblies = map[int]cs.FleetAssembly{1: {Name: "Task Force", RallyPlanetNum: 7, Designs: map[int]int{1: 10}, SourceFleets: map[int]bool{3: true}, BuildMissing: true, Revision: 2, LeaderFleetNum: 3, NotifiedRevision: 2, Status: "Assembling"}}
	p.Automation.PriorityFleets = map[string]bool{"2:17": true, "3:17": true}
	require.NoError(t, c.UpdatePlayerOrders(t.Context(), p))
	loaded, err := c.GetPlayerForGame(t.Context(), game.ID, 1)
	require.NoError(t, err)
	require.Equal(t, p.DesignMaintenance, loaded.DesignMaintenance)
	require.Equal(t, p.Automation, loaded.Automation)
	p.DesignMaintenance = cs.DesignMaintenance{1: cs.RetireDesign}
	require.NoError(t, c.UpdatePlayerOrders(t.Context(), p))
	light, err := c.GetLightPlayerForGame(t.Context(), game.ID, GetPlayerParams{PlayerNum: 1})
	require.NoError(t, err)
	require.Equal(t, p.DesignMaintenance, light.DesignMaintenance)
	p.DesignMaintenance = nil
	require.NoError(t, c.UpdatePlayerOrders(t.Context(), p))
	loaded, err = c.GetPlayerForGame(t.Context(), game.ID, 1)
	require.NoError(t, err)
	require.Empty(t, loaded.DesignMaintenance)
}
