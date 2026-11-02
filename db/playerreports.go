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

// Report encoding reads only PlanetIntels. During UpdateFullGame those reports
// are immutable; player IDs/designs and SQL writes remain on the caller goroutine.
// The unbuffered hand-off bounds live batches to the current eight and the next
// eight players. stop joins all workers even if a database write fails early.
type playerReportBatch struct {
	reports []driver.Value
	err     error
}

func prefetchPlayerReports(players []*cs.Player) (<-chan playerReportBatch, func()) {
	output := make(chan playerReportBatch)
	done := make(chan struct{})
	var wg sync.WaitGroup
	var once sync.Once
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(output)
		for start := 0; start < len(players); start += 8 {
			select {
			case <-done:
				return
			default:
			}
			reports, err := encodePlayerReports(players[start:min(start+8, len(players))])
			select {
			case output <- playerReportBatch{reports, err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return output, func() {
		once.Do(func() { close(done) })
		wg.Wait()
	}
}
