package cs

import (
	"fmt"
	"sync"
)

// Planet estimates simulate up to 100 future years. They read already-computed
// player/design/rules state and mutate only their own planet. Keep battles,
// actual production, RNG and fleet/design instance counts outside this barrier.
func (g *FullGame) computePlanetSpecs(workers int) error {
	errors := make([]error, len(g.Planets))
	calculate := func(i int) {
		planet := g.Planets[i]
		if !planet.Owned() {
			return
		}
		player := g.getPlayer(planet.PlayerNum)
		// Keep any estimator fallback initialization local to the worker.
		local := *player
		planet.Spec = ComputePlanetSpec(&g.Rules, &local, planet)
		if err := planet.PopulateProductionQueueDesigns(&local); err != nil {
			errors[i] = fmt.Errorf("planet %s unable to populate queue designs: %w", planet.Name, err)
			return
		}
		if err := planet.PopulateProductionQueueEstimates(&g.Rules, &local); err != nil {
			errors[i] = fmt.Errorf("planet %s unable to populate queue estimates: %w", planet.Name, err)
			return
		}
		planet.MarkDirty()
	}
	workers = min(max(1, workers), len(g.Planets))
	if workers <= 1 || len(g.Planets) < 16 {
		for i := range g.Planets {
			calculate(i)
		}
	} else {
		jobs := make(chan int)
		var wg sync.WaitGroup
		for range workers {
			wg.Go(func() {
				for i := range jobs {
					calculate(i)
				}
			})
		}
		for i := range g.Planets {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
	}
	// Stable error ordering regardless of scheduling.
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}
