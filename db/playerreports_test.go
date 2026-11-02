package db

import (
	"math"
	"testing"
	"time"

	"github.com/sirgwain/craig-stars/cs"
	"github.com/stretchr/testify/require"
)

func TestPrefetchReportsOrderedAndLossless(t *testing.T) {
	players := make([]*cs.Player, 19)
	for i := range players {
		players[i] = &cs.Player{Intels: cs.Intels{PlanetIntels: []*cs.Planet{
			{MapObject: cs.MapObject{Num: i + 1, Name: "individual report", ReportAge: i}}, nil,
		}}}
	}
	expected, err := encodePlayerReports(players)
	require.NoError(t, err)
	batches, stop := prefetchPlayerReports(players)
	defer stop()
	index := 0
	for batch := range batches {
		require.NoError(t, batch.err)
		for _, report := range batch.reports {
			require.Equal(t, expected[index], report)
			index++
		}
	}
	require.Equal(t, len(players), index)
}

func TestPrefetchReportsErrorAndEarlyStop(t *testing.T) {
	players := make([]*cs.Player, 24)
	for i := range players {
		players[i] = &cs.Player{Intels: cs.Intels{PlanetIntels: []*cs.Planet{{}}}}
	}
	players[9].PlanetIntels[0].Spec.DefenseCoverage = math.NaN()
	batches, stop := prefetchPlayerReports(players)
	defer stop()
	require.NoError(t, (<-batches).err)
	require.Error(t, (<-batches).err)
	_, open := <-batches
	require.False(t, open)
	stop()

	// Simulate a SQL failure before consuming any encoded report. Cancellation
	// must unblock the hand-off and join workers before the caller resumes edits.
	_, cancel := prefetchPlayerReports(players)
	finished := make(chan struct{})
	go func() { cancel(); cancel(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("report workers did not stop")
	}
	players[0].PlanetIntels[0].Name = "safe to mutate after workers join"
}
