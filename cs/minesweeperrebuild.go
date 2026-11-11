package cs

import "slices"

// Leave one assigned sweeper on each known active field, then give first
// access to the remaining rebuild slots to unassigned fleets.
func (t *turnGenerator) deferMinesweeperRebuilds(p *Player, deferred map[*Fleet]bool) {
	fields := map[string]bool{}
	for _, field := range p.MinefieldIntels {
		if reportedHostileMinefield(p, field) {
			fields[minefieldAssignmentKey(field)] = true
		}
	}
	if len(fields) == 0 {
		return
	}
	var candidates []*Fleet
	covered := map[string]bool{}
	total, rebuilding := 0, 0
	for _, f := range t.game.getFleets(p.Num) {
		if f.Delete || !automaticMinesweeper(p, f) {
			continue
		}
		total++
		if f.InDesignMaintenance() {
			rebuilding++
			continue
		}
		if f.InAutomaticSafety() || p.FleetInActiveAssembly(f) || fuelNetworkManaged(f) {
			continue
		}
		if t.maintenanceNeeded(p, f) && hasShipsToReplace(p, f) {
			candidates = append(candidates, f)
		} else {
			covered[f.GetTag(minesweeperTargetTag)] = true
		}
	}
	slices.SortStableFunc(candidates, func(a, b *Fleet) int {
		if fields[a.GetTag(minesweeperTargetTag)] != fields[b.GetTag(minesweeperTargetTag)] {
			if fields[a.GetTag(minesweeperTargetTag)] {
				return 1
			}
			return -1
		}
		return a.Num - b.Num
	})
	slots := max(0, total/2-rebuilding)
	for _, f := range candidates {
		key := f.GetTag(minesweeperTargetTag)
		if fields[key] && !covered[key] || slots == 0 || len(f.Waypoints) > 1 {
			deferred[f] = true
			covered[key] = true
			continue
		}
		slots--
	}
}
