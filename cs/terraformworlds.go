package cs

// Explicit permission to settle hostile but recoverable worlds also schedules
// the ordinary paid minimum terraforming. Claim Adjuster never pays for this.
func (t *turnGenerator) planTerraformingWorlds() {
	for _, p := range t.game.Players {
		if !p.Automation.ServeTerraformingWorlds || p.Race.Spec.Instaforming {
			continue
		}
		for _, planet := range t.game.getPlanets(p.Num) {
			if p.Race.GetPlanetHabitability(planet.Hab) > 0 || prospectivePlanetQuality(p, planet) <= 0 {
				continue
			}
			found := false
			for _, item := range planet.ProductionQueue {
				if item.Type == QueueItemTypeAutoMinTerraform || item.Type == QueueItemTypeAutoMaxTerraform || item.Type == QueueItemTypeTerraformEnvironment {
					found = true
					break
				}
			}
			if !found {
				queueMinimumTerraforming(planet)
			}
		}
	}
}

func queueMinimumTerraforming(planet *Planet) {
	planet.ProductionQueue = append([]ProductionQueueItem{{Type: QueueItemTypeAutoMinTerraform, Quantity: 100, Note: "Automatic minimum terraforming for a designated colony"}}, planet.ProductionQueue...)
}
