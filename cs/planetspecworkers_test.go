//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func planetWorkerScenario(n int) TestScenario {
	planets := make([]ScenarioPlanet, n)
	for i := range planets {
		planets[i] = ScenarioPlanet{Name: fmt.Sprintf("Colony %d", i), Owner: 1, Cargo: Cargo{Colonists: 1500, Germanium: 20}}
	}
	return TestScenario{Players: []ScenarioPlayer{{}}, Planets: planets}
}
func prepareWorkerGame(n int) *FullGame {
	g := BuildScenario(planetWorkerScenario(n))
	for _, p := range g.Planets {
		p.Factories = 100
		p.Mines = 100
		p.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeAutoFactories, Quantity: 250}, {Type: QueueItemTypeAutoMines, Quantity: 250}}
	}
	if err := g.computeSpecs(); err != nil {
		panic(err)
	}
	return g
}
func TestParallelPlanetForecastsMatchSequentialResults(t *testing.T) {
	serial, parallel := prepareWorkerGame(32), prepareWorkerGame(32)
	require.NoError(t, serial.computePlanetSpecs(1))
	require.NoError(t, parallel.computePlanetSpecs(8))
	for i := range serial.Planets {
		require.Equal(t, serial.Planets[i].Spec, parallel.Planets[i].Spec)
		require.Equal(t, serial.Planets[i].ProductionQueue, parallel.Planets[i].ProductionQueue)
		require.Equal(t, serial.Planets[i].Cargo, parallel.Planets[i].Cargo)
	}
}
func BenchmarkPlanetForecasts(b *testing.B) {
	for _, workers := range []int{1, 8} {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			g := prepareWorkerGame(128)
			b.ResetTimer()
			for b.Loop() {
				if err := g.computePlanetSpecs(workers); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
