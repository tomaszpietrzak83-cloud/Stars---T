package cs

// A collector already on reported debris completes its pickup before an empty
// return or another speculative interception can overwrite the collection job.
func (t *turnGenerator) assignLocalDebrisPickups(p *Player) {
	for _, f := range t.game.getFleets(p.Num) {
		if f.Delete || f.Starbase || p.Automation.FleetRoles[f.Num] != "freighter" || f.CargoMass() != 0 || f.Spec.CargoCapacity <= 0 || f.InAutomaticSafety() || f.InDesignMaintenance() || f.GetTag("maintenance-pending") != "" || fuelNetworkManaged(f) || readTankerJob(f) != nil || f.GetTag(tankerAwaitTag) != "" || p.FleetInActiveAssembly(f) || f.GetTag(traderConvoyTag) != "" {
			continue
		}
		job := logisticsWork(f)
		if job != nil && job.Kind != "return" && job.Kind != "salvage" {
			continue
		}
		for _, report := range p.SalvageIntels {
			if report == nil || report.Delete || report.Position != f.Position || report.Cargo.ToMineral().Total() <= 0 {
				continue
			}
			actual := t.game.getSalvage(report.Num)
			if actual == nil || actual.Delete || actual.Position != f.Position || actual.Cargo.ToMineral().Total() <= 0 || t.armedEnemyAt(p, f.Position) {
				continue
			}
			delete(f.Tags, anticipatedSalvageTag)
			delete(f.Tags, containmentTag)
			delete(f.Tags, "empty-salvage")
			setCollectionBattlePlan(p, f)
			saveLogisticsWork(f, &logisticsJob{Kind: "salvage", TargetNum: report.Num, Position: report.Position, Status: "Collecting reported debris already under collector"})
			break
		}
	}
}
