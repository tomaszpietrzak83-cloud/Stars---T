//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDispersedPatrolFuelCachePreservesTransfers(t *testing.T) {
	u := tankerUniverse(t)
	p := u.Player(1)
	base := u.Fleet("Customer")
	for _, point := range []Vector{{0, 0}, {60, 0}, {600, 200}} {
		var actual, original []*Fleet
		for i := 0; i < 12; i++ {
			f := *base
			f.Num = 100 + i
			f.Name = "Distinct name"
			f.Position = point
			f.Fuel = (i % 3) * f.Spec.FuelCapacity / 3
			if i == 10 {
				f.Fuel = f.Spec.FuelCapacity
			}
			p.Automation.FleetRoles[f.Num] = "patrol"
			if i == 11 {
				p.Automation.FleetRoles[f.Num] = "escort"
			}
			copy := f
			actual = append(actual, &f)
			original = append(original, &copy)
		}
		u.turn.referenceBalanceDispersedPatrolFuel(p, original)
		u.turn.balanceDispersedPatrolFuel(p, actual)
		for i := range actual {
			require.Equal(t, original[i], actual[i])
		}
	}
	a := *base
	b := a
	b.Num = 999
	b.Name = "Other identity"
	b.BaseName = "Other"
	ka, ok := dispersedPatrolReturnKey(&a)
	require.True(t, ok)
	kb, ok := dispersedPatrolReturnKey(&b)
	require.True(t, ok)
	require.Equal(t, ka, kb)
	b.Fuel++
	kb, _ = dispersedPatrolReturnKey(&b)
	require.NotEqual(t, ka, kb)
	b = a
	b.Tags = Tags{frontierPatrolTag: "yes"}
	kb, _ = dispersedPatrolReturnKey(&b)
	require.NotEqual(t, ka, kb)
	b = a
	b.Cargo.Ironium++
	kb, _ = dispersedPatrolReturnKey(&b)
	require.NotEqual(t, ka, kb)
	b = a
	b.Spec.FuelCapacity++
	kb, _ = dispersedPatrolReturnKey(&b)
	require.NotEqual(t, ka, kb)
}
