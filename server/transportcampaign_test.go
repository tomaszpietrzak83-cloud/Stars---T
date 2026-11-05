package server

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sirgwain/craig-stars/config"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	"github.com/stretchr/testify/require"
)

// Optional private campaign: the supplied audit backup is copied AGAIN to a
// temporary database. Never point this test at the player's database.
func TestCopiedTransportCampaign(t *testing.T) {
	source := os.Getenv("STARS_TRANSPORT_CAMPAIGN_DB")
	if source == "" {
		t.Skip("optional private backup")
	}
	absolute, err := filepath.Abs(source)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/")
	raw, err := os.ReadFile(absolute)
	require.NoError(t, err)
	beforeHash := sha256.Sum256(raw)
	destination := filepath.Join(t.TempDir(), "campaign.db")
	require.NoError(t, os.WriteFile(destination, raw, 0600))
	out, err := filepath.Abs(os.Getenv("STARS_TRANSPORT_CAMPAIGN_OUT"))
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(out)), "/audit-results/")
	require.NoError(t, os.MkdirAll(out, 0700))
	turns, err := strconv.Atoi(os.Getenv("STARS_TRANSPORT_CAMPAIGN_TURNS"))
	require.NoError(t, err)
	require.Greater(t, turns, 0)
	require.LessOrEqual(t, turns, 30)
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(old)
	cfg := config.Config{}
	cfg.Database.Filename = destination
	cfg.Database.SkipUpgrade = true
	cfg.Database.ReadConnectionParams = "?_txlock=deferred"
	cfg.Database.WriteConnectionParams = "?_txlock=immediate&_busy_timeout=1200"
	conn := db.NewConn()
	require.NoError(t, conn.Connect(context.Background(), &cfg))
	defer conn.Close()
	gameID := int64(3)
	if value := os.Getenv("STARS_TRANSPORT_CAMPAIGN_GAME"); value != "" {
		gameID, err = strconv.ParseInt(value, 10, 64)
		require.NoError(t, err)
	}
	runner := NewGameRunner(conn, cfg)
	g, err := conn.NewReadClient().GetFullGame(t.Context(), gameID)
	require.NoError(t, err)
	year := g.Year
	owners := map[int]int{}
	for _, planet := range g.Planets {
		owners[planet.Num] = planet.PlayerNum
	}
	captures := 0
	for step := 0; step <= turns; step++ {
		if step > 0 {
			_, err = runner.GenerateTurn(gameID)
			require.NoError(t, err)
			g, err = conn.NewReadClient().GetFullGame(t.Context(), gameID)
			require.NoError(t, err)
			for _, planet := range g.Planets {
				if planet.PlayerNum == 1 && owners[planet.Num] != 0 && owners[planet.Num] != 1 {
					captures++
					t.Logf("RECAPTURE year=%d planet=%d name=%s formerOwner=%d population=%d", g.Year, planet.Num, planet.Name, owners[planet.Num], planet.GetPopulation())
				}
				owners[planet.Num] = planet.PlayerNum
			}
		}
		require.Equal(t, year+step, g.Year)
		file, err := os.Create(filepath.Join(out, strconv.Itoa(g.Year)+".json.gz"))
		require.NoError(t, err)
		// Match the regular archive: AI report caches are not physical state and
		// serializing all of them would dominate the campaign's time and memory.
		exported := *g
		exported.Players = make([]*cs.Player, len(g.Players))
		for i, original := range g.Players {
			local := *original
			if local.AIControlled {
				local.Intels = cs.Intels{}
			}
			exported.Players[i] = &local
		}
		gz, err := gzip.NewWriterLevel(file, gzip.BestSpeed)
		require.NoError(t, err)
		require.NoError(t, json.NewEncoder(gz).Encode(&exported))
		require.NoError(t, gz.Close())
		require.NoError(t, file.Close())
		colonies, pop, aboard, freighters := 0, 0, 0, 0
		for _, p := range g.Planets {
			if p.PlayerNum == 1 {
				colonies++
				pop += p.Cargo.Colonists * 100
			}
		}
		for _, f := range g.Fleets {
			if f.PlayerNum == 1 && !f.Delete {
				aboard += f.Cargo.Colonists * 100
				for _, p := range g.Players {
					if p.Num == 1 && p.Automation.FleetRoles[f.Num] == "freighter" {
						freighters++
					}
				}
			}
		}
		t.Logf("CAMPAIGN year=%d colonies=%d surfacePopulation=%d aboardAllFleets=%d freighters=%d", g.Year, colonies, pop, aboard, freighters)
	}
	after, err := os.ReadFile(absolute)
	require.NoError(t, err)
	require.Equal(t, beforeHash, sha256.Sum256(after), "audit input must remain unchanged")
	if os.Getenv("STARS_REQUIRE_RECAPTURES") == "1" {
		require.Positive(t, captures, "the copied campaign must actually capture a hostile colony")
	}
}
