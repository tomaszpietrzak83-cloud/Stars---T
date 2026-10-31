//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResearchBudgetIsReservedUnlessColonyUsesOnlyLeftovers(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Cargo: Cargo{Colonists: 1000, Ironium: 100000, Germanium: 100000}, ProductionQueue: []ScenarioProductionQueueItem{{Type: QueueItemTypeAutoFactories, Quantity: 1000}, {Type: QueueItemTypeAutoMines, Quantity: 1000}}}}})
	p, planet := u.Player(1), u.Planet("Home")
	p.ResearchAmount = 15
	p.Researching = Construction
	planet.ContributesOnlyLeftoverToResearch = true
	for range 5 {
		require.NoError(t, u.turn.generateTurn())
		require.Zero(t, p.ResearchSpentLastYear)
	}
	require.Zero(t, p.TechLevelsSpent.Construction)
	planet.ContributesOnlyLeftoverToResearch = false
	require.NoError(t, u.turn.generateTurn())
	require.Positive(t, p.ResearchSpentLastYear)
}
