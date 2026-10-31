package server

import (
	"compress/gzip"
	"encoding/json"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestTurnArchivesPreserveOrdersAndResultsWithoutMutatingGame(t *testing.T) {
	gr := &gameRunner{}
	gr.config.Database.Filename = filepath.Join(t.TempDir(), "data.db")
	gr.config.Game.TurnArchives = true
	game := &cs.FullGame{Game: &cs.Game{DBObject: cs.DBObject{ID: 7}, Year: 2450}, Universe: &cs.Universe{}}
	game.Players = []*cs.Player{{Num: 1, Intels: cs.Intels{PlanetIntels: []*cs.Planet{{MapObject: cs.MapObject{Name: "Human report"}}}}}, {Num: 2, AIControlled: true, Intels: cs.Intels{PlanetIntels: []*cs.Planet{{MapObject: cs.MapObject{Name: "AI cache"}}}}}}
	before, _ := json.Marshal(game)
	require.NoError(t, gr.archiveTurn(game, "orders"))
	require.NoError(t, gr.archiveTurn(game, "orders"))
	after, _ := json.Marshal(game)
	require.Equal(t, string(before), string(after))
	game.Year++
	require.NoError(t, gr.archiveTurn(game, "result"))
	files, err := filepath.Glob(filepath.Join(filepath.Dir(gr.config.Database.Filename), "turn-archives/game-7/*.json.gz"))
	require.NoError(t, err)
	require.Len(t, files, 3)
	f, err := os.Open(files[0])
	require.NoError(t, err)
	defer f.Close()
	z, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer z.Close()
	var saved struct {
		Format, Phase string
		Game          cs.FullGame
	}
	require.NoError(t, json.NewDecoder(z).Decode(&saved))
	require.Equal(t, 2450, saved.Game.Year)
	require.Equal(t, "orders", saved.Phase)
	require.Len(t, saved.Game.Players[0].PlanetIntels, 1)
	require.Empty(t, saved.Game.Players[1].PlanetIntels)
	require.Len(t, game.Players[1].PlanetIntels, 1)
}
