package server

import (
	"connectrpc.com/connect"
	"context"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	"github.com/sirgwain/craig-stars/proto/converter"
	v1 "github.com/sirgwain/craig-stars/proto/gen/craig_stars/v1"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAutomaticColonizerRolePersistsAfterSplitAllAPI(t *testing.T) {
	database, cfg := connectSubmitTestDB(t)
	c := database.NewReadWriteClient()
	ctx := t.Context()
	user, err := c.CreateUser(ctx, cs.NewUser("inheritance", "pw", "", cs.RoleUser))
	require.NoError(t, err)
	gr := NewGameRunner(database, cfg)
	full, err := gr.HostGame(user.ID, cs.NewGameSettings().WithHost(cs.Humanoids()))
	require.NoError(t, err)
	p := full.Players[0]
	var colony *cs.Fleet
	for _, f := range full.Fleets {
		if f.PlayerNum == p.Num && !f.Starbase && f.Spec.Colonizer {
			colony = f
			break
		}
	}
	require.NotNil(t, colony)
	colony.Tokens[0].Quantity = 3
	colony.Spec = cs.ComputeFleetSpec(&full.Rules, p, colony)
	colony.Fuel = colony.Spec.FuelCapacity
	require.NoError(t, c.SaveFleet(ctx, colony))
	game, err := c.GetGame(ctx, full.ID)
	require.NoError(t, err)
	ctx = context.WithValue(ctx, keyDbRead, database.NewReadClient())
	ctx = context.WithValue(ctx, keyDbWrite, c)
	ctx = context.WithValue(ctx, keyGame, game)
	ctx = context.WithValue(ctx, keyGamePlayer, &game.Players[0])
	orders := converter.C.ConvertCSPlayer(p).PlayerOrders
	orders.Automation = &v1.LogisticsAutomation{FleetRoles: map[int32]string{int32(colony.Num): "colonizer:auto"}}
	service := NewPlayerServiceHandler(database, cfg, newDiscordNotifier(database, cfg))
	_, err = service.UpdatePlayerOrders(ctx, connect.NewRequest(&v1.UpdatePlayerOrdersRequest{GameId: full.ID, Orders: orders}))
	require.NoError(t, err)
	fleetService := NewFleetServiceHandler(database)
	result, err := fleetService.SplitAllFleets(ctx, connect.NewRequest(&v1.SplitAllFleetsRequest{GameId: full.ID, FleetNum: int32(colony.Num)}))
	require.NoError(t, err)
	require.Len(t, result.Msg.Fleets, 3)
	saved, err := c.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	for _, f := range result.Msg.Fleets {
		require.Equal(t, "colonizer:auto", saved.Automation.FleetRoles[int(f.MapObject.Num)])
	}
	original := p.GetDesign(colony.Tokens[0].DesignNum)
	draft := converter.C.ConvertCSShipDesign(original)
	draft.Name = "Colony Mk 2"
	designs := NewshipDesignServiceHandler(database)
	upgraded, err := designs.CreateShipDesign(ctx, connect.NewRequest(&v1.CreateShipDesignRequest{GameId: full.ID, Design: draft, UpgradeFromDesignNum: int32(original.Num)}))
	require.NoError(t, err)
	require.NotNil(t, upgraded.Msg.PlayerOrders)
	newest := upgraded.Msg.Design.Num
	storedUpgrade, err := c.GetShipDesignByNum(ctx, full.ID, p.Num, int(newest))
	require.NoError(t, err)
	require.Equal(t, game.Year, storedUpgrade.Spec.VersionCreatedYear)
	require.Equal(t, original.Num, storedUpgrade.Spec.PreviousVersionNum)
	require.Equal(t, newest, upgraded.Msg.PlayerOrders.DesignMaintenance[int32(original.Num)])
	draft = upgraded.Msg.Design
	draft.Name = "Colony Mk 3"
	third, err := designs.CreateShipDesign(ctx, connect.NewRequest(&v1.CreateShipDesignRequest{GameId: full.ID, Design: draft, UpgradeFromDesignNum: newest}))
	require.NoError(t, err)
	require.Equal(t, third.Msg.Design.Num, third.Msg.PlayerOrders.DesignMaintenance[int32(original.Num)])
	require.Equal(t, third.Msg.Design.Num, third.Msg.PlayerOrders.DesignMaintenance[newest])
	storedThird, err := c.GetShipDesignByNum(ctx, full.ID, p.Num, int(third.Msg.Design.Num))
	require.NoError(t, err)
	require.Equal(t, game.Year, storedThird.Spec.VersionCreatedYear)
	require.Equal(t, int(newest), storedThird.Spec.PreviousVersionNum)
	require.Equal(t, third.Msg.PlayerOrders.Automation.DesignFamilies[int32(original.Num)], third.Msg.PlayerOrders.Automation.DesignFamilies[third.Msg.Design.Num])

	// The intermediate zero-instance version remains historical after Mk 3.
	history, err := c.GetShipDesignsForPlayer(ctx, full.ID, p.Num)
	require.NoError(t, err)
	found := false
	for _, d := range history {
		if d.Num == int(newest) {
			found = true
			require.Zero(t, d.Spec.NumInstances)
			require.Equal(t, "Colony Mk 2", d.Name)
		}
	}
	require.True(t, found)
	require.NotEqual(t, upgraded.Msg.Design.GameDbObject.Id, third.Msg.Design.GameDbObject.Id)

	// Regression for a stale backend rejecting the newly introduced patrol role.
	armed := cs.NewShipDesign(p.Num, 100).WithHull(cs.Destroyer.Name).WithSlots(cs.DestroyerDeltaTestSlots).WithSpec(&full.Rules, p)
	armed.GameID = full.ID
	require.NoError(t, c.SaveShipDesign(ctx, armed))
	p.Designs = append(p.Designs, armed)
	colony.Tokens = []cs.ShipToken{{DesignNum: armed.Num, Quantity: 1}}
	p.InjectDesigns([]*cs.Fleet{colony})
	colony.Spec = cs.ComputeFleetSpec(&full.Rules, p, colony)
	colony.Fuel = colony.Spec.FuelCapacity
	require.NoError(t, c.SaveFleet(ctx, colony))
	patrolOrders := third.Msg.PlayerOrders
	patrolOrders.Automation.FleetRoles[int32(colony.Num)] = "patrol"
	accepted, err := service.UpdatePlayerOrders(ctx, connect.NewRequest(&v1.UpdatePlayerOrdersRequest{GameId: full.ID, Orders: patrolOrders}))
	require.NoError(t, err)
	require.Equal(t, "patrol", accepted.Msg.Player.PlayerOrders.Automation.FleetRoles[int32(colony.Num)])

}
