package db

import (
	"encoding/json"
	"github.com/sirgwain/craig-stars/config"
	"github.com/sirgwain/craig-stars/db/generated"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLocalSerialAndParallelReportsOnSnapshot(t *testing.T) {
	source := os.Getenv("STARS_REPORT_BENCH_SNAPSHOT")
	if source == "" {
		t.Skip("optional read-only audit snapshot comparison")
	}
	absolute, err := filepath.Abs(source)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/")
	cfg := config.Config{}
	cfg.Database.Filename = absolute
	cfg.Database.SkipUpgrade = true
	conn := NewConn()
	require.NoError(t, conn.Connect(t.Context(), &cfg))
	defer conn.Close()
	c := conn.NewReadClient().(*client)
	started := time.Now()
	serial, err := c.reader.GetPlayersForGame(t.Context(), 10)
	require.NoError(t, err)
	serialTime := time.Since(started)
	started = time.Now()
	parallel, err := c.reader.GetPlayersForGameParallel(t.Context(), 10)
	require.NoError(t, err)
	parallelTime := time.Since(started)
	require.Equal(t, serial, parallel, "every player column/report and row order must match")
	originalBytes, storedBytes := 0, 0
	for _, player := range serial {
		plain, err := json.Marshal(player.PlanetIntels)
		require.NoError(t, err)
		encoded, err := player.PlanetIntels.Value()
		require.NoError(t, err)
		var decoded generated.PlanetIntels
		require.NoError(t, decoded.Scan(encoded))
		require.Equal(t, player.PlanetIntels, &decoded)
		originalBytes += len(plain)
		storedBytes += len(encoded.([]byte))
	}
	t.Logf("Lossless report storage: %d JSON bytes -> %d stored bytes", originalBytes, storedBytes)
	t.Logf("Report read+decode: serial=%s parallel=%s workers=%d players=%d", serialTime, parallelTime, runtime.GOMAXPROCS(0), len(serial))
}
