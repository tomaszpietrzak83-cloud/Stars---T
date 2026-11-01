//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSalvageCollectorMatchesDebrisSize(t *testing.T) {
	for _, tc := range []struct {
		amount int
		small  bool
	}{{5, true}, {5000, false}} {
		t.Run(fmt.Sprint(tc.amount), func(t *testing.T) {
			u := logisticsUniverse(t)
			p, small, large := u.Player(1), u.Fleet("Escort"), u.Fleet("Transport")
			small.Spec.CargoCapacity = 20
			p.Automation.FleetRoles[small.Num] = "patrol"
			addExpeditionTestDock(p, Vector{})
			salvage := u.Game.Salvages[0]
			salvage.Cargo = Cargo{Ironium: tc.amount}
			u.Run((*turnGenerator).planPatrolSalvage)
			selected := large
			if tc.small {
				selected = small
			}
			require.NotNil(t, logisticsWork(selected))
			require.Equal(t, "salvage", logisticsWork(selected).Kind)
		})
	}
}

func TestSurvivingPatrolTargetRecruitsSupportWithoutStoppingHunter(t *testing.T) {
	u := logisticsUniverse(t)
	p, leader, support := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	p.Automation.FleetRoles[leader.Num] = "patrol"
	p.Automation.FleetRoles[support.Num] = "patrol"
	enemy := patrolTestEnemy(0)
	physical := *enemy
	physical.Tokens = []ShipToken{{Quantity: 1}}
	p.FleetIntels = []*Fleet{enemy}
	addExpeditionTestDock(p, Vector{})
	savePatrolJob(leader, patrolJob{Leader: leader.Num, Enemy: enemy.PlayerNum, Target: enemy.Num})
	u.Run(func(tg *turnGenerator) {
		tg.markSurvivingPatrolTarget(&BattleRecord{fleets: []*Fleet{leader, &physical}})
	})
	require.Greater(t, readPatrolJob(leader).RecruitPower, patrolPower(leader))
	u.Run((*turnGenerator).planPatrolAutomation)
	require.Equal(t, leader.Num, readPatrolJob(support).Leader)
	require.Greater(t, len(leader.Waypoints), 1, "the original hunter must keep attacking while support approaches")
}

func TestSpeculativeCollectorReservationExpiresWithoutDebris(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	_ = p
	f.SetTag(anticipatedSalvageTag, `{"Position":{"x":50,"y":0},"Until":1}`)
	u.Run((*turnGenerator).planPatrolSalvage)
	require.Empty(t, f.GetTag(anticipatedSalvageTag))
}
