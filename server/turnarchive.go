package server

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/sirgwain/craig-stars/cs"
	"os"
	"path/filepath"
	"time"
)

// Private game state only, never authentication data or the whole server DB.
// The data directory (including these archives) is excluded from publication.
func (gr *gameRunner) archiveTurn(game *cs.FullGame, phase string) error {
	if !gr.config.Game.TurnArchives {
		return nil
	}
	dir := filepath.Join(filepath.Dir(gr.config.Database.Filename), "turn-archives", fmt.Sprintf("game-%d", game.ID))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".turn-*.tmp")
	if err != nil {
		return err
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	// Analysis archives retain the full physical universe, every player's
	// orders/tech/economy, and human reports. Repeating all AI planet-report
	// caches would dominate turn time and disk space without adding physical data.
	exported := *game
	exported.Players = make([]*cs.Player, len(game.Players))
	for i, original := range game.Players {
		local := *original
		if local.AIControlled {
			local.Intels = cs.Intels{}
		}
		exported.Players[i] = &local
	}
	z, _ := gzip.NewWriterLevel(f, gzip.BestSpeed)
	err = json.NewEncoder(z).Encode(struct {
		Format     string
		Phase      string
		RecordedAt time.Time
		Game       *cs.FullGame
	}{"tomek-stars-turn-v1", phase, time.Now().UTC(), &exported})
	if closeErr := z.Close(); err == nil {
		err = closeErr
	}
	if syncErr := f.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	name := fmt.Sprintf("year-%d-%s-%s.json.gz", game.Year, phase, time.Now().UTC().Format("20060102T150405.000000000Z"))
	return os.Rename(temporary, filepath.Join(dir, name))
}
