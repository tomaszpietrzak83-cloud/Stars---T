package cs

import (
	"slices"
)

// Trader tiers improve their actual role, including the hybrid abilities which
// distinguish trader parts. Resource prices rise about 10%, rather than faster
// than the utility gained. The predecessor licence remains mandatory.
func EEETraderComponents() []TechHullComponent {
	var components []TechHullComponent
	for _, base := range []TechHullComponent{EnigmaPulsar, MegaPolyShell, LangstonShell, AlienMiner, MultiCargoPod, AntiMatterTorpedo} {
		previous := base
		for index, tier := range []string{"II", "III"} {
			upgrade := upgradeTraderComponent(previous)
			upgrade.Name = "EEE " + base.Name + " " + tier
			applyTraderFamilyTier(&upgrade, base.Name, index)
			components = append(components, upgrade)
			previous = upgrade
		}
	}
	fuel := TechHullComponent{Tech: NewTechWithOrigin("EEE Trader Fuel Cell I", NewCost(10, 0, 4, 22), TechRequirements{Acquirable: true, TechLevel: TechLevel{Energy: 15, Propulsion: 13, Construction: 12}}, 70, TechCategoryMechanical, OriginMysteryTrader, TechTagFuelTank), Mass: 10, FuelBonus: 600, FuelGeneration: 5, HullSlotType: HullSlotTypeMechanical}
	computer := TechHullComponent{Tech: NewTechWithOrigin("EEE Harmonic Computer I", NewCost(2, 0, 20, 20), TechRequirements{Acquirable: true, TechLevel: TechLevel{Energy: 15, Electronics: 17}}, 70, TechCategoryElectrical, OriginMysteryTrader, TechTagTorpedoBonus, TechTagTorpedoJammer, TechTagInitiativeBonus), Mass: 3, TorpedoJamming: 0.15, TorpedoBonus: 0.25, InitiativeBonus: 1, HullSlotType: HullSlotTypeElectrical}
	for _, base := range []TechHullComponent{fuel, computer} {
		components = append(components, base)
		previous := base
		for index, tier := range []string{"II", "III"} {
			upgrade := upgradeTraderComponent(previous)
			upgrade.Name = base.Name[:len(base.Name)-1] + tier
			if base.FuelBonus > 0 {
				upgrade.FuelBonus = []int{800, 1050}[index]
				upgrade.FuelGeneration = []int{8, 12}[index]
			}
			if base.TorpedoBonus > 0 {
				upgrade.TorpedoBonus = []float64{0.32, 0.40}[index]
				upgrade.TorpedoJamming = []float64{0.20, 0.25}[index]
				upgrade.InitiativeBonus = index + 2
			}
			components = append(components, upgrade)
			previous = upgrade
		}
	}
	return components
}

func upgradeTraderComponent(previous TechHullComponent) TechHullComponent {
	upgrade := previous
	upgrade.Requirements.AcquiredTechPrerequisite = previous.Name
	for _, field := range TechFields {
		if level := upgrade.Requirements.TechLevel.Get(field); level > 0 {
			upgrade.Requirements.TechLevel.Set(field, min(26, level+2))
		}
	}
	upgrade.Cost = Cost{Ironium: (previous.Cost.Ironium*11 + 9) / 10, Boranium: (previous.Cost.Boranium*11 + 9) / 10, Germanium: (previous.Cost.Germanium*11 + 9) / 10, Resources: (previous.Cost.Resources*11 + 9) / 10}
	upgrade.Ranking++
	if previous.Armor > 0 {
		upgrade.Armor = (previous.Armor*125 + 99) / 100
	}
	if previous.Shield > 0 {
		upgrade.Shield = (previous.Shield*125 + 99) / 100
	}
	if previous.CargoBonus > 0 {
		upgrade.CargoBonus = (previous.CargoBonus*125 + 99) / 100
	}
	if previous.MiningRate > 0 {
		upgrade.MiningRate = (previous.MiningRate*125 + 99) / 100
	}
	if previous.Power > 0 {
		upgrade.Power = (previous.Power*125 + 99) / 100
	}
	upgrade.FuelUsage = slices.Clone(previous.FuelUsage)
	for i, value := range upgrade.FuelUsage {
		if value > 0 {
			upgrade.FuelUsage[i] = max(1, value*82/100)
		}
	}
	return upgrade
}

func applyTraderFamilyTier(part *TechHullComponent, family string, tier int) {
	switch family {
	case EnigmaPulsar.Name:
		part.Mass = []int{18, 16}[tier]
		part.MovementBonus = []float64{0.65, 0.80}[tier]
		part.CloakUnits = []int{30, 40}[tier]
	case MegaPolyShell.Name:
		part.Armor = []int{520, 650}[tier]
		part.Shield = []int{130, 170}[tier]
		part.TorpedoJamming = []float64{0.23, 0.26}[tier]
		part.CloakUnits = []int{50, 60}[tier]
		part.ScanRange = []int{100, 125}[tier]
		part.ScanRangePen = []int{50, 65}[tier]
	case LangstonShell.Name:
		part.Armor = []int{85, 110}[tier]
		part.Shield = []int{165, 210}[tier]
		part.TorpedoJamming = []float64{0.08, 0.11}[tier]
		part.CloakUnits = []int{25, 30}[tier]
		part.ScanRange = []int{65, 85}[tier]
		part.ScanRangePen = []int{35, 45}[tier]
	case AlienMiner.Name:
		part.MiningRate = []int{13, 17}[tier]
		part.TorpedoJamming = []float64{0.33, 0.36}[tier]
		part.MovementBonus = []float64{0.6, 0.7}[tier]
		part.CloakUnits = []int{75, 90}[tier]
	case MultiCargoPod.Name:
		part.CargoBonus = []int{330, 420}[tier]
		part.Armor = []int{65, 85}[tier]
		part.CloakUnits = []int{25, 30}[tier]
	case AntiMatterTorpedo.Name:
		part.Power = []int{80, 105}[tier]
		part.Accuracy = []int{87, 89}[tier]
	}
}

func eeeTraderTechs() []Tech {
	var result []Tech
	for _, component := range EEETraderComponents() {
		result = append(result, component.Tech)
	}
	return result
}

func eligibleTraderPart(player *Player, tech *Tech) bool {
	return !player.HasAcquiredTech(tech) && (tech.Requirements.AcquiredTechPrerequisite == "" || player.AcquiredTechs[tech.Requirements.AcquiredTechPrerequisite])
}
