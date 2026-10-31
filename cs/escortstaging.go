package cs

const escortStagingTag = "escort-staging"

// Free escorts wait across the empire, rather than all at the capital. An
// assigned convoy, fuel rescue or repair always takes precedence over staging.
func (t *turnGenerator) planEscortStaging() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if p.Automation.FleetRoles[f.Num] != "escort" || f.Delete || f.Starbase || f.InDesignMaintenance() || f.InAutomaticSafety() || fuelNetworkManaged(f) || f.GetTag(escortTag) != "" || f.GetTag(tankerAwaitTag) != "" || readTankerJob(f) != nil || p.FleetInActiveAssembly(f) || f.GetTag("maintenance-pending") != "" || f.GetTag(bomberBorrowTag) != "" {
				continue
			}
			f.SetTag(escortStagingTag, "Moving economically to frontier escort coverage; return fuel reserved")
			reserve := automaticReturnFuel(&t.game.Rules, p, f, f.Position, expeditionStations(p, f), false, 20)
			if f.Fuel < f.Spec.FuelCapacity/4 || f.Fuel < reserve {
				delete(f.Tags, frontierPatrolTag)
				if !t.returnPatrolToFuel(p, f) {
					requestAutomaticRescue(p, f, true)
				}
				continue
			}
			if !t.patrolFrontier(p, f) {
				delete(f.Tags, escortStagingTag)
			}
		}
	}
}
