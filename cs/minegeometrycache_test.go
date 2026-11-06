//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

// Preserve the merged mine-avoidance rules as the independent reference.
func referenceMineLegSafeRound2(rules *Rules, player *Player, from, to Vector, warp int, step float64, sweep bool) bool {
	for _, field := range player.MinefieldIntels {
		if !mineObstructsNavigation(player, field) {
			continue
		}
		radius, clearance := field.Radius(), automaticMineClearance(field)
		center := field.Position
		if !routeBoundsOverlap(from, to, center, center, clearance) {
			continue
		}
		closest := segmentDistance(center, from, to)
		if closest > clearance {
			continue
		}
		startDistance := from.DistanceTo(center)
		if field.Detonate {
			// A detonating field can only be escaped before this year's blast.
			end := to.ToFloat64()
			if distance := from.DistanceTo(to); distance > step {
				end = from.ToFloat64().Add(to.Subtract(from).Normalized().Scale(step))
			}
			if startDistance > clearance || end.DistanceTo(center.ToFloat64()) <= clearance || to.DistanceTo(center) <= clearance {
				return false
			}
			continue
		}
		safeWarp := rules.MinefieldStatsByType[field.MinefieldType].MaxSpeed + player.Race.Spec.MinefieldSafeWarpBonus
		if safeWarp == 0 {
			safeWarp = 3
		}
		if sweep && (warp <= safeWarp || closest > radius+1) {
			continue
		}
		// A fleet already inside may leave outward, at safe warp if it still
		// crosses actual mines. This also handles newly expanded fields.
		if startDistance <= clearance && to.DistanceTo(center) > clearance &&
			closest >= startDistance-1e-9 &&
			(startDistance > radius+1 || warp <= safeWarp) {
			continue
		}
		return false
	}
	return true
}

func TestMineGeometryCachePreservesSpeedsSweepingAndScope(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	rng := rand.New(rand.NewSource(2494))
	for i := 0; i < 60; i++ {
		p.MinefieldIntels = append(p.MinefieldIntels, &Minefield{MapObject: MapObject{Num: i + 1, PlayerNum: i%3 + 1, Position: Vector{rng.Intn(401) - 200, rng.Intn(401) - 200}}, MinefieldType: MinefieldTypeStandard, NumMines: 1 + rng.Intn(1600), Detonate: i%7 == 0})
	}
	cleanupIndex := scopedRouteThreatIndex(p)
	defer cleanupIndex()
	for batch := 0; batch < 2; batch++ {
		cleanup := scopedRouteThreatCache(p)
		for i := 0; i < 600; i++ {
			from, to := Vector{rng.Intn(501) - 250, rng.Intn(501) - 250}, Vector{rng.Intn(501) - 250, rng.Intn(501) - 250}
			if i%5 == 0 {
				from = p.MinefieldIntels[i%60].Position
			}
			if i%11 == 0 {
				to = from
			}
			for warp := 1; warp <= 11; warp++ {
				for _, sweep := range []bool{false, true} {
					step := float64(warp*warp) * 1.3
					require.Equal(t, referenceMineLegSafeRound2(&u.Game.Rules, p, from, to, warp, step, sweep), automaticMineLegSafe(&u.Game.Rules, p, from, to, warp, step, sweep))
				}
			}
		}
		require.NotEmpty(t, p.routeThreatIndex.mineLegs)
		// A nested route search must restore the enclosing geometry cache.
		previous := p.routeThreatIndex.mineLegs
		nested := scopedRouteThreatCache(p)
		require.Empty(t, p.routeThreatIndex.mineLegs)
		nested()
		require.Equal(t, previous, p.routeThreatIndex.mineLegs)
		cleanup()
		require.Nil(t, p.routeThreatIndex.mineLegs)
		for _, field := range p.MinefieldIntels {
			field.Position.X += 100
			field.NumMines *= 2
			field.Detonate = !field.Detonate
		}
	}
	cleanup := scopedRouteThreatCache(p)
	for i := 0; i < 4200; i++ {
		routeMineObstacles(p, Vector{}, Vector{X: i, Y: 10000})
	}
	require.LessOrEqual(t, len(p.routeThreatIndex.mineLegs), 4096)
	cleanup()
	// No cache may be created by a check outside an explicit route-search scope.
	automaticMineLegSafe(&u.Game.Rules, p, Vector{}, Vector{X: 20}, 5, 25, false)
	require.Nil(t, p.routeThreatIndex.mineLegs)
}
