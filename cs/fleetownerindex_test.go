package cs

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFleetOwnerIndexTracksProductionAndNumberReuse(t *testing.T) {
	u := NewUniverse(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	a := &Fleet{MapObject: MapObject{PlayerNum: 1, Num: 1}}
	b := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 1}}
	u.Fleets = []*Fleet{a, b}
	require.Equal(t, []*Fleet{a}, u.getFleets(1))
	for i := 2; i < 20; i++ {
		f := &Fleet{MapObject: MapObject{PlayerNum: 1, Num: i}}
		u.Fleets = append(u.Fleets, f)
		require.NoError(t, u.addFleet(f))
		require.Equal(t, len(u.Fleets), u.fleetIndexSize, "production must extend the valid index")
		require.Same(t, f, u.getFleets(1)[i-1])
	}
	u.deleteFleet(a)
	reused := &Fleet{MapObject: MapObject{PlayerNum: 1, Num: 1}}
	u.Fleets = append(u.Fleets, reused)
	require.NoError(t, u.addFleet(reused))
	got := u.getFleets(1)
	require.NotContains(t, got, a)
	require.Same(t, reused, got[len(got)-1])
	require.Equal(t, []*Fleet{b}, u.getFleets(2))
	// Callers receive independent slices: sorting or overwriting one must not
	// corrupt the cached universe order.
	got[0] = reused
	require.NotSame(t, reused, u.getFleets(1)[0])
}

func TestFleetOwnerIndexFallsBackForBulkAppendAndCompaction(t *testing.T) {
	u := NewUniverse(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	a := &Fleet{MapObject: MapObject{PlayerNum: 1, Num: 1}}
	u.Fleets = []*Fleet{a}
	u.getFleets(1)
	b := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 1}}
	c := &Fleet{MapObject: MapObject{PlayerNum: 1, Num: 2}}
	u.Fleets = append(u.Fleets, b, c)
	require.NoError(t, u.addFleet(c))
	require.Equal(t, []*Fleet{a, c}, u.getFleets(1))
	require.Equal(t, []*Fleet{b}, u.getFleets(2))
	u.Fleets = []*Fleet{c}
	require.Equal(t, []*Fleet{c}, u.getFleets(1))
	require.Empty(t, u.getFleets(2))
}
