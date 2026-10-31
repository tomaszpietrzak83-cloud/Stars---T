package server

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	"github.com/sirgwain/craig-stars/db"
)

func checkDesignMaintenanceLock(ctx context.Context, num int) error {
	game, seat := contextGame(ctx), contextGamePlayer(ctx)
	player, err := contextDb(ctx).GetLightPlayerForGame(ctx, game.ID, db.GetPlayerParams{PlayerNum: seat.Num})
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("load maintenance orders"))
	}
	for source, target := range player.DesignMaintenance {
		if source == num || target == num {
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
