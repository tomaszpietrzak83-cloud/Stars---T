//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGateTimeBoundKeepsReachableJumpsAndRejectsDistantEntries(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	f.Position = Vector{}
	a := *u.Planet("Home")
	b := a
	a.Position, b.Position = Vector{81, 0}, Vector{919, 0}
	for _, station := range []*Planet{&a, &b} {
		station.Spec.HasStargate = true
		station.Spec.SafeRange = 10000
		station.Spec.SafeHullMass = 1000000
	}
	fleet := []*Fleet{f}
	stations := []*Planet{&a, &b}
	require.True(t, automaticGateCompatible(p, fleet, &a))
	require.Equal(t, 3, automaticGateTurnLowerBound(p, fleet, stations, Vector{}, Vector{1000, 0}, 9), "entry, jump and exit each take a turn")
	a.Position, b.Position = Vector{}, Vector{1000, 0}
	require.Equal(t, 1, automaticGateTurnLowerBound(p, fleet, stations, Vector{}, Vector{1000, 0}, 9), "direct jump remains possible")
	a.Position, b.Position = Vector{500, 500}, Vector{900, 900}
	require.Equal(t, 1, automaticGateTurnLowerBound(p, fleet, stations, Vector{}, Vector{81, 0}, 9), "nearby ordinary flight remains possible")
	require.Greater(t, automaticGateTurnLowerBound(p, fleet, stations, Vector{}, Vector{1000, 0}, 9), 2)
	f.Spec.CanJump = true
	b.Position = Vector{1000, 0}
	require.Equal(t, 1, automaticGateTurnLowerBound(p, fleet, stations, Vector{}, b.Position, 9), "onboard gate needs no entry dock")
	jump := *f
	jump.Tokens = append([]ShipToken(nil), f.Tokens...)
	design := *jump.Tokens[0].design
	design.Spec.Mass = 100000
	jump.Tokens[0].design = &design
	f.Spec.CanJump = false
	a.Position = Vector{}
	a.Spec.SafeHullMass = f.Tokens[0].design.Spec.Mass
	require.Equal(t, 1, automaticGateTurnLowerBound(p, []*Fleet{f, &jump}, stations, Vector{}, b.Position, 9), "onboard-gate ship ignores source mass limit in mixed convoy")
}
