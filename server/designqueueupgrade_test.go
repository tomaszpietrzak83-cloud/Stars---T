package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/proto/converter"
	v1 "github.com/sirgwain/craig-stars/proto/gen/craig_stars/v1"
	"github.com/stretchr/testify/require"
)

func TestDesignerUpgradeReplacesProductionImmediately(t *testing.T) {
	for _, starbase := range []bool{false, true} {
		t.Run(map[bool]string{false: "ships", true: "starbases"}[starbase], func(t *testing.T) {
			conn, cfg := connectSubmitTestDB(t)
			c, ctx := conn.NewReadWriteClient(), t.Context()
			user, err := c.CreateUser(ctx, cs.NewUser("queue-upgrade", "pw", "", cs.RoleUser))
			require.NoError(t, err)
			full, err := NewGameRunner(conn, cfg).HostGame(user.ID, cs.NewGameSettings().WithHost(cs.Humanoids()))
			require.NoError(t, err)
			p := full.Players[0]
			var old *cs.ShipDesign
			for _, d := range p.Designs {
				if d.Spec.Starbase == starbase {
					old = d
					break
				}
			}
			require.NotNil(t, old)
			planets, err := c.GetPlanetsForPlayer(ctx, full.ID, p.Num)
			require.NoError(t, err)
			require.NotEmpty(t, planets)
			planet := planets[0]
			kind := cs.QueueItemTypeShipToken
			if starbase {
				kind = cs.QueueItemTypeStarbase
			}
			paid := cs.Cost{Ironium: 1, Resources: 2}
			planet.ProductionQueue = []cs.ProductionQueueItem{
				{Type: cs.QueueItemTypeFactory, Quantity: 2},
				{Type: kind, DesignNum: old.Num, Quantity: 3, Allocated: paid, Note: "keep mission", Tags: cs.Tags{"replacement-order": `{"group":"test"}`}},
				{Type: kind, DesignNum: old.Num, Quantity: 5},
			}
			require.NoError(t, c.SavePlanet(ctx, planet))
			game, err := c.GetGame(ctx, full.ID)
			require.NoError(t, err)
			ctx = context.WithValue(ctx, keyDbRead, conn.NewReadClient())
			ctx = context.WithValue(ctx, keyDbWrite, c)
			ctx = context.WithValue(ctx, keyGame, game)
			ctx = context.WithValue(ctx, keyGamePlayer, &game.Players[0])
			svc := NewshipDesignServiceHandler(conn)
			upgrade := func(name string, from int) int {
				draft := converter.C.ConvertCSShipDesign(old)
				draft.Name = name
				resp, err := svc.CreateShipDesign(ctx, connect.NewRequest(&v1.CreateShipDesignRequest{GameId: full.ID, Design: draft, UpgradeFromDesignNum: int32(from)}))
				require.NoError(t, err)
				require.Len(t, resp.Msg.Planets, 1)
				num := int(resp.Msg.Design.Num)
				saved, err := c.GetPlanetByNum(ctx, full.ID, planet.Num)
				require.NoError(t, err)
				require.Len(t, saved.ProductionQueue, 3)
				require.Equal(t, cs.QueueItemTypeFactory, saved.ProductionQueue[0].Type)
				for i, quantity := range []int{3, 5} {
					require.Equal(t, num, saved.ProductionQueue[i+1].DesignNum)
					require.Equal(t, quantity, saved.ProductionQueue[i+1].Quantity)
				}
				require.Equal(t, paid, saved.ProductionQueue[1].Allocated)
				require.Equal(t, "keep mission", saved.ProductionQueue[1].Note)
				require.Equal(t, planet.ProductionQueue[1].Tags, saved.ProductionQueue[1].Tags)
				return num
			}
			upgrade("Queue Mk 2", old.Num)
			// Upgrading an older family member must also replace the queued Mk 2.
			upgrade("Queue Mk 3", old.Num)
		})
	}
}
