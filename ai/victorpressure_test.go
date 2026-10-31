package ai

import (
	"github.com/sirgwain/craig-stars/cs"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDeclaredHumanVictorPressureUsesPublicVictoryAndKeepsOtherEnemies(t *testing.T) {
	game := cs.NewGame()
	p := cs.NewPlayer(1, cs.NewRace().WithSpec(&game.Rules))
	p.Relations = []cs.PlayerRelationship{{Relation: cs.PlayerRelationFriend}, {Relation: cs.PlayerRelationNeutral}, {Relation: cs.PlayerRelationEnemy}}
	ai := NewAIPlayer(game, &cs.StaticTechStore, p, cs.PlayerMapObjects{}, 2)
	ai.applyVictorPressure()
	require.False(t, p.IsEnemy(2))
	game.VictorDeclared = true
	ai = NewAIPlayer(game, &cs.StaticTechStore, p, cs.PlayerMapObjects{}, 2)
	ai.applyVictorPressure()
	require.True(t, p.IsEnemy(2))
	require.True(t, p.IsEnemy(3))
	require.Equal(t, 0, ai.config.startAttackingYear)
	require.GreaterOrEqual(t, ai.config.pace, 1.75)
}
func TestIdleAttackPrefersTheDeclaredVictorOverAnEasierNearbyEnemy(t *testing.T) {
	old := Experiment
	Experiment = func(_ int, behavior string) bool { return behavior == "war-range" }
	t.Cleanup(func() { Experiment = old })
	game := cs.NewGame()
	game.VictorDeclared = true
	p := cs.NewPlayer(1, cs.NewRace().WithSpec(&game.Rules))
	p.AIDifficulty = cs.AIDifficultyEasy
	attacker := NewAIPlayer(game, &cs.StaticTechStore, p, cs.PlayerMapObjects{}, 2)
	fleet := &cs.Fleet{}
	fleet.Spec.Engine.IdealSpeed = 9
	victor := &cs.Planet{MapObject: cs.MapObject{Num: 1, PlayerNum: 2, Position: cs.Vector{X: 300, Y: 0}}, Cargo: cs.Cargo{Colonists: 1000}}
	other := &cs.Planet{MapObject: cs.MapObject{Num: 2, PlayerNum: 3, Position: cs.Vector{X: 10, Y: 0}}, Cargo: cs.Cargo{Colonists: 10}}
	require.Same(t, victor, attacker.getBestPlanetToBomb(fleet, map[int]*cs.Planet{1: victor, 2: other}))
	attacker.priorityVictors = nil
	require.Same(t, other, attacker.getBestPlanetToBomb(fleet, map[int]*cs.Planet{1: victor, 2: other}))
}
