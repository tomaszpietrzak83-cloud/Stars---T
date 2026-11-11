package db

import (
	"strings"
	"testing"

	"github.com/sirgwain/craig-stars/db/generated"
	"github.com/stretchr/testify/require"
)

func TestGameListingUsesCoveringIndex(t *testing.T) {
	c, conn := connectTestDBWithConn()
	defer conn.Close()
	game, player := c.createTestGameWithPlayer(t.Context())
	// Large variable-length fields precede status fields in SQLite's row layout.
	// Reading status must not walk these overflow pages just to render the menu.
	_, err := conn.dbWrite.ExecContext(t.Context(), "UPDATE players SET messages = ?, archived = 1, submitted_turn = 1 WHERE id = ?", strings.Repeat(" ", 2*1024*1024), player.ID)
	require.NoError(t, err)

	for _, query := range []string{
		generated.GetGamesWithPlayersForUser,
		generated.GetGamesWithPlayers,
		generated.GetGameWithPlayers,
	} {
		// All three generated queries bind three parameters or fewer. SQLite
		// accepts repeated numbered parameters as a single binding.
		args := []any{1}
		if query == generated.GetGamesWithPlayers {
			args = []any{nil, nil, nil}
		}
		if query == generated.GetGameWithPlayers {
			args = []any{game.ID, nil}
		}
		rows, err := conn.dbRead.QueryContext(t.Context(), "EXPLAIN QUERY PLAN "+query, args...)
		require.NoError(t, err)
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			require.NoError(t, rows.Scan(&id, &parent, &unused, &detail))
			plan = append(plan, detail)
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		require.Contains(t, strings.Join(plan, "\n"), "COVERING INDEX idx_players_game_listing")
	}
	listed, err := c.GetGamesForUser(t.Context(), 1)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.True(t, listed[0].Players[0].Archived)
	require.True(t, listed[0].Players[0].SubmittedTurn)
	_, err = conn.dbWrite.ExecContext(t.Context(), "UPDATE players SET archived = 0, submitted_turn = 0 WHERE id = ?", player.ID)
	require.NoError(t, err)
	listed, err = c.GetGamesForUser(t.Context(), 1)
	require.NoError(t, err)
	require.False(t, listed[0].Players[0].Archived)
	require.False(t, listed[0].Players[0].SubmittedTurn)
}
