package cs

import (
	"fmt"
	"math"
	"sort"
)

// This is a forecast, never permission to colonize an occupied planet. Only
// fresh reported populations and our already dispatched normal bombs are used.
func (t *turnGenerator) estimatedBombingClearance(p *Player, planet *Planet) int {
	if planet == nil || planet.ReportAge > 2 || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) || planet.Spec.HasStarbase {
		return 0
	}
	rate, floor, arrival := 0.0, 0, 0
	for _, f := range t.game.getFleets(p.Num) {
		if f.Delete || p.Automation.FleetRoles[f.Num] != "bomber" || f.InAutomaticSafety() || f.InDesignMaintenance() || bomberGoal(f) != planet.Num {
			continue
		}
		if f.Position != planet.Position {
			if len(f.Waypoints) < 2 {
				continue
			}
			arrival = max(arrival, int(math.Ceil(f.Position.DistanceTo(planet.Position)/max(1, f.StrategicSpeed(f.Waypoints[1].WarpSpeed)))))
		}
		for _, bomb := range f.Spec.Bombs {
			rate += bomb.KillRate * float64(bomb.Quantity) / 100
			floor += bomb.MinKillRate * bomb.Quantity / 100
		}
	}
	if floor <= 0 {
		return 0
	} // Smart bombs alone cannot empty the last population.
	pop := float64(planet.Cargo.Colonists) * math.Pow(1.25, float64(arrival))
	survival := 1 - Clamp(planet.Spec.DefenseCoverage, 0.0, 1.0)
	for year := 1; year+arrival <= 12; year++ {
		pop -= max(1, float64(floor)*survival, math.Floor(pop*rate*survival))
		if pop <= 0 {
			return arrival + year
		}
		pop *= 1.25 // Conservative upper growth bound, not hidden defender production.
	}
	return 0
}

func (t *turnGenerator) stageColonizerForBombing(p *Player, f *Fleet, job *colonizationJob) bool {
	if f.Cargo.Colonists <= 0 {
		return false
	}
	candidates := []*Planet{}
	seen := map[int]bool{}
	for _, bomber := range t.game.getFleets(p.Num) {
		if bomber.Delete || p.Automation.FleetRoles[bomber.Num] != "bomber" {
			continue
		}
		num := bomberGoal(bomber)
		target := p.GetPlanetIntel(num)
		if target != nil && !seen[num] {
			candidates = append(candidates, target)
			seen[num] = true
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if a, b := expansionPriority(p, candidates[i]), expansionPriority(p, candidates[j]); a != b {
			return a > b
		}
		return candidates[i].Num < candidates[j].Num
	})
	for _, target := range candidates {
		clearance := t.estimatedBombingClearance(p, target)
		if clearance == 0 || prospectivePlanetQuality(p, target) <= 0 {
			continue
		}
		reserved := false
		for _, other := range t.game.getFleets(p.Num) {
			saved := readColonizationJob(other)
			if other != f && !other.Delete && saved != nil && saved.Kind == "preposition" && saved.TargetNum == target.Num {
				reserved = true
				break
			}
		}
		if reserved {
			continue
		}
		var stage *Planet
		var route *expeditionRoute
		for _, dock := range expeditionStations(p, f) {
			if dock.Position.DistanceTo(target.Position) > 120 || stage != nil && stage.Position.DistanceTo(target.Position) <= dock.Position.DistanceTo(target.Position) {
				continue
			}
			candidate := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, dock.Position, dock.ToTarget(), automaticRoutePolicy{MaxTurns: 12})
			if candidate != nil && !candidate.WaitForFuel && clearance <= candidate.Turns+3 {
				stage, route = dock, candidate
			}
		}
		if stage == nil {
			continue
		}
		delete(f.Tags, logisticsTag)
		t.releaseEscorts(p, f)
		applyExpeditionRoute(p, f, route)
		job.Kind, job.TargetNum = "preposition", target.Num
		job.Status = fmt.Sprintf("Staging at friendly %s for %s; estimated bombardment clearance in %d years. Colonize only after confirmed evacuation", stage.Name, target.Name, clearance)
		writeColonizationJob(f, job)
		return true
	}
	return false
}
