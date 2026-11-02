package cs

// Only the direct preceding member of the same family qualifies. The four-year
// interval is between project creation years, not the later scrap/arrival year.
// Zero dates in older saves are unknown; do not manufacture historical dates.
func rapidUpgradeRecovery(p *Player, old, next *ShipDesign, oldCost Cost) (Cost, bool) {
	if old == nil || next == nil || old.Num == next.Num || old.PlayerNum != p.Num || next.PlayerNum != p.Num || old.OriginalPlayerNum != 0 || next.OriginalPlayerNum != 0 || old.Spec.Starbase || next.Spec.Starbase || old.Hull != next.Hull {
		return Cost{}, false
	}
	family := p.Automation.DesignFamilies[old.Num]
	if family == 0 || family != p.Automation.DesignFamilies[next.Num] || next.Spec.PreviousVersionNum != old.Num || next.Version != max(1, old.Version)+1 {
		return Cost{}, false
	}
	before, after := old.Spec.VersionCreatedYear, next.Spec.VersionCreatedYear
	if before <= 0 || after < before || after-before > 4 {
		return Cost{}, false
	}
	// Integer arithmetic rounds down each resource independently and never adds
	// ordinary scrap/racial bonuses on top. Construction remains full price.
	return Cost{Ironium: oldCost.Ironium * 9 / 10, Boranium: oldCost.Boranium * 9 / 10, Germanium: oldCost.Germanium * 9 / 10, Resources: oldCost.Resources * 9 / 10}, true
}
