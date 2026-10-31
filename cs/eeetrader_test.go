//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTraderTiersRequirePredecessorInEveryRewardPath(t *testing.T) {
	p := testPlayer()
	second := StaticTechStore.GetHullComponent("EEE Enigma Pulsar II")
	third := StaticTechStore.GetHullComponent("EEE Enigma Pulsar III")
	require.NotNil(t, second)
	require.NotNil(t, third)
	require.False(t, eligibleTraderPart(p, &second.Tech))
	require.False(t, eligibleTraderPart(p, &third.Tech))
	p.AcquiredTechs[EnigmaPulsar.Name] = true
	part, reward := getMysteryTraderPart(newIntRandom(), p, MysteryTraderRewardEngine)
	require.Equal(t, second.Name, part.Name)
	require.Equal(t, MysteryTraderRewardEngine, reward)
	require.False(t, eligibleTraderPart(p, &third.Tech))
	p.AcquiredTechs[second.Name] = true
	part, _ = getMysteryTraderPart(newIntRandom(), p, MysteryTraderRewardEngine)
	require.Equal(t, third.Name, part.Name)
	// Force the fallback by owning every engine. Locked tier III parts from
	// other categories must never become eligible through a reroll.
	p.AcquiredTechs[third.Name] = true
	for range 80 {
		part, _ = getMysteryTraderPart(rules.random, p, MysteryTraderRewardEngine)
		if part != nil && part.Requirements.AcquiredTechPrerequisite != "" {
			require.True(t, p.AcquiredTechs[part.Requirements.AcquiredTechPrerequisite])
		}
	}
}

func TestTraderTierCatalogueAndMeaningfulBonuses(t *testing.T) {
	parts := EEETraderComponents()
	require.Len(t, parts, 18)
	for _, part := range parts {
		require.Equal(t, OriginMysteryTrader, part.Origin)
		require.True(t, part.Requirements.Acquirable)
		require.NotNil(t, StaticTechStore.GetHullComponent(part.Name))
		if name := part.Requirements.AcquiredTechPrerequisite; name != "" {
			parent := StaticTechStore.GetHullComponent(name)
			require.NotNil(t, parent)
			require.Greater(t, part.Cost.Resources, parent.Cost.Resources)
			require.LessOrEqual(t, part.Cost.Resources, (parent.Cost.Resources*112+99)/100)
			if parent.Armor > 0 {
				require.GreaterOrEqual(t, part.Armor, parent.Armor*120/100)
				require.LessOrEqual(t, part.Armor, (parent.Armor*140+99)/100)
			}
			if parent.CargoBonus > 0 {
				require.GreaterOrEqual(t, part.CargoBonus, parent.CargoBonus*120/100)
				require.LessOrEqual(t, part.CargoBonus, (parent.CargoBonus*140+99)/100)
			}
			if parent.Power > 0 {
				require.GreaterOrEqual(t, part.Power, parent.Power*120/100)
				require.LessOrEqual(t, part.Power, (parent.Power*140+99)/100)
			}
			if parent.FuelBonus > 0 {
				require.GreaterOrEqual(t, part.FuelBonus, parent.FuelBonus*120/100)
				require.Greater(t, part.FuelGeneration, parent.FuelGeneration)
			}
			if parent.TorpedoBonus > 0 {
				require.GreaterOrEqual(t, part.TorpedoBonus, parent.TorpedoBonus*1.2)
				require.Greater(t, part.TorpedoJamming, parent.TorpedoJamming)
			}
			if parent.Category == TechCategoryEngine {
				require.Less(t, part.Mass, parent.Mass)
				require.Greater(t, part.MovementBonus, parent.MovementBonus)
				require.Greater(t, part.CloakUnits, parent.CloakUnits)
				for warp := 6; warp <= 10; warp++ {
					require.Less(t, part.FuelUsage[warp], parent.FuelUsage[warp])
				}
			}
			p := NewPlayer(1, NewRace()).WithTechLevels(TechLevel{26, 26, 26, 26, 26, 26})
			p.AcquiredTechs[part.Name] = true
			require.False(t, p.HasTech(&part.Tech))
			p.AcquiredTechs[name] = true
			require.True(t, p.HasTech(&part.Tech))
		}
	}
}
