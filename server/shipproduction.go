package server

import (
	"connectrpc.com/connect"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	"github.com/sirgwain/craig-stars/proto/converter"
	v1 "github.com/sirgwain/craig-stars/proto/gen/craig_stars/v1"
	"slices"
)

func shipProductionSource(ctx context.Context, c db.ReadClient, g *cs.GameWithPlayers, num int) (*cs.Player, []*cs.Planet, string, error) {
	p, err := c.GetLightPlayerForGameWithDesigns(ctx, g.ID, db.GetPlayerParams{PlayerNum: num})
	if err != nil {
		return nil, nil, "", err
	}
	if p == nil {
		return nil, nil, "", fmt.Errorf("player not found")
	}
	planets, err := c.GetPlanetsForPlayer(ctx, g.ID, num)
	if err != nil {
		return nil, nil, "", err
	}
	fleets, err := c.GetFleetsForPlayer(ctx, g.ID, num)
	if err != nil {
		return nil, nil, "", err
	}
	bases := map[int]*cs.Fleet{}
	for _, f := range fleets {
		if f.Starbase {
			bases[f.PlanetNum] = f
		}
	}
	for _, planet := range planets {
		planet.Starbase = bases[planet.Num]
	}
	slices.SortFunc(planets, func(a, b *cs.Planet) int { return a.Num - b.Num })
	data, err := json.Marshal([]any{g.Year, g.State, p.SubmittedTurn, p.PlayerOrders, p.Race, p.TechLevels, p.Designs, planets, bases})
	if err != nil {
		return nil, nil, "", err
	}
	hash := sha256.Sum256(data)
	return p, planets, hex.EncodeToString(hash[:]), nil
}

func shipProductionResponse(plan cs.EmpireShipProduction, hash string, year int) *v1.ShipProductionResponse {
	response := &v1.ShipProductionResponse{RequestedQuantity: int32(plan.Requested), AssignedQuantity: int32(plan.Assigned), ThisYearQuantity: int32(plan.ThisYear), UnassignedQuantity: int32(plan.Unassigned), AutoRole: plan.AutoRole, SourceHash: hash, Year: int32(year)}
	for _, yard := range plan.Yards {
		response.Yards = append(response.Yards, &v1.ShipyardProduction{Planet: converter.C.ConvertCSPlanet(yard.Planet), CapacityThisYear: int32(yard.Capacity), ThisYearQuantity: int32(yard.ThisYear), LaterQuantity: int32(yard.Later), YearsToComplete: int32(yard.Years), Reason: yard.Reason})
	}
	return response
}

func (s *planetService) PreviewShipProduction(ctx context.Context, req *connect.Request[v1.ShipProductionRequest]) (*connect.Response[v1.ShipProductionResponse], error) {
	g := contextGame(ctx)
	p, planets, hash, err := shipProductionSource(ctx, contextDb(ctx), g, contextGamePlayer(ctx).Num)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	plan, err := cs.PlanEmpireShipProduction(&g.Rules, p, planets, int(req.Msg.DesignNum), int(req.Msg.Quantity), req.Msg.AutoRole, req.Msg.FillThisYear)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(shipProductionResponse(plan, hash, g.Year)), nil
}

func (s *planetService) QueueShipProduction(ctx context.Context, req *connect.Request[v1.ShipProductionRequest]) (*connect.Response[v1.ShipProductionResponse], error) {
	if req.Msg.ExpectedSourceHash == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("preview the ship order before applying it"))
	}
	var response *v1.ShipProductionResponse
	err := s.db.WrapInTransaction(func(c db.Client) error {
		g, err := c.GetGame(ctx, req.Msg.GameId)
		if err != nil {
			return err
		}
		if g == nil {
			return connect.NewError(connect.CodeNotFound, fmt.Errorf("game not found"))
		}
		p, planets, hash, err := shipProductionSource(ctx, c, g, contextGamePlayer(ctx).Num)
		if err != nil {
			return err
		}
		if g.State != cs.GameStateWaitingForPlayers || p.SubmittedTurn {
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("orders cannot be changed during this turn state"))
		}
		if hash != req.Msg.ExpectedSourceHash {
			return connect.NewError(connect.CodeAborted, fmt.Errorf("production changed since the preview; refresh the allocation"))
		}
		plan, err := cs.PlanEmpireShipProduction(&g.Rules, p, planets, int(req.Msg.DesignNum), int(req.Msg.Quantity), req.Msg.AutoRole, req.Msg.FillThisYear)
		if err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		}
		for _, yard := range plan.Yards {
			if yard.ThisYear+yard.Later == 0 {
				continue
			}
			if err := cs.NewOrderer().UpdatePlanetOrders(&g.Rules, p, yard.Planet, yard.Planet.PlanetOrders); err != nil {
				return err
			}
			if err := c.SavePlanet(ctx, yard.Planet); err != nil {
				return err
			}
		}
		response = shipProductionResponse(plan, hash, g.Year)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(response), nil
}
