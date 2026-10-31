package server

import (
	"context"
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
	_, err = runner.GenerateTurn(gameID)
	require.NoError(t, err)
	after, err := database.NewReadClient().GetFullGame(t.Context(), gameID)
	require.NoError(t, err)
	require.Equal(t, before.Year+1, after.Year)
	beforePositions := map[int]cs.Vector{}
	for _, fleet := range before.Fleets {
		if fleet.PlayerNum == 1 {
			beforePositions[fleet.Num] = fleet.Position
		}
	}
	recoveredScouts, recoveredColonizers := 0, 0
	for _, fleet := range after.Fleets {
		if fleet.PlayerNum != 1 {
			continue
		}
		role := after.Players[0].Automation.FleetRoles[fleet.Num]
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
