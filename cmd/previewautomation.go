package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirgwain/craig-stars/config"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	"github.com/spf13/cobra"
)

func newPreviewAutomationCmd() *cobra.Command {
	var database, output string
	var gameID int64
	var playerNum int
	c := &cobra.Command{Use: "preview-automation", Short: "Export navigation-only automation orders without advancing a turn", RunE: func(cmd *cobra.Command, args []string) error {
		if database == "" || output == "" || gameID <= 0 || playerNum <= 0 {
			return fmt.Errorf("database, output, game and player are required")
		}
		if _, err := os.Stat(database); err != nil {
			return err
		}
		cfg := config.Config{}
		cfg.Database.Filename = database
		cfg.Database.SkipUpgrade = true
		cfg.Database.ReadConnectionParams = "?_txlock=deferred"
		cfg.Database.WriteConnectionParams = "?_txlock=immediate&_busy_timeout=1200"
		conn := db.NewConn()
		if err := conn.Connect(cmd.Context(), &cfg); err != nil {
			return err
		}
		defer conn.Close()
		game, err := conn.NewReadClient().GetFullGame(cmd.Context(), gameID)
		if err != nil {
			return err
		}
		if game.State != cs.GameStateWaitingForPlayers {
			return fmt.Errorf("game is not waiting for orders")
		}
		plan, err := game.PreviewAutomaticRoutes(playerNum)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		if err = os.WriteFile(output, data, 0600); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Navigation preview: game %d, year %d, %d changed fleets. No turn generated.\n", gameID, plan.Year, len(plan.Fleets))
		return nil
	}}
	c.Flags().StringVar(&database, "database", "", "Existing snapshot database")
	c.Flags().StringVar(&output, "output", "", "Navigation JSON output")
	c.Flags().Int64Var(&gameID, "game", 0, "Game ID")
	c.Flags().IntVar(&playerNum, "player", 0, "Player number")
	return c
}
func init() { rootCmd.AddCommand(newPreviewAutomationCmd()) }
