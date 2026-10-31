package cs

import "slices"

// Late stations retain the native separate batteries and economic tradeoffs.
// Bastion upgrades Ultra's six batteries (96 -> 120 weapons), not four.
func EEEStationHulls() []TechHull {
	b := UltraStation
	b.Tech = NewTech("EEE Bastion Station", NewCost(160, 110, 380, 850), TechRequirements{TechLevel: TechLevel{Construction: 22, Energy: 20, Electronics: 16}}, 60, TechCategoryStarbaseHull)
	b.Armor, b.Initiative = 1250, 17
	b.Slots = slices.Clone(UltraStation.Slots)
	s := DeathStar
	s.Tech = NewTech("EEE Sanctuary Star", NewCost(160, 110, 460, 1050), TechRequirements{TechLevel: TechLevel{Construction: 24, Energy: 23, Electronics: 20}, PRTsRequired: []PRT{AR}}, 70, TechCategoryStarbaseHull)
	s.Armor, s.Initiative, s.MaxPopulation = 1875, 19, 3_750_000
	s.Slots = slices.Clone(DeathStar.Slots)
	for _, h := range []*TechHull{&b, &s} {
		positions := map[HullSlotType][]VectorFloat64{
			HullSlotTypeOrbitalElectrical: {{0, -2}, {0, 2}},
			HullSlotTypeWeapon:            {{-2, -2}, {2, 2}, {2, -2}, {-2, 2}, {-2, 0}, {2, 0}},
			HullSlotTypeShield:            {{-1, -2}, {1, 2}},
			HullSlotTypeShieldArmor:       {{1, -2}, {-1, 2}},
			HullSlotTypeElectrical:        {{-2, -1}, {2, 1}, {2, -1}, {-2, 1}, {-2, 0}, {2, 0}},
		}
		index := map[HullSlotType]int{}
		for i := range h.Slots {
			slot := &h.Slots[i]
			slot.Position = positions[slot.Type][index[slot.Type]]
			index[slot.Type]++
			switch slot.Type {
			case HullSlotTypeWeapon:
				if h == &b {
					slot.Capacity = 20
				}
			case HullSlotTypeShield:
				if h == &b {
					slot.Capacity = 24
				} else {
					slot.Capacity = 36
				}
			case HullSlotTypeShieldArmor:
				slot.Capacity = 24
			case HullSlotTypeElectrical:
				slot.Capacity = 4
			}
		}
	}
	return []TechHull{b, s}
}
