package db

import (
	"github.com/sirgwain/craig-stars/cs"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEEERaceAndPlayerPersistence(t *testing.T) {
	c := connectTestDB()
	race := cs.NewRace()
	race.UserID = 1
	race.EEETraits = cs.EEEEfficientDrives | cs.EEEResearchSpecialization
	race.EEEResearchField = cs.Propulsion
	require.NoError(t, c.SaveRace(t.Context(), race))
	loaded, err := c.GetRace(t.Context(), race.ID)
	require.NoError(t, err)
	require.Equal(t, race.EEETraits, loaded.EEETraits)
	require.Equal(t, cs.Propulsion, loaded.EEEResearchField)
	race.EEETraits = cs.EEEExtractionSpecialists
	race.EEEResearchField = cs.TechFieldNone
	require.NoError(t, c.SaveRace(t.Context(), race))
	loaded, err = c.GetRace(t.Context(), race.ID)
	require.NoError(t, err)
	require.Equal(t, race.EEETraits, loaded.EEETraits)
	require.Empty(t, loaded.EEEResearchField)
	game := c.createTestGame(t.Context())
	p := &cs.Player{GameDBObject: cs.GameDBObject{GameID: game.ID}, UserID: 1, Num: 1, Name: "EEE", Race: *race}
	require.NoError(t, c.SavePlayer(t.Context(), p))
	player, err := c.GetPlayerForGame(t.Context(), game.ID, 1)
	require.NoError(t, err)
	require.Equal(t, race.EEETraits, player.Race.EEETraits)
}
