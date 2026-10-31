package server

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	"github.com/sirgwain/craig-stars/db"
)

func checkDesignMaintenanceLock(ctx context.Context, num int, deleting ...bool) error {
	game, seat := contextGame(ctx), contextGamePlayer(ctx)
	player, err := contextDb(ctx).GetLightPlayerForGame(ctx, game.ID, db.GetPlayerParams{PlayerNum: seat.Num})
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("load maintenance orders"))
	}
	// Only an unused superseded source may be removed while a family rebuild is
	// configured. Targets, physical ships and paid production stay protected.
	unusedSource := false
	if len(deleting) > 0 && deleting[0] && player.DesignMaintenance[num] > 0 {
		fleets, err := contextDb(ctx).GetFleetsForPlayer(ctx, game.ID, seat.Num)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		planets, err := contextDb(ctx).GetPlanetsForPlayer(ctx, game.ID, seat.Num)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		unusedSource = true
		for _, fleet := range fleets {
			for _, token := range fleet.Tokens {
				if token.DesignNum == num && token.Quantity > 0 {
					unusedSource = false
				}
			}
		}
		for _, planet := range planets {
			for _, item := range planet.ProductionQueue {
				if item.DesignNum == num && item.Quantity > 0 {
					unusedSource = false
				}
			}
		}
	}
	for source, target := range player.DesignMaintenance {
		if (source == num && !unusedSource) || target == num {
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("clear maintenance orders before editing or deleting this design"))
		}
	}
	for _, plan := range player.Automation.FleetAssemblies {
		if plan.CompletedRevision != plan.Revision && plan.Designs[num] > 0 {
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("finish or cancel the fleet composition before editing or deleting this design"))
		}
	}
	return nil
}
