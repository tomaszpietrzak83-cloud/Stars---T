//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

// These are characterization tests: they document limitations, not desired fixes.
func TestPatrolAuditThreatBoundary(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	target, support := patrolTestEnemy(100), patrolTestEnemy(400)
	support.Num++
	target.Position = Vector{}
	p.FleetIntels = []*Fleet{target, support}
	support.Position = Vector{30, 0}
	require.Equal(t, 100, patrolThreat(p, target))
	support.Position = Vector{31, 0}
	require.Equal(t, 100, patrolThreat(p, target))
	support.Heading, support.WarpSpeed = VectorFloat64{X: -1}, 6
	require.Equal(t, 500, patrolThreat(p, target))
	t.Log("Historical radius defect fixed: stationary 30/31-ly contacts excluded; public converging course included")
}

func TestPatrolAuditDoesNotReserveSectorDefender(t *testing.T) {
	u := logisticsUniverse(t)
	p, a, b := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	b.Position = a.Position
	p.Automation.FleetRoles = map[int]string{a.Num: "patrol", b.Num: "patrol"}
	a.SetTag(patrolSectorTag, "1")
	b.SetTag(patrolSectorTag, "2")
	// This test isolates sector recruitment; equipment must be known under
	// the newer mandatory-reconnaissance rule.
	p.FleetIntels = []*Fleet{patrolTestEnemy(patrolPower(a), p)}
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planPatrolAutomation)
	require.True(t, b.Delete)
	require.True(t, readPatrolJob(a).Grouped)
	require.True(t, patrolCanAttack(p, a))
	t.Log("both differently assigned sector defenders recruited into one combat stack; no sector reserve")
}

func TestPatrolAuditUnknownEquipmentUsesOwnWeaponScale(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	e := patrolTestEnemy(0)
	e.Tokens = []ShipToken{{DesignNum: 99, Quantity: 10}}
	require.Equal(t, 40*reportedWeaponUnit(p), patrolReportedPower(p, e))
	p.ShipDesignIntels = []*ShipDesign{{PlayerNum: 2, Num: 99, Hull: Destroyer.Name}}
	low := patrolReportedPower(p, e)
	p.Designs = append(p.Designs, &ShipDesign{Slots: []ShipDesignSlot{{HullComponent: DeltaTorpedo.Name, Quantity: 1}}, Spec: ShipDesignSpec{HasWeapons: true, PowerRating: 10000}})
	require.Greater(t, patrolReportedPower(p, e), low)
	t.Logf("same unscanned enemy estimate changes from %d to %d when only own design library changes", low, patrolReportedPower(p, e))
}

// Actual native battles, fresh in-memory scenarios and reproducible independent seeds.
// Select the smallest homogeneous patrol stack satisfying the real 150% rule.
func TestPatrolAuditCombatMatrix(t *testing.T) {
	designs := []ShipDesign{DesignDestroyerDelta, DesignStalwartDefender, DesignBattleBeam, DesignBattleTorpedo, DesignJihadCruiser}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	total, wiped, incomplete := 0, 0, 0
	for _, own := range designs {
		for _, enemy := range designs {
			for _, count := range []int{1, 5, 20} {
				losses, unfinished, n, ownPower, enemyPower := 0, 0, 0, 0, 0
				for seed := 0; seed < 20; seed++ {
					// First build computes per-design specs; second builds the actual stack.
					scenario := TestScenario{Players: []ScenarioPlayer{
						{Designs: Designs(own), Fleets: []ScenarioFleet{{Name: "Patrol", Design: own.Name, Quantity: 1}}},
						{Designs: Designs(enemy), Fleets: []ScenarioFleet{{Name: "Enemy", Design: enemy.Name, Quantity: count}}},
					}}
					probe := BuildScenario(scenario)
					ownPower = patrolPower(probe.Fleets[0])
					enemyPower = patrolPower(probe.Fleets[1])
					require.Positive(t, ownPower)
					n = (enemyPower*3/2 + 1 + ownPower - 1) / ownPower
					scenario.Players[0].Fleets[0].Quantity = n
					game := BuildScenario(scenario)
					p, q, a, b := game.Players[0], game.Players[1], game.Fleets[0], game.Fleets[1]
					p.Relations = []PlayerRelationship{{Relation: PlayerRelationFriend}, {Relation: PlayerRelationEnemy}}
					q.Relations = []PlayerRelationship{{Relation: PlayerRelationEnemy}, {Relation: PlayerRelationFriend}}
					p.FleetIntels = []*Fleet{b}
					p.ShipDesignIntels = []*ShipDesign{b.Tokens[0].design}
					p.shipIntelIndex = nil
					savePatrolJob(a, patrolJob{Leader: a.Num, Enemy: 2, Target: b.Num})
					require.True(t, patrolCanAttack(p, a), fmt.Sprintf("%s vs %s: own=%d threat=%d", own.Name, enemy.Name, patrolPower(a), patrolThreat(p, b)))
					game.Rules.ResetSeed(int64(seed))
					battle := newBattler(logger, &game.Rules, 1, map[int]*Player{1: p, 2: q}, []*Fleet{a, b}, nil)
					record := battle.runBattle()
					if record.Stats.ShipsDestroyedByPlayer[1] == n {
						losses++
					}
					if record.Stats.ShipsDestroyedByPlayer[2] < count {
						unfinished++
					}
				}
				total += 20
				wiped += losses
				incomplete += unfinished
				t.Logf("MATRIX own=%s n=%d power=%d enemy=%s n=%d power=%d wiped=%d/20 enemy_survives=%d/20", own.Name, n, ownPower*n, enemy.Name, count, enemyPower, losses, unfinished)
			}
		}
	}
	require.Equal(t, 1500, total)
	require.Positive(t, wiped, "the current rating admits losing attacks; this audit must expose the limitation")
	t.Logf("TOTAL battles=%d patrol_wiped=%d enemy_survives=%d", total, wiped, incomplete)
}
