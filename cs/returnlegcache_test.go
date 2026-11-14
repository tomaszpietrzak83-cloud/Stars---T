//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSharedReturnLegCachePreservesEveryRoute(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	home := u.Planet("Home")
	near, far := *home, *home
	near.Num, near.Position = 2, Vector{X: 100}
	far.Num, far.Position = 3, Vector{X: 450}
	p.PlanetIntels = []*Planet{home, &near, &far}
	for _, gates := range []bool{false, true} {
		for _, dock := range p.PlanetIntels {
			dock.Spec.HasStargate, dock.Spec.SafeRange, dock.Spec.SafeHullMass = gates, 500, 10000
		}
		for _, allowThreat := range []bool{false, true} {
			for _, detonate := range []bool{false, true} {
				p.MinefieldIntels = []*Minefield{{MapObject: MapObject{PlayerNum: 2, Position: Vector{X: 25}}, MinefieldType: MinefieldTypeStandard, NumMines: 100, Detonate: detonate}}
				cache := map[automaticLegCacheKey]bool{}
				for repeat := 0; repeat < 2; repeat++ {
					for i := 0; i < 12; i++ {
						point := Vector{X: i * 40, Y: i % 3 * 8}
						policy := automaticRoutePolicy{OneWay: true, EmergencyReturn: true, MaxTurns: 8, AllowThreat: allowThreat}
						want := referenceRouteRound2(&u.Game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, policy)
						policy.returnLegCache = cache
						got := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, policy)
						require.Equal(t, want, got)
					}
				}
				require.NotEmpty(t, cache)
			}
		}
	}
}

func TestMinedDestinationEarlyRejectionMatchesOriginalSearch(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	f.Spec.MineSweep = 100
	for _, role := range []string{"patrol", "minesweeper"} {
		p.Automation.FleetRoles[f.Num] = role
		for _, owner := range []int{p.Num, 2} {
			for _, detonate := range []bool{false, true} {
				field := &Minefield{MapObject: MapObject{PlayerNum: owner, Position: Vector{X: 50}}, MinefieldType: MinefieldTypeStandard, NumMines: 100, Detonate: detonate}
				p.MinefieldIntels = []*Minefield{field}
				for _, point := range []Vector{{}, {X: 36}, {X: 37}, {X: 38}, {X: 50}, {X: 62}, {X: 63}, {X: 64}} {
					policy := automaticRoutePolicy{OneWay: true, Fast: true, MaxTurns: 8}
					want := referenceRouteRound2(&u.Game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, policy)
					got := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, policy)
					require.Equal(t, want, got, "role=%s owner=%d detonate=%v point=%v", role, owner, detonate, point)
				}
			}
		}
	}
}
