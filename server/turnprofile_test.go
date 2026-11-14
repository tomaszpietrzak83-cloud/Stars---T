package server

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	httpprof "net/http/pprof"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirgwain/craig-stars/config"
	"github.com/sirgwain/craig-stars/db"
	"github.com/stretchr/testify/require"
)

// Opt-in diagnostic. Only an audit backup is accepted, and only a second,
// temporary copy is connected to the database client. Outputs are private.
func TestTurnProfileOnCopiedBackup(t *testing.T) {
	source := os.Getenv("STARS_PROFILE_BACKUP")
	if source == "" {
		t.Skip("optional copied-backup benchmark")
	}
	absolute, err := filepath.Abs(source)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/")
	raw, err := os.ReadFile(absolute)
	require.NoError(t, err)
	hash := sha256.Sum256(raw)
	destination := filepath.Join(t.TempDir(), "profile.db")
	if retained := os.Getenv("STARS_PROFILE_DATABASE"); retained != "" {
		destination, err = filepath.Abs(retained)
		require.NoError(t, err)
		require.Contains(t, strings.ToLower(filepath.ToSlash(destination)), "/audit-results/")
		_, err := os.Stat(destination)
		require.True(t, os.IsNotExist(err), "never overwrite an existing retained database")
	}
	require.NoError(t, os.WriteFile(destination, raw, 0600))
	raw = nil
	t.Cleanup(func() {
		data, err := os.ReadFile(absolute)
		require.NoError(t, err)
		require.Equal(t, hash, sha256.Sum256(data), "input backup must remain unchanged")
	})
	out, err := filepath.Abs(os.Getenv("STARS_PROFILE_OUT"))
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(out)), "/audit-results/")
	require.NoError(t, os.MkdirAll(out, 0700))
	logFile, err := os.Create(filepath.Join(out, "phases.log"))
	require.NoError(t, err)
	defer logFile.Close()
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, nil)))
	defer slog.SetDefault(old)
	cfg := config.Config{}
	cfg.Database.Filename = destination
	cfg.Database.SkipUpgrade = true
	cfg.Game.TurnArchives = true
	cfg.Database.ReadConnectionParams = "?_txlock=deferred"
	cfg.Database.WriteConnectionParams = "?_txlock=immediate&_busy_timeout=1200"
	conn := db.NewConn()
	require.NoError(t, conn.Connect(t.Context(), &cfg))
	defer conn.Close()
	id, err := strconv.ParseInt(os.Getenv("STARS_PROFILE_GAME"), 10, 64)
	require.NoError(t, err)
	before, err := conn.NewReadClient().GetGame(t.Context(), id)
	require.NoError(t, err)
	require.NotNil(t, before)
	if port := os.Getenv("STARS_PROFILE_PORT"); port != "" {
		listener, err := net.Listen("tcp", "127.0.0.1:"+port)
		require.NoError(t, err)
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/profile", httpprof.Profile)
		mux.Handle("/debug/pprof/goroutine", httpprof.Handler("goroutine"))
		server := &http.Server{Handler: mux}
		go server.Serve(listener)
		defer server.Close()
	}
	var profile *os.File
	var stopProfile func()
	if os.Getenv("STARS_PROFILE_CPU") != "" {
		profile, err = os.Create(filepath.Join(out, "turn.cpu"))
		require.NoError(t, err)
		var mu sync.Mutex
		active, stopped := false, false
		startProfile := func() {
			mu.Lock()
			defer mu.Unlock()
			if !stopped {
				if err := pprof.StartCPUProfile(profile); err != nil {
					t.Error(err)
					return
				}
				active = true
			}
		}
		stopProfile = func() {
			mu.Lock()
			defer mu.Unlock()
			if !stopped {
				stopped = true
				if active {
					pprof.StopCPUProfile()
				}
				profile.Close()
			}
		}
		defer stopProfile()
		delay := time.Duration(0)
		if raw := os.Getenv("STARS_PROFILE_DELAY"); raw != "" {
			delay, err = time.ParseDuration(raw)
			require.NoError(t, err)
			require.GreaterOrEqual(t, delay, time.Duration(0))
		}
		if delay > 0 {
			timer := time.AfterFunc(delay, startProfile)
			defer timer.Stop()
		} else {
			startProfile()
		}
		// A bounded capture remains readable even if a pathological turn runs
		// for many minutes. It never interrupts the simulated turn.
		if duration := os.Getenv("STARS_PROFILE_DURATION"); duration != "" {
			limit, err := time.ParseDuration(duration)
			require.NoError(t, err)
			require.Positive(t, limit)
			timer := time.AfterFunc(delay+limit, stopProfile)
			defer timer.Stop()
		}
	}
	started := time.Now()
	result, err := NewGameRunner(conn, cfg).GenerateTurn(id)
	elapsed := time.Since(started)
	if profile != nil {
		stopProfile()
	}
	require.NoError(t, err)
	require.Equal(t, TurnGenerated, result)
	t.Logf("GenerateTurn: %s", elapsed)
	after, err := conn.NewReadClient().GetFullGame(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, before.Year+1, after.Year)
	// Stream individual records to bound memory; retain every player's reports.
	file, err := os.Create(filepath.Join(out, "state.jsonl.gz"))
	require.NoError(t, err)
	defer file.Close()
	z := gzip.NewWriter(file)
	defer z.Close()
	enc := json.NewEncoder(z)
	write := func(label string, value any) {
		_, err := io.WriteString(z, label+"\n")
		require.NoError(t, err)
		require.NoError(t, enc.Encode(value))
	}
	write("game", after.Game)
	for _, p := range after.Players {
		write("player", p)
	}
	for _, f := range after.Fleets {
		write("fleet", f)
	}
	for _, p := range after.Planets {
		write("planet", p)
	}
	for _, f := range after.Starbases {
		write("starbase", f)
	}
	write("wormholes", after.Wormholes)
	write("salvages", after.Salvages)
	write("minefields", after.Minefields)
	write("packets", after.MineralPackets)
	write("traders", after.MysteryTraders)
}
