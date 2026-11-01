package db

import (
	"context"
	"database/sql/driver"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db/generated"
	"runtime"
	"sync"
)

func encodePlayerReports(players []*cs.Player) ([]driver.Value, error) {
	out := make([]driver.Value, len(players))
	errs := make([]error, len(players))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(8, runtime.GOMAXPROCS(0), len(players)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				reports := generated.PlanetIntels(players[i].PlanetIntels)
				out[i], errs[i] = reports.Value()
			}
		}()
	}
	for i := range players {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (c *client) saveEncodedPlayer(ctx context.Context, p *cs.Player, report driver.Value) error {
	if p.ID == 0 {
		return c.SavePlayer(ctx, p)
	}
	_, err := c.writer.UpdatePlayerEncodedReports(ctx, c.converter.ConvertGamePlayerToUpdateParams(p), report)
	return err
}
