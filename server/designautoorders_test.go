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

func TestDesignAutoOrdersPersistsOnInUseDesignAndUpgrade(t *testing.T) {
	database, cfg := connectSubmitTestDB(t)
	client := database.NewReadWriteClient()
	ctx := t.Context()
	user, err := client.CreateUser(ctx, cs.NewUser("design-auto-orders", "pw", "", cs.RoleUser))
	require.NoError(t, err)
	full, err := NewGameRunner(database, cfg).HostGame(user.ID, cs.NewGameSettings().WithHost(cs.Humanoids()))
	require.NoError(t, err)
	p := full.Players[0]
	var original *cs.ShipDesign
	for _, d := range p.Designs {
		if !d.Spec.Starbase && d.Spec.FuelCapacity > 0 && d.Spec.NumInstances > 0 {
			original = d
			break
		}
	}
	require.NotNil(t, original, "exercise the in-use blueprint path")
	game, err := client.GetGame(ctx, full.ID)
	require.NoError(t, err)
	ctx = context.WithValue(ctx, keyDbRead, database.NewReadClient())
	ctx = context.WithValue(ctx, keyDbWrite, client)
	ctx = context.WithValue(ctx, keyGame, game)
	ctx = context.WithValue(ctx, keyGamePlayer, &game.Players[0])
	svc := NewshipDesignServiceHandler(database)
	before, err := client.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	result, err := svc.UpdateShipDesignAutoOrders(ctx, connect.NewRequest(&v1.UpdateShipDesignAutoOrdersRequest{GameId: full.ID, Num: int32(original.Num), Role: "tanker"}))
	require.NoError(t, err)
	require.Equal(t, "tanker", result.Msg.Design.Spec.DefaultAutoRole)
	stored, err := client.GetShipDesignByNum(ctx, full.ID, p.Num, original.Num)
	require.NoError(t, err)
	require.Equal(t, "tanker", stored.Spec.DefaultAutoRole)
	require.Equal(t, original.Slots, stored.Slots)
	require.Equal(t, original.Spec.VersionCreatedYear, stored.Spec.VersionCreatedYear)
	after, err := client.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	require.Equal(t, before.Automation.FleetRoles, after.Automation.FleetRoles)
	draft := converter.C.ConvertCSShipDesign(stored)
	draft.Name = "Auto orders successor"
	draft.Spec.DefaultAutoRole = ""
	created, err := svc.CreateShipDesign(ctx, connect.NewRequest(&v1.CreateShipDesignRequest{GameId: full.ID, Design: draft, UpgradeFromDesignNum: int32(original.Num)}))
	require.NoError(t, err)
	require.Equal(t, "tanker", created.Msg.Design.Spec.DefaultAutoRole)
	reloaded, err := client.GetShipDesignByNum(ctx, full.ID, p.Num, int(created.Msg.Design.Num))
	require.NoError(t, err)
	require.Equal(t, "tanker", reloaded.Spec.DefaultAutoRole)
	_, err = svc.UpdateShipDesignAutoOrders(ctx, connect.NewRequest(&v1.UpdateShipDesignAutoOrdersRequest{GameId: full.ID, Num: int32(original.Num), Role: "invented-role"}))
	require.Error(t, err)
}
