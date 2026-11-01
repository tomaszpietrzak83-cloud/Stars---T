// Hand-maintained companion: identical SQL and parameter order.
package generated

import (
	"context"
	"database/sql/driver"
)

func (q *Queries) UpdatePlayerEncodedReports(ctx context.Context, arg UpdatePlayerParams, reports driver.Value) (int64, error) {
	result, err := q.db.ExecContext(ctx, UpdatePlayer,
		arg.GameID,
		arg.UserID,
		arg.Name,
		arg.Num,
		arg.Ready,
		arg.AiControlled,
		arg.SubmittedTurn,
		arg.Color,
		arg.DefaultHullSet,
		arg.TechLevelsEnergy,
		arg.TechLevelsWeapons,
		arg.TechLevelsPropulsion,
		arg.TechLevelsConstruction,
		arg.TechLevelsElectronics,
		arg.TechLevelsBiotechnology,
		arg.TechLevelsSpentEnergy,
		arg.TechLevelsSpentWeapons,
		arg.TechLevelsSpentPropulsion,
		arg.TechLevelsSpentConstruction,
		arg.TechLevelsSpentElectronics,
		arg.TechLevelsSpentBiotechnology,
		arg.ResearchAmount,
		arg.ResearchSpentLastYear,
		arg.NextResearchField,
		arg.Researching,
		arg.BattlePlans,
		arg.ProductionPlans,
		arg.TransportPlans,
		arg.Relations,
		arg.Automation,
		arg.DesignMaintenance,
		arg.CargoTransfers,
		arg.Messages,
		arg.BattleRecords,
		arg.PlayerIntels,
		arg.ScoreIntels,
		reports,
		arg.FleetIntels,
		arg.ShipDesignIntels,
		arg.MineralPacketIntels,
		arg.MinefieldIntels,
		arg.WormholeIntels,
		arg.MysteryTraderIntels,
		arg.SalvageIntels,
		arg.Race,
		arg.Stats,
		arg.ScoreHistory,
		arg.AchievedVictoryConditions,
		arg.Victor,
		arg.Guest,
		arg.AiDifficulty,
		arg.AcquiredTechs,
		arg.Archived,
		arg.ID,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
