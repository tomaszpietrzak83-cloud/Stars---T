//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExpandedMapsGenerateProportionalEmpirePopulations(t *testing.T) {
	for _, tc := range []struct {
		size           Size
		width, empires int
		density        Density
	}{{SizeVast, 2800, 32, DensitySparse}, {SizeImmense, 3200, 40, DensitySparse}, {SizeColossal, 4000, 64, DensitySparse}, {SizeVast, 2800, 32, DensityPacked}, {SizeImmense, 3200, 40, DensityPacked}, {SizeColossal, 4000, 64, DensityPacked}} {
		t.Run(string(tc.size)+string(tc.density), func(t *testing.T) {
			client := NewGamer()
			game := newSeededGame(*NewGameSettings().WithSize(tc.size).WithDensity(tc.density))
			if tc.density == DensityPacked {
				game.Seed = 17
				game.Rules.ResetSeed(17)
			}
			area, err := game.Rules.GetArea(tc.size)
			require.NoError(t, err)
			require.Equal(t, Vector{tc.width, tc.width}, area)
			players := make([]*Player, tc.empires)
			for i := range players {
				players[i] = client.NewPlayer(int64(i+1), *NewRace(), &game.Rules).WithNum(i + 1)
			}
			universe, err := client.GenerateUniverse(game, players)
			require.NoError(t, err)
			count, err := game.Rules.GetNumPlanets(tc.size, tc.density)
			require.NoError(t, err)
			require.Len(t, universe.Planets, count)
			names := map[string]bool{}
			for _, planet := range universe.Planets {
				require.False(t, names[planet.Name], "duplicate planet name %s", planet.Name)
				names[planet.Name] = true
			}
			positions := map[Vector]bool{}
			for _, player := range players {
				owned := universe.getPlanets(player.Num)
				require.NotEmpty(t, owned)
				for _, planet := range owned {
					require.False(t, positions[planet.Position])
					positions[planet.Position] = true
				}
			}
			require.Positive(t, game.Rules.RandomMineralDepositChanceForSize[tc.size])
			require.Positive(t, game.Rules.WormholePairsForSize[tc.size])
		})
	}
}
