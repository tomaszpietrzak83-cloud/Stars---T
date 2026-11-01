package cs

import "slices"

// Freshly automated combat stacks become a pool of independently assignable
// ships. Active packs, escorts, repair and rebuild missions remain intact.
func (t *turnGenerator) splitAutomaticCombatPools() {
	if t.ordersOnly {
		return
	}
	for _, player := range t.game.Players {
		for _, fleet := range slices.Clone(t.game.getFleets(player.Num)) {
			role := player.Automation.FleetRoles[fleet.Num]
			if (role != "patrol" && role != "escort") || fleet.Spec.TotalShips <= 1 || !patrolAvailable(player, fleet) || readPatrolJob(fleet).Leader != 0 || fleet.GetTag(bomberBorrowTag) != "" || fleet.GetTag(patrolSupportTag) != "" || readAutomaticFuelRequest(fleet) != nil {
				continue
			}
			children, err := NewOrderer().SplitAll(&t.game.Rules, player, t.game.getFleets(player.Num), fleet)
			if err != nil {
				continue
			}
			for _, child := range children {
				t.game.Fleets = append(t.game.Fleets, child)
				_ = t.game.Universe.addFleet(child)
				player.Automation.FleetRoles[child.Num] = role
			}
			for _, member := range append([]*Fleet{fleet}, children...) {
				delete(member.Tags, frontierPatrolTag)
				holdAutomaticFleet(player, member, "Independent "+role+"; available for frontier coverage or a coordinated wolf pack")
			}
		}
	}
}
