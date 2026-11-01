package server

import (
	"context"
	"encoding/json"
	"github.com/sirgwain/craig-stars/config"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLocalAutomationOnCopiedSave(t *testing.T) {
	oldLog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(oldLog) })
	source := os.Getenv("STARS_COPY_REPRO_DB")
	if source == "" {
		t.Skip("optional local-save regression")
	}
	absolute, err := filepath.Abs(source)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/", "only an audit backup may be used")
	content, err := os.ReadFile(absolute)
	require.NoError(t, err)
	destination := filepath.Join(t.TempDir(), "copied-game.db")
	require.NoError(t, os.WriteFile(destination, content, 0600))
	cfg := config.Config{}
	cfg.Database.Filename = destination
	cfg.Database.SkipUpgrade = true
	cfg.Game.TurnArchives = os.Getenv("STARS_COPY_TURN_ARCHIVES") != ""
	cfg.Database.ReadConnectionParams = "?_txlock=deferred"
	cfg.Database.WriteConnectionParams = "?_txlock=immediate&_busy_timeout=1200"
	database := db.NewConn()
	require.NoError(t, database.Connect(context.Background(), &cfg))
	defer database.Close()
	gameID := int64(6)
	if value := os.Getenv("STARS_COPY_REPRO_GAME"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		require.NoError(t, err)
		gameID = parsed
	}
	runner := NewGameRunner(database, cfg)
	before, err := database.NewReadClient().GetFullGame(t.Context(), gameID)
	require.NoError(t, err)
	if os.Getenv("STARS_COPY_PREVIEW_PRODUCTION") != "" {
		plan, err := before.PreviewProductionOrders(1)
		require.NoError(t, err)
		require.Equal(t, before.Year, plan.Year)
		for _, change := range plan.Planets {
			planet := before.Planets[change.Num-1]
			planet.ProductionQueue = change.ProductionQueue
			require.NoError(t, database.NewReadWriteClient().SavePlanet(t.Context(), planet))
		}
		t.Logf("Applied production-only preview to %d planets on a temporary copy", len(plan.Planets))
	}
	if os.Getenv("STARS_COPY_PREVIEW_ROUTES") != "" {
		preview, err := before.PreviewAutomaticRoutes(1)
		require.NoError(t, err)
		require.Equal(t, before.Year, preview.Year)
		for _, change := range preview.Fleets {
			for _, fleet := range before.Fleets {
				if fleet.ID == change.ID {
					fleet.Waypoints = change.Waypoints
					fleet.Tags = change.Tags
					require.NoError(t, database.NewReadWriteClient().SaveFleet(t.Context(), fleet))
				}
			}
		}
		t.Logf("Applied navigation-only preview to %d fleets on a temporary copy", len(preview.Fleets))
	}
	started := time.Now()
	_, err = runner.GenerateTurn(gameID)
	t.Logf("Native GenerateTurn wall time: %s", time.Since(started))
	require.NoError(t, err)
	after, err := database.NewReadClient().GetFullGame(t.Context(), gameID)
	require.NoError(t, err)
	require.Equal(t, before.Year+1, after.Year)
	if output := os.Getenv("STARS_COPY_RESULT_JSON"); output != "" {
		absolute, err := filepath.Abs(output)
		require.NoError(t, err)
		require.Contains(t, strings.ToLower(filepath.ToSlash(absolute)), "/audit-results/", "private copied results stay outside published source")
		data, err := json.Marshal(after)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(absolute, data, 0600))
	}

	if cfg.Game.TurnArchives {
		files, e := filepath.Glob(filepath.Join(filepath.Dir(destination), "turn-archives", "game-*", "*.json.gz"))
		require.NoError(t, e)
		require.Len(t, files, 2)
		for _, name := range files {
			info, e := os.Stat(name)
			require.NoError(t, e)
			t.Logf("Turn archive: %s (%d bytes)", filepath.Base(name), info.Size())
		}
	}
	beforePositions := map[int]cs.Vector{}
	for _, fleet := range before.Fleets {
		if fleet.PlayerNum == 1 {
			beforePositions[fleet.Num] = fleet.Position
		}
	}
	idleMaintenance := map[int]bool{}
	for _, f := range before.Fleets {
		if f.PlayerNum != 1 || !f.InDesignMaintenance() || len(f.Waypoints) > 1 || f.Fuel <= 0 {
			continue
		}
		atDock := false
		for _, planet := range before.Planets {
			atDock = atDock || planet.PlayerNum == 1 && planet.Spec.HasStarbase && planet.Position == f.Position
		}
		if !atDock {
			idleMaintenance[f.Num] = true
		}
	}
	movedMaintenance := 0
	recoveredScouts, recoveredColonizers := 0, 0
	for _, fleet := range after.Fleets {
		if fleet.PlayerNum != 1 {
			continue
		}
		role := after.Players[0].Automation.FleetRoles[fleet.Num]
		if idleMaintenance[fleet.Num] && (fleet.Position != beforePositions[fleet.Num] || len(fleet.Waypoints) > 1) {
			movedMaintenance++
		}
		if raw := fleet.GetTag("awaiting-tanker"); raw != "" && len(fleet.Waypoints) < 2 {
			n, _ := strconv.Atoi(raw)
			for _, donor := range after.Fleets {
				if donor.PlayerNum == 1 && donor.Num == n {
					var job struct{ Position cs.Vector }
					require.NoError(t, json.Unmarshal([]byte(donor.GetTag("automatic-tanker")), &job))
					require.Equal(t, job.Position, fleet.Position, "a waiting customer must already be at the agreed rendezvous, not stranded away from it")
				}
			}
		}
		require.GreaterOrEqual(t, fleet.Fuel, 0, "automatic planning must not spend nonexistent fuel")
		if (role == "scout" || strings.HasPrefix(role, "colonizer:")) && fleet.Position == beforePositions[fleet.Num] && len(fleet.Waypoints) < 2 {
			t.Logf("Stationary %s role=%s fuel=%d/%d tags=%v", fleet.Name, role, fleet.Fuel, fleet.Spec.FuelCapacity, fleet.Tags)
		}
		if role == "scout" && (fleet.Position != beforePositions[fleet.Num] || len(fleet.Waypoints) > 1) {
			recoveredScouts++
		}
		if strings.HasPrefix(role, "colonizer:") && (fleet.Position != beforePositions[fleet.Num] || len(fleet.Waypoints) > 1) {
			recoveredColonizers++
		}
	}
	if len(idleMaintenance) > 0 {
		require.GreaterOrEqual(t, movedMaintenance, (len(idleMaintenance)+1)/2)
		t.Logf("Previously idle off-dock rebuild fleets moving/routed: %d/%d", movedMaintenance, len(idleMaintenance))
	}
	t.Logf("Automatic scout fleets moving or routed: %d; colony fleets moving or routed: %d", recoveredScouts, recoveredColonizers)
	worked := 0
	patrolsIntercepting := 0
	for _, fleet := range after.Fleets {
		if fleet.PlayerNum == 1 && after.Players[0].Automation.FleetRoles[fleet.Num] == "patrol" {
			t.Logf("Patrol %s position=%v job=%s fuel=%d", fleet.Name, fleet.Position, fleet.GetTag("automatic-patrol"), fleet.Fuel)
			if strings.Contains(fleet.GetTag("automatic-patrol"), "Intercepting") {
				patrolsIntercepting++
			}
		}
		if fleet.PlayerNum == 1 && after.Players[0].Automation.FleetRoles[fleet.Num] == "freighter" {
			t.Logf("%s passengers=%d position=%v status=%s", fleet.Name, fleet.Cargo.Colonists*100, fleet.Position, fleet.GetTag("logistics-job"))
			if fleet.Cargo.Colonists > 0 || fleet.GetTag("logistics-job") != "" {
				worked++
			}
		}
	}
	require.Greater(t, worked, 0, "owned automatic freighters must receive real passenger or mineral work")
	if os.Getenv("STARS_COPY_REPRO_PATROL") != "" {
		require.Greater(t, patrolsIntercepting, 0, "nearby scouts must trigger actual patrol interception")
	}
}
