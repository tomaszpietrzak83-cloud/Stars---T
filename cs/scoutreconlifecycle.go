package cs

import "slices"

// Only current player reports can authorize contact with a fleet number.
// The number may have disappeared in a merge or refer to a new split child.
func (t *turnGenerator) refreshScoutReconOrders() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			job := readScoutRecon(f)
			if job == nil || f.Delete {
				continue
			}
			report := p.GetFleetIntel(job.Enemy, job.Target)
			valid := report != nil && !report.Delete && (report.ReportAge == 0 && (job.Watching || unknownFleetEquipment(p, report)) || job.Minelayer && trackableMinelayer(p, report))
			if valid && len(job.Designs) > 0 {
				for _, token := range report.Tokens {
					if !slices.Contains(job.Designs, token.DesignNum) {
						valid = false
						break
					}
				}
			}
			if valid {
				continue
			}
			delete(f.Tags, scoutReconTag)
			delete(f.Tags, preyWatchTag)
			delete(f.Tags, scoutTag)
			if !f.InAutomaticSafety() && !f.InDesignMaintenance() && !fuelNetworkManaged(f) && f.GetTag(tankerAwaitTag) == "" {
				holdAutomaticFleet(p, f, "Reconnaissance target changed; selecting fresh reports")
			}
		}
	}
}
