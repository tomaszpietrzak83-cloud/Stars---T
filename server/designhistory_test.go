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

func TestDesignHistoryIsServerOwnedAcrossCreateAndEdit(t *testing.T) {
	database, cfg := connectSubmitTestDB(t)
	c := database.NewReadWriteClient()
	ctx := t.Context()
	user, err := c.CreateUser(ctx, cs.NewUser("design-history", "pw", "", cs.RoleUser))
	require.NoError(t, err)
	full, err := NewGameRunner(database, cfg).HostGame(user.ID, cs.NewGameSettings().WithHost(cs.Humanoids()))
	require.NoError(t, err)
	p := full.Players[0]
	var original *cs.ShipDesign
	for _, d := range p.Designs {
		if !d.Spec.Starbase {
			original = d
			break
		}
	}
	require.NotNil(t, original)
	require.Equal(t, full.Year, original.Spec.VersionCreatedYear, "new-game starter designs have known dates")
	game, err := c.GetGame(ctx, full.ID)
	require.NoError(t, err)
	game.Year = 2440 // Request context only; no personal game or turn is involved.
	ctx = context.WithValue(ctx, keyDbRead, database.NewReadClient())
	ctx = context.WithValue(ctx, keyDbWrite, c)
	ctx = context.WithValue(ctx, keyGame, game)
	ctx = context.WithValue(ctx, keyGamePlayer, &game.Players[0])
	service := NewshipDesignServiceHandler(database)
	draft := converter.C.ConvertCSShipDesign(original)
	draft.Name = "History Mk 2"
	result, err := service.CreateShipDesign(ctx, connect.NewRequest(&v1.CreateShipDesignRequest{GameId: full.ID, Design: draft, UpgradeFromDesignNum: int32(original.Num)}))
	require.NoError(t, err)
	stored, err := c.GetShipDesignByNum(ctx, full.ID, p.Num, int(result.Msg.Design.Num))
	require.NoError(t, err)
	require.Equal(t, 2440, stored.Spec.VersionCreatedYear)
	require.Equal(t, original.Num, stored.Spec.PreviousVersionNum)
	version := stored.Version
	orders, err := c.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	orders.DesignMaintenance = nil
	require.NoError(t, c.UpdatePlayerOrders(ctx, orders))
	game.Year = 2443
	draft = result.Msg.Design
	draft.Name = "History Mk 2 edited"
	draft.Version = 999
	_, err = service.UpdateShipDesign(ctx, connect.NewRequest(&v1.UpdateShipDesignRequest{GameId: full.ID, Design: draft}))
	require.NoError(t, err)
	stored, err = c.GetShipDesignByNum(ctx, full.ID, p.Num, int(draft.Num))
	require.NoError(t, err)
	require.Equal(t, 2440, stored.Spec.VersionCreatedYear, "editing cannot rewrite the project creation year")
	require.Equal(t, original.Num, stored.Spec.PreviousVersionNum)
	require.Equal(t, version, stored.Version)
	loaded, err := c.GetLightPlayerForGameWithDesigns(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	require.Equal(t, 2440, loaded.GetDesign(stored.Num).Spec.VersionCreatedYear)
}
