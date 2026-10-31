package server

import (
	"connectrpc.com/connect"
	"context"
	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/proto/converter"
	v1 "github.com/sirgwain/craig-stars/proto/gen/craig_stars/v1"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEEERaceAPIRejectsOppositesAndPersistsValidTraits(t *testing.T) {
	connection, _ := connectSubmitTestDB(t)
	c := connection.NewReadWriteClient()
	ctx := t.Context()
	user, err := c.CreateUser(ctx, cs.NewUser("eee-test", "pw", "", cs.RoleUser))
	require.NoError(t, err)
	ctx = context.WithValue(ctx, keyDb, c)
	ctx = context.WithValue(ctx, keyDbRead, c)
	ctx = context.WithValue(ctx, keyDbWrite, c)
	ctx = context.WithValue(ctx, keyUserSession, userSession{ID: user.ID})
	svc := NewRaceServiceHandler(connection)
	race := cs.NewRace()
	race.EEETraits = cs.EEEEfficientDrives | cs.EEEInefficientPropulsion
	_, err = svc.CreateRace(ctx, connect.NewRequest(&v1.CreateRaceRequest{Race: converter.C.ConvertCSRace(*race)}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	race.EEETraits = cs.EEEEfficientDrives | cs.EEEResearchSpecialization
	race.EEEResearchField = cs.Electronics
	response, err := svc.CreateRace(ctx, connect.NewRequest(&v1.CreateRaceRequest{Race: converter.C.ConvertCSRace(*race)}))
	require.NoError(t, err)
	stored, err := c.GetRace(ctx, response.Msg.Race.Id)
	require.NoError(t, err)
	require.Equal(t, race.EEETraits, stored.EEETraits)
	require.Equal(t, cs.Electronics, stored.EEEResearchField)
	response.Msg.Race.EeeTraits = int32(cs.EEESlowMaintenance | cs.EEEMaintenanceCrews)
	response.Msg.Race.EeeResearchField = v1.TechField_TECH_FIELD_UNSPECIFIED
	_, err = svc.UpdateRace(ctx, connect.NewRequest(&v1.UpdateRaceRequest{Race: response.Msg.Race}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
