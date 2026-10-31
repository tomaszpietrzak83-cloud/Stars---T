// Hand-maintained companion to sqlc queries: keep the same ordered row shape.
package generated

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

func (q *Queries) GetPlayersForGameParallel(ctx context.Context, gameID int64) ([]Player, error) {
	rows, err := q.db.QueryContext(ctx, GetPlayersForGame, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []*planetReportJob
	input, finish := planetReportWorkers(runtime.GOMAXPROCS(0))
	defer finish()
	for rows.Next() {
		var i Player
		var report []byte
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.GameID,
			&i.UserID,
			&i.Name,
			&i.Num,
			&i.Ready,
			&i.AiControlled,
			&i.SubmittedTurn,
			&i.Color,
			&i.DefaultHullSet,
			&i.TechLevelsEnergy,
			&i.TechLevelsWeapons,
			&i.TechLevelsPropulsion,
			&i.TechLevelsConstruction,
			&i.TechLevelsElectronics,
			&i.TechLevelsBiotechnology,
			&i.TechLevelsSpentEnergy,
			&i.TechLevelsSpentWeapons,
			&i.TechLevelsSpentPropulsion,
			&i.TechLevelsSpentConstruction,
			&i.TechLevelsSpentElectronics,
			&i.TechLevelsSpentBiotechnology,
			&i.ResearchAmount,
			&i.ResearchSpentLastYear,
			&i.NextResearchField,
			&i.Researching,
			&i.BattlePlans,
			&i.ProductionPlans,
			&i.TransportPlans,
			&i.Relations,
			&i.CargoTransfers,
			&i.DesignMaintenance,
			&i.Automation,
			&i.Messages,
			&i.BattleRecords,
			&i.PlayerIntels,
			&i.ScoreIntels,
			&report,
			&i.FleetIntels,
			&i.ShipDesignIntels,
			&i.MineralPacketIntels,
			&i.MinefieldIntels,
			&i.WormholeIntels,
			&i.MysteryTraderIntels,
			&i.SalvageIntels,
			&i.Race,
			&i.Stats,
			&i.ScoreHistory,
			&i.AchievedVictoryConditions,
			&i.Victor,
			&i.Guest,
			&i.AiDifficulty,
			&i.AcquiredTechs,
			&i.Archived,
			&i.Revision,
		); err != nil {
			return nil, err
		}
		job := &planetReportJob{item: &i, raw: report}
		jobs = append(jobs, job)
		input <- job
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	finish()
	items := make([]Player, len(jobs))
	for i, job := range jobs {
		if job.err != nil {
			return nil, job.err
		}
		items[i] = *job.item
	}
	return items, nil
}

type planetReportJob struct {
	item *Player
	raw  []byte
	err  error
}

// A bounded hand-off overlaps SQLite reading with independent report decoding.
// Raw report blobs are released immediately, instead of retaining every AI's
// entire galaxy JSON until all decoding finishes.
func planetReportWorkers(workers int) (chan *planetReportJob, func()) {
	input := make(chan *planetReportJob)
	var wg sync.WaitGroup
	var once sync.Once
	for range max(1, workers) {
		wg.Go(func() {
			for job := range input {
				if job.raw != nil {
					job.item.PlanetIntels = new(PlanetIntels)
					if err := job.item.PlanetIntels.Scan(job.raw); err != nil {
						job.err = fmt.Errorf("player %d planet reports: %w", job.item.Num, err)
					}
				}
				job.raw = nil
			}
		})
	}
	return input, func() { once.Do(func() { close(input); wg.Wait() }) }
}
func decodePlanetReports(ctx context.Context, items []Player, reports [][]byte, workers int) error {
	input, finish := planetReportWorkers(min(max(1, workers), max(1, len(items))))
	defer finish()
	jobs := make([]*planetReportJob, len(items))
	for i := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		job := &planetReportJob{item: &items[i], raw: reports[i]}
		jobs[i] = job
		input <- job
	}
	finish()
	for _, job := range jobs {
		if job.err != nil {
			return job.err
		}
	}
	return nil
}
