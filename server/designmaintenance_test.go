package server

import (
	"connectrpc.com/connect"
	"context"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	v1 "github.com/sirgwain/craig-stars/proto/gen/craig_stars/v1"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDesignMaintenanceOrdersAPI(t *testing.T) {
	dbConn, cfg := connectSubmitTestDB(t)
	c := dbConn.NewReadWriteClient()
	ctx := t.Context()
	user, err := c.CreateUser(ctx, cs.NewUser("maintenance", "pw", "", cs.RoleUser))
	require.NoError(t, err)
	gr := NewGameRunner(dbConn, cfg)
	full, err := gr.HostGame(user.ID, cs.NewGameSettings().WithHost(cs.Humanoids()))
	require.NoError(t, err)
	p := full.Players[0]
	var old *cs.ShipDesign
	for _, design := range p.Designs {
		if !design.Spec.Starbase {
			old = design
			break
		}
	}
	require.NotNil(t, old)
	target := *old
	target.ID, target.Num, target.Name = 0, p.GetNextDesignNum(p.Designs), "Sword2"
	require.NoError(t, c.SaveShipDesign(ctx, &target))
	game, err := c.GetGame(ctx, full.ID)
	require.NoError(t, err)
	ctx = context.WithValue(ctx, keyDbRead, dbConn.NewReadClient())
	ctx = context.WithValue(ctx, keyGame, game)
	ctx = context.WithValue(ctx, keyGamePlayer, &game.Players[0])
	svc := NewPlayerServiceHandler(dbConn, cfg, newDiscordNotifier(dbConn, cfg))
	originalFleets, err := c.GetFleetsForPlayer(ctx, full.ID, p.Num)
	require.NoError(t, err)
	requests := map[int32]bool{}
	for _, fleet := range originalFleets {
		for _, token := range fleet.Tokens {
			if token.DesignNum == old.Num {
				requests[int32(fleet.Num)] = true
			}
		}
	}
	request := func(targetNum int) *connect.Request[v1.UpdatePlayerOrdersRequest] {
		return connect.NewRequest(&v1.UpdatePlayerOrdersRequest{GameId: full.ID, Orders: &v1.PlayerOrders{DesignMaintenance: map[int32]int32{int32(old.Num): int32(targetNum)}, Automation: &v1.LogisticsAutomation{RebuildFleets: requests}}})
	}
	_, err = svc.UpdatePlayerOrders(ctx, request(99999))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	response, err := svc.UpdatePlayerOrders(ctx, request(target.Num))
	require.NoError(t, err)
	require.Equal(t, int32(target.Num), response.Msg.Player.PlayerOrders.DesignMaintenance[int32(old.Num)])
	saved, err := c.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	require.Equal(t, cs.DesignMaintenance{old.Num: target.Num}, saved.DesignMaintenance)
	_, err = gr.GenerateTurn(full.ID)
	require.NoError(t, err)
	fleets, err := c.GetFleetsForPlayer(ctx, full.ID, p.Num)
	require.NoError(t, err)
	converted := 0
	for _, fleet := range fleets {
		for _, token := range fleet.Tokens {
			if token.DesignNum == target.Num {
				converted += token.Quantity
			}
		}
	}
	require.Positive(t, converted, "modernized ships must persist after generating and reloading a turn")
	_, err = NewshipDesignServiceHandler(dbConn).DeleteShipDesign(ctx, connect.NewRequest(&v1.DeleteShipDesignRequest{GameId: full.ID, Num: int32(target.Num)}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	_, err = svc.UpdatePlayerOrders(ctx, connect.NewRequest(&v1.UpdatePlayerOrdersRequest{GameId: full.ID, Orders: &v1.PlayerOrders{}}))
	require.NoError(t, err)
	saved, err = c.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	require.Empty(t, saved.DesignMaintenance)
}

func TestDeleteUnusedSupersededDesign(t *testing.T) {
	conn, cfg := connectSubmitTestDB(t)
	c := conn.NewReadWriteClient()
	ctx := t.Context()
	user, err := c.CreateUser(ctx, cs.NewUser("empty-design", "pw", "", cs.RoleUser))
	require.NoError(t, err)
	full, err := NewGameRunner(conn, cfg).HostGame(user.ID, cs.NewGameSettings().WithHost(cs.Humanoids()))
	require.NoError(t, err)
	p := full.Players[0]
	var base *cs.ShipDesign
	for _, d := range p.Designs {
		if !d.Spec.Starbase && !d.CannotDelete {
			base = d
			break
		}
	}
	require.NotNil(t, base)
	old := *base
	old.ID = 0
	old.Num = p.GetNextDesignNum(p.Designs)
	old.Name = "Unused Mk 1"
	require.NoError(t, c.SaveShipDesign(ctx, &old))
	p.Designs = append(p.Designs, &old)
	next := old
	next.ID = 0
	next.Num = p.GetNextDesignNum(p.Designs)
	next.Name = "Unused Mk 2"
	require.NoError(t, c.SaveShipDesign(ctx, &next))
	p.DesignMaintenance = cs.DesignMaintenance{old.Num: next.Num}
	require.NoError(t, c.UpdatePlayerOrders(ctx, p))
	game, err := c.GetGame(ctx, full.ID)
	require.NoError(t, err)
	ctx = context.WithValue(ctx, keyDbRead, conn.NewReadClient())
	ctx = context.WithValue(ctx, keyGame, game)
	ctx = context.WithValue(ctx, keyGamePlayer, &game.Players[0])
	svc := NewshipDesignServiceHandler(conn)
	// A replacement target stays protected even when it has zero ships.
	_, err = svc.DeleteShipDesign(ctx, connect.NewRequest(&v1.DeleteShipDesignRequest{GameId: full.ID, Num: int32(next.Num)}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	planets, err := c.GetPlanetsForPlayer(ctx, full.ID, p.Num)
	require.NoError(t, err)
	planets[0].ProductionQueue = []cs.ProductionQueueItem{{Type: cs.QueueItemTypeShipToken, DesignNum: old.Num, Quantity: 1}}
	require.NoError(t, c.SavePlanet(ctx, planets[0]))
	_, err = svc.DeleteShipDesign(ctx, connect.NewRequest(&v1.DeleteShipDesignRequest{GameId: full.ID, Num: int32(old.Num)}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	planets[0].ProductionQueue = nil
	require.NoError(t, c.SavePlanet(ctx, planets[0]))
	_, err = svc.DeleteShipDesign(ctx, connect.NewRequest(&v1.DeleteShipDesignRequest{GameId: full.ID, Num: int32(old.Num)}))
	require.NoError(t, err)
	saved, err := c.GetShipDesignByNum(ctx, full.ID, p.Num, old.Num)
	require.NoError(t, err)
	require.Nil(t, saved)
	player, err := c.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	require.NotContains(t, player.DesignMaintenance, old.Num)
}
