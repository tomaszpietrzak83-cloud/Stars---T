package server

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	v1 "github.com/sirgwain/craig-stars/proto/gen/craig_stars/v1"
	"github.com/stretchr/testify/require"
	"testing"
)

func productionAPIFixture(t *testing.T) (DBConnection, context.Context, *cs.FullGame, int) {
	t.Helper()
	database, cfg := connectSubmitTestDB(t)
	c := database.NewReadWriteClient()
	ctx := t.Context()
	user, err := c.CreateUser(ctx, cs.NewUser("production-test", "test", "", cs.RoleUser))
	require.NoError(t, err)
	g, err := NewGameRunner(database, cfg).HostGame(user.ID, cs.NewGameSettings().WithHost(cs.Humanoids()))
	require.NoError(t, err)
	var home *cs.Planet
	for _, planet := range g.Planets {
		if planet.OwnedBy(g.Players[0].Num) && planet.Starbase != nil {
			home = planet
			break
		}
	}
	require.NotNil(t, home)
	data, err := json.Marshal(home)
	require.NoError(t, err)
	var second cs.Planet
	require.NoError(t, json.Unmarshal(data, &second))
	baseData, err := json.Marshal(home.Starbase)
	require.NoError(t, err)
	second.Starbase = &cs.Fleet{}
	require.NoError(t, json.Unmarshal(baseData, second.Starbase))
	second.ID = 0
	second.Num = 99999
	second.Name = "Second production base"
	second.Position = home.Position.Add(cs.Vector{X: 20, Y: 20})
	second.Homeworld = false
	second.ProductionQueue = nil
	second.Starbase.ID = 0
	second.Starbase.Num = second.Num
	second.Starbase.Position = second.Position
	second.Starbase.OrbitingPlanetNum = second.Num
	second.Starbase.PlanetNum = second.Num
	require.NoError(t, c.SavePlanet(ctx, &second))
	require.NoError(t, c.SaveFleet(ctx, second.Starbase))
	game, err := c.GetGame(ctx, g.ID)
	require.NoError(t, err)
	ctx = context.WithValue(ctx, keyDbRead, database.NewReadClient())
	ctx = context.WithValue(ctx, keyDbWrite, c)
	ctx = context.WithValue(ctx, keyGame, game)
	ctx = context.WithValue(ctx, keyGamePlayer, &game.Players[0])
	design := 0
	for _, d := range g.Players[0].Designs {
		if !d.Spec.Starbase && d.Spec.FuelCapacity > 0 && d.Spec.Cost.Resources > 0 {
			design = d.Num
			break
		}
	}
	require.Positive(t, design)
	return database, ctx, g, design
}

func productionQueues(t *testing.T, c db.ReadClient, ctx context.Context, id int64) string {
	t.Helper()
	planets, err := c.GetPlanetsForPlayer(ctx, id, 1)
	require.NoError(t, err)
	q := map[int]cs.PlanetOrders{}
	for _, p := range planets {
		q[p.Num] = p.PlanetOrders
	}
	b, err := json.Marshal(q)
	require.NoError(t, err)
	return string(b)
}

func TestShipProductionAPIPreviewPersistenceAndStaleRejection(t *testing.T) {
	database, ctx, g, design := productionAPIFixture(t)
	c := database.NewReadWriteClient()
	service := NewPlanetServiceHandler(database)
	before := productionQueues(t, c, ctx, g.ID)
	req := &v1.ShipProductionRequest{GameId: g.ID, DesignNum: int32(design), Quantity: cs.MaxShipProductionOrder, AutoRole: "tanker", FillThisYear: true}
	preview, err := service.PreviewShipProduction(ctx, connect.NewRequest(req))
	require.NoError(t, err)
	require.Len(t, preview.Msg.Yards, 2)
	require.Positive(t, preview.Msg.AssignedQuantity)
	require.JSONEq(t, before, productionQueues(t, c, ctx, g.ID), "preview is read-only")
	req.FillThisYear = false
	req.Quantity = preview.Msg.AssignedQuantity
	req.ExpectedSourceHash = preview.Msg.SourceHash
	applied, err := service.QueueShipProduction(ctx, connect.NewRequest(req))
	require.NoError(t, err)
	require.Equal(t, req.Quantity, applied.Msg.AssignedQuantity)
	stored, err := c.GetPlanetsForPlayer(ctx, g.ID, 1)
	require.NoError(t, err)
	yards := 0
	for _, p := range stored {
		for _, item := range p.ProductionQueue {
			if item.GetTag(cs.ProductionAutoRoleTag) == "tanker" {
				yards++
				require.Equal(t, 1, item.YearsToBuildAll)
			}
		}
	}
	require.Equal(t, 2, yards)
	_, err = service.QueueShipProduction(ctx, connect.NewRequest(req))
	require.Equal(t, connect.CodeAborted, connect.CodeOf(err), "repeating apply must not duplicate orders")
	reloaded, err := c.GetGame(ctx, g.ID)
	require.NoError(t, err)
	require.Equal(t, 2400, reloaded.Year, "queueing never generates a turn")
}

type failProductionClient struct {
	db.Client
	count int
}

func (c *failProductionClient) SavePlanet(ctx context.Context, p *cs.Planet) error {
	c.count++
	if c.count == 2 {
		return fmt.Errorf("simulated second-yard save failure")
	}
	return c.Client.SavePlanet(ctx, p)
}

type failProductionDB struct{ DBConnection }

func (d failProductionDB) WrapInTransaction(fn func(db.Client) error) error {
	return d.DBConnection.WrapInTransaction(func(c db.Client) error { return fn(&failProductionClient{Client: c}) })
}

func TestShipProductionAPIRollsBackEveryYardOnSaveFailure(t *testing.T) {
	database, ctx, g, design := productionAPIFixture(t)
	c := database.NewReadWriteClient()
	service := NewPlanetServiceHandler(failProductionDB{database})
	before := productionQueues(t, c, ctx, g.ID)
	req := &v1.ShipProductionRequest{GameId: g.ID, DesignNum: int32(design), Quantity: cs.MaxShipProductionOrder, FillThisYear: true}
	preview, err := service.PreviewShipProduction(ctx, connect.NewRequest(req))
	require.NoError(t, err)
	req.Quantity = preview.Msg.AssignedQuantity
	req.FillThisYear = false
	req.ExpectedSourceHash = preview.Msg.SourceHash
	_, err = service.QueueShipProduction(ctx, connect.NewRequest(req))
	require.ErrorContains(t, err, "second-yard save failure")
	require.JSONEq(t, before, productionQueues(t, c, ctx, g.ID))
}
