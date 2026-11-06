package server

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/sirgwain/craig-stars/cs"
	"github.com/sirgwain/craig-stars/db"
	"github.com/stretchr/testify/require"
)

type failingTurnLoadClient struct {
	db.Client
	err      error
	stateErr error
}

func (c failingTurnLoadClient) GetFullGame(context.Context, int64) (*cs.FullGame, error) {
	return nil, c.err
}

func (c failingTurnLoadClient) UpdateGameState(ctx context.Context, id int64, state cs.GameState) error {
	if state == cs.GameStateGeneratingTurnError && c.stateErr != nil {
		return c.stateErr
	}
	return c.Client.UpdateGameState(ctx, id, state)
}

type failingTurnLoadConnection struct {
	db.DBConn
	client db.Client
}

func (c failingTurnLoadConnection) NewReadWriteClient() db.Client { return c.client }

func TestGenerateTurnLoadFailureReleasesGeneratingState(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "read error", true: "missing game"}[missing], func(t *testing.T) {
			conn, cfg := connectSubmitTestDB(t)
			id := hostTwoHumanGame(t, conn, cfg)
			before, err := conn.NewReadClient().GetGame(t.Context(), id)
			require.NoError(t, err)
			cause := errors.New("injected load failure")
			var loadErr error = cause
			if missing {
				loadErr = nil
				cause = errNotFound
			}
			faulty := failingTurnLoadConnection{conn, failingTurnLoadClient{Client: conn.NewReadWriteClient(), err: loadErr}}
			result, err := NewGameRunner(faulty, cfg).GenerateTurn(id)
			require.ErrorIs(t, err, cause)
			require.Equal(t, TurnNotGenerated, result)
			after, err := conn.NewReadClient().GetGame(t.Context(), id)
			require.NoError(t, err)
			require.Equal(t, before.Year, after.Year)
			require.Equal(t, cs.GameStateGeneratingTurnError, after.State)
			// Retrying after the read fault is removed still works.
			result, err = NewGameRunner(conn, cfg).GenerateTurn(id)
			require.NoError(t, err)
			require.Equal(t, TurnGenerated, result)
		})
	}
}

func TestGenerateTurnPreservesLoadAndStateErrors(t *testing.T) {
	conn, cfg := connectSubmitTestDB(t)
	id := hostTwoHumanGame(t, conn, cfg)
	loadErr, stateErr := errors.New("load failed"), errors.New("state write failed")
	faulty := failingTurnLoadConnection{conn, failingTurnLoadClient{conn.NewReadWriteClient(), loadErr, stateErr}}
	_, err := NewGameRunner(faulty, cfg).GenerateTurn(id)
	require.ErrorIs(t, err, loadErr)
	require.ErrorIs(t, err, stateErr)
}

func TestGenerateTurnAIErrorDoesNotCommitPartialTurn(t *testing.T) {
	conn, cfg := connectSubmitTestDB(t)
	id := hostTwoHumanGame(t, conn, cfg)
	before, err := conn.NewReadClient().GetFullGame(t.Context(), id)
	require.NoError(t, err)
	runner := NewGameRunner(conn, cfg).(*gameRunner)
	cause := errors.New("injected planning failure")
	var failed *cs.Player
	runner.aiTurn = func(g *cs.FullGame, p *cs.Player, objects cs.PlayerMapObjects, victors []int) error {
		p.Name = "partial AI mutation"
		failed = p
		return cause
	}
	result, err := runner.GenerateTurn(id)
	require.ErrorIs(t, err, cause)
	require.Equal(t, TurnNotGenerated, result)
	require.NotNil(t, failed)
	require.False(t, failed.SubmittedTurn)
	after, err := conn.NewReadClient().GetFullGame(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, before.Year, after.Year)
	require.Equal(t, cs.GameStateGeneratingTurnError, after.State)
	require.Equal(t, before.Players, after.Players)
	require.Equal(t, before.Fleets, after.Fleets)
	require.Equal(t, before.Planets, after.Planets)
	runner.aiTurn = nil
	result, err = runner.GenerateTurn(id)
	require.NoError(t, err)
	require.Equal(t, TurnGenerated, result)
}

func TestAIWorkersJoinAllErrorsBeforeReturning(t *testing.T) {
	conn, cfg := connectSubmitTestDB(t)
	id := hostTwoHumanGame(t, conn, cfg)
	g, err := conn.NewReadClient().GetFullGame(t.Context(), id)
	require.NoError(t, err)
	for _, p := range g.Players {
		p.AIControlled = true
		p.SubmittedTurn = true
	}
	runner := NewGameRunner(conn, cfg).(*gameRunner)
	first, last := errors.New("first"), errors.New("last")
	var completed atomic.Int32
	runner.aiTurn = func(g *cs.FullGame, p *cs.Player, objects cs.PlayerMapObjects, victors []int) error {
		defer completed.Add(1)
		switch p.Num {
		case 1:
			return first
		case 3:
			return last
		}
		return nil
	}
	err = runner.processAITurnsWorkers(g, 2)
	require.ErrorIs(t, err, first)
	require.ErrorIs(t, err, last)
	require.Equal(t, int32(3), completed.Load())
	require.False(t, g.Players[0].SubmittedTurn)
	require.True(t, g.Players[1].SubmittedTurn)
	require.False(t, g.Players[2].SubmittedTurn)
}

func TestHostGameAIErrorRollsBackUniverse(t *testing.T) {
	conn, cfg := connectSubmitTestDB(t)
	user, err := conn.NewReadWriteClient().CreateUser(t.Context(), cs.NewUser("AI-failure-host", "", "", cs.RoleUser))
	require.NoError(t, err)
	runner := NewGameRunner(conn, cfg).(*gameRunner)
	cause := errors.New("initial AI orders failed")
	runner.aiTurn = func(*cs.FullGame, *cs.Player, cs.PlayerMapObjects, []int) error { return cause }
	_, err = runner.HostGame(user.ID, cs.NewGameSettings().WithSize(cs.SizeTiny).WithHost(cs.Humanoids()).WithAIPlayer(cs.AIDifficultyNormal, 0))
	require.ErrorIs(t, err, cause)
	games, err := conn.NewReadClient().GetGames(t.Context())
	require.NoError(t, err)
	require.Empty(t, games)
}
