//go:build !wasi && !wasm

package cs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEdgeStartBandUsesMapWidthOnEverySide(t *testing.T) {
	for _, area := range []Vector{{1000, 1000}, {2000, 1000}, {1000, 2000}} {
		for _, percent := range []int{0, 3, 4, 7, 10, 11} {
			d := area.X * percent / 100
			for _, point := range []Vector{{d, area.Y / 2}, {area.X - d, area.Y / 2}, {area.X / 2, d}, {area.X / 2, area.Y - d}} {
				require.Equal(t, percent >= 4 && percent <= 10, edgeStartScore(point, area) == 0, "area=%v point=%v", area, point)
			}
		}
	}
}

func TestGeneratedEdgeHomeworldStaysInsideBand(t *testing.T) {
	for _, size := range []Size{SizeTiny, SizeTinyWide, SizeSmall} {
		for _, clumping := range []bool{false, true} {
			for seed := range int64(30) {
				game := newSeededGame(*NewGameSettings().WithSize(size).WithDensity(DensitySparse))
				game.HostStartPosition = "edge"
				game.GalaxyClumping = clumping
				game.Rules.ResetSeed(seed)
				players := []*Player{NewPlayer(1, NewRace()), NewPlayer(2, NewRace())}
				players[0].AIControlled = true
				ug := NewUniverseGenerator(game, players).(*universeGenerator)
				var err error
				ug.area, err = game.Rules.GetArea(size)
				require.NoError(t, err)
				ug.Universe = &Universe{}
				require.NoError(t, ug.generatePlanets())
				indices, err := ug.getPlayerPlanets()
				require.NoError(t, err)
				p := ug.Universe.Planets[indices[1]].Position
				d := min(p.X, ug.area.X-p.X, p.Y, ug.area.Y-p.Y)
				require.GreaterOrEqual(t, 100*d, 4*ug.area.X)
				require.LessOrEqual(t, 100*d, 10*ug.area.X)
			}
		}
	}
}
