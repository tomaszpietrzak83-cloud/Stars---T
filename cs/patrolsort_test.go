package cs

import (
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// Independent comparator without memoization, including the new explicit landing priorities.
func referencePatrolTargets(p *Player, f *Fleet) []*Fleet {
	enemies := slices.Clone(p.FleetIntels)
	sort.SliceStable(enemies, func(i, j int) bool {
		if enemies[i] == nil {
			return false
		}
		if enemies[j] == nil {
			return true
		}
		ri, rj := patrolTargetPriority(p, enemies[i], enemyApproachingColony(p, enemies[i])), patrolTargetPriority(p, enemies[j], enemyApproachingColony(p, enemies[j]))
		if ri != rj {
			return ri < rj
		}
		if p.patrolEvaluator != nil {
			_, di := patrolColonyDeadline(p, enemies[i])
			_, dj := patrolColonyDeadline(p, enemies[j])
			if di != dj {
				return di < dj
			}
		}
		ib, jb := patrolReportedBomber(p, enemies[i]), patrolReportedBomber(p, enemies[j])
		if ib != jb {
			return ib
		}
		ii, ji := enemyApproachingColony(p, enemies[i]), enemyApproachingColony(p, enemies[j])
		if ii != ji {
			return ii
		}
		return f.Position.DistanceTo(enemies[i].Position) < f.Position.DistanceTo(enemies[j].Position)
	})
	return enemies
}

func TestPatrolTargetSortPreservesOrderAndRefreshesReports(t *testing.T) {
	p := &Player{Num: 1}
	p.PlanetIntels = []*Planet{nil, {MapObject: MapObject{PlayerNum: 1, Position: Vector{100, 0}}}}
	f := &Fleet{MapObject: MapObject{Position: Vector{0, 0}}}
	// Equal distance, nil reports, duplicate pointers, bombers, stale/deleted
	// contacts, friends and different headings must retain the old stable order.
	for i := range 200 {
		e := &Fleet{MapObject: MapObject{Num: i + 1, PlayerNum: 2, Position: Vector{i%9 + 1, 0}, ReportAge: i % 3}}
		e.Heading = VectorFloat64{X: float64(i%3) - 1}
		e.WarpSpeed = 9
		e.Spec.Bomber = i%7 == 0
		e.Delete = i%13 == 0
		p.FleetIntels = append(p.FleetIntels, e)
	}
	p.FleetIntels = append(p.FleetIntels, nil, p.FleetIntels[2], nil)
	original := slices.Clone(p.FleetIntels)
	check := func() {
		want, got := referencePatrolTargets(p, f), sortedPatrolTargets(p, f)
		require.Len(t, got, len(want))
		for i := range want {
			require.True(t, want[i] == got[i], "pointer ordering differs at %d", i)
		}
		for i := range original {
			require.True(t, original[i] == p.FleetIntels[i], "intel source reordered")
		}
	}
	check()
	p.patrolEvaluator = &patrolEvaluator{}
	check()
	p.PlanetIntels[1].PlayerNum = 2
	p.FleetIntels[0].Heading.X = 1
	p.FleetIntels[0].Spec.Bomber = true
	f.Position = Vector{1000, 0}
	check() // No cached priorities or distances may survive the sort call.
}
