package server

import (
	"github.com/sirgwain/craig-stars/cs"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRaceTemplatesAreLegalAndCoverAllPrimaryTraits(t *testing.T) {
	types := map[cs.PRT]bool{}
	names := map[string]bool{}
	extras := 0
	for _, race := range raceTemplates() {
		require.Less(t, race.ID, int64(0))
		require.False(t, names[race.PluralName])
		names[race.PluralName] = true
		require.NoError(t, race.Validate())
		require.GreaterOrEqual(t, race.ComputeRacePoints(1650), 0, race.PluralName)
		types[race.PRT] = true
		if race.EEETraits != 0 {
			extras++
		}
	}
	require.Len(t, types, 10)
	require.GreaterOrEqual(t, len(names), 17)
	require.Equal(t, 4, extras)
}

func TestNewMapsHostAndAdvanceGamesWithTemplateRaces(t *testing.T) {
	connection, cfg := connectSubmitTestDB(t)
	c := connection.NewReadWriteClient()
	user, err := c.CreateUser(t.Context(), cs.NewUser("content-test", "pw", "", cs.RoleUser))
	require.NoError(t, err)
	gr := NewGameRunner(connection, cfg)
	for _, size := range []cs.Size{cs.SizeVast, cs.SizeImmense, cs.SizeColossal, cs.SizeTitanic, cs.SizeGalactic} {
		settings := cs.NewGameSettings().WithSize(size).WithHost(raceTemplates()[len(raceTemplates())-1])
		game, err := gr.HostGame(user.ID, settings)
		require.NoError(t, err)
		require.NotEmpty(t, game.Planets)
		require.NotEmpty(t, game.Fleets)
		before := game.Year
		require.NoError(t, cs.NewGamer().GenerateTurn(game.Game, game.Universe, game.Players))
		require.Equal(t, before+1, game.Year)
	}
}
