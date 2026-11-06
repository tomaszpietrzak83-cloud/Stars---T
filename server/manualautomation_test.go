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

func TestManualOrdersAfterMergedFleetHasStaleAssignments(t *testing.T) {
	conn, cfg := connectSubmitTestDB(t)
	c := conn.NewReadWriteClient()
	ctx := t.Context()
	user, err := c.CreateUser(ctx, cs.NewUser("manual-orders", "pw", "", cs.RoleUser))
	require.NoError(t, err)
	full, err := NewGameRunner(conn, cfg).HostGame(user.ID, cs.NewGameSettings().WithHost(cs.Humanoids()))
	require.NoError(t, err)
	p := full.Players[0]
	fleets, err := c.GetFleetsForPlayer(ctx, full.ID, p.Num)
	require.NoError(t, err)
	f := fleets[0]
	p.Automation.FleetRoles = map[int]string{f.Num: "escort", 999: "scout"}
	p.Automation.PatrolMemory = map[string]cs.PatrolContactMemory{"2:8": {Enemy: 2, Target: 8, Seen: 2400, Position: cs.Vector{X: 100}}}
	p.Automation.OccupationSupport = map[int]bool{1: true}
	require.NoError(t, c.UpdatePlayerOrders(ctx, p))
	f.Tags = cs.Tags{"bomber-protection-borrow": `{"role":"patrol"}`, "note": "keep"}
	require.NoError(t, c.SaveFleet(ctx, f))
	game, err := c.GetGame(ctx, full.ID)
	require.NoError(t, err)
	ctx = context.WithValue(ctx, keyDbRead, conn.NewReadClient())
	ctx = context.WithValue(ctx, keyGame, game)
	ctx = context.WithValue(ctx, keyGamePlayer, &game.Players[0])
	_, err = NewPlayerServiceHandler(conn, cfg, newDiscordNotifier(conn, cfg)).UpdatePlayerOrders(ctx, connect.NewRequest(&v1.UpdatePlayerOrdersRequest{GameId: full.ID, Orders: &v1.PlayerOrders{Automation: &v1.LogisticsAutomation{FleetRoles: map[int32]string{999: "scout"}, RebuildFleets: map[int32]bool{int32(f.Num): true}}}}))
	require.NoError(t, err)
	saved, err := c.GetFleetByNum(ctx, full.ID, p.Num, f.Num)
	require.NoError(t, err)
	require.Empty(t, saved.GetTag("bomber-protection-borrow"))
	require.Equal(t, "keep", saved.GetTag("note"))
	require.Equal(t, f.FleetOrders, saved.FleetOrders)
	player, err := c.GetLightPlayerForGame(ctx, full.ID, db.GetPlayerParams{PlayerNum: p.Num})
	require.NoError(t, err)
	require.Empty(t, player.Automation.FleetRoles)
	require.Empty(t, player.Automation.RebuildFleets)
	require.Equal(t, p.Automation.PatrolMemory, player.Automation.PatrolMemory, "ordinary client orders cannot erase server-owned contact memory")
	require.Equal(t, p.Automation.OccupationSupport, player.Automation.OccupationSupport)
}
