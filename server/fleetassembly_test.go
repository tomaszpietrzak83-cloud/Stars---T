package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	"github.com/sirgwain/craig-stars/proto/converter"
	v1 "github.com/sirgwain/craig-stars/proto/gen/craig_stars/v1"
	"github.com/stretchr/testify/require"
)

func TestFleetAssemblyOrdersPersistAndCompleteAcrossTurns(t *testing.T) {
	dbConn, cfg := connectSubmitTestDB(t)
	c := dbConn.NewReadWriteClient()
	ctx := t.Context()
	user, err := c.CreateUser(ctx, cs.NewUser("composer", "pw", "", cs.RoleUser))
	require.NoError(t, err)
	gr := NewGameRunner(dbConn, cfg)
	full, err := gr.HostGame(user.ID, cs.NewGameSettings().WithHost(cs.Humanoids()))
	require.NoError(t, err)
	p := full.Players[0]
	var scout *cs.Fleet
	for _, fleet := range full.Fleets {
		if fleet.PlayerNum == p.Num && !fleet.Starbase && fleet.Spec.CargoCapacity == 0 {
			scout = fleet
			break
		}
	}
	require.NotNil(t, scout)
	var yard *cs.Planet
	for _, planet := range full.Planets {
		if planet.PlayerNum == p.Num && planet.Spec.HasStarbase {
			yard = planet
			break
		}
	}
	require.NotNil(t, yard)
	game, err := c.GetGame(ctx, full.ID)
	require.NoError(t, err)
	ctx = context.WithValue(ctx, keyDbRead, dbConn.NewReadClient())
	ctx = context.WithValue(ctx, keyGame, game)
	ctx = context.WithValue(ctx, keyGamePlayer, &game.Players[0])
	svc := NewPlayerServiceHandler(dbConn, cfg, newDiscordNotifier(dbConn, cfg))
	orders := converter.C.ConvertCSPlayer(p).PlayerOrders
	orders.Automation = &v1.LogisticsAutomation{FleetAssemblies: map[int32]*v1.FleetAssembly{1: {Name: "Explorer Wing", RallyPlanetNum: int32(yard.Num), Designs: map[int32]int32{int32(scout.Tokens[0].DesignNum): 2}, SourceFleets: map[int32]bool{int32(scout.Num): true}, BuildMissing: true, Revision: 1}}}
	send := func() error {
		_, err := svc.UpdatePlayerOrders(ctx, connect.NewRequest(&v1.UpdatePlayerOrdersRequest{GameId: full.ID, Orders: orders}))
		return err
	}
	orders.Automation.FleetAssemblies[1].CompletedRevision = 1
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(send()))
	orders.Automation.FleetAssemblies[1].CompletedRevision = 0
	require.NoError(t, send())
	for range 4 {
		_, err = gr.GenerateTurn(full.ID)
		require.NoError(t, err)
		saved, err := c.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
		require.NoError(t, err)
		if saved.Automation.FleetAssemblies[1].CompletedRevision == 1 {
			break
		}
	}
	saved, err := c.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	plan := saved.Automation.FleetAssemblies[1]
	require.Equal(t, 1, plan.CompletedRevision)
	require.Equal(t, 1, plan.NotifiedRevision)
	require.Positive(t, plan.LeaderFleetNum)
	fleets, err := c.GetFleetsForPlayer(ctx, full.ID, p.Num)
	require.NoError(t, err)
	found := false
	for _, fleet := range fleets {
		if fleet.Num == plan.LeaderFleetNum {
			require.Equal(t, 2, fleet.Spec.TotalShips)
			found = true
		}
	}
	require.True(t, found)
	// A stale pre-turn request must not overwrite server-owned progress.
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(send()))
}
