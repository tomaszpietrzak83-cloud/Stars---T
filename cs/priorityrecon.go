package cs

import (
	"math"
	"strconv"
)

const priorityReconTag = "priority-colony-recon"

// One expendable scout refreshes each marked colony. The destination comes
// only from this player's reports; routes still reserve a viable return.
func (t *turnGenerator) planPriorityColonyRecon(p *Player, f *Fleet) bool {
	return t.planColonyRecon(p, f, true)
}

func (t *turnGenerator) planColonyRecon(p *Player, f *Fleet, priorityOnly bool) bool {
	if !scoutReconEligible(p, f) {
		return false
	}
	var best *Planet
	var route *expeditionRoute
	score := math.Inf(1)
	for _, planet := range p.PlanetIntels {
		if planet == nil || planet.PlayerNum == p.Num || planet.Owned() && p.IsFriend(planet.PlayerNum) || planet.ReportAge <= 2 && !recaptureEquipmentMissing(p, planet) || planet.Unexplored() || !planet.Owned() && !p.Automation.PriorityPlanets[planet.Num] {
			continue
		}
		if priorityOnly && !p.Automation.PriorityPlanets[planet.Num] {
			continue
		}
		if !p.Automation.PriorityPlanets[planet.Num] && (f.Spec.ScanRangePen <= 2 || planet.ReportAge < 5) {
			continue
		}
		claimed := false
		for _, other := range t.game.getFleets(p.Num) {
			if other != f && !other.Delete && p.Automation.FleetRoles[other.Num] == "scout" && other.GetTag(priorityReconTag) == strconv.Itoa(planet.Num) && len(other.Waypoints) > 1 {
				claimed = true
				break
			}
		}
		if claimed {
			continue
		}
		position := scoutColonyScanPoint(p, f, planet)
		target := planet.ToTarget()
		if position != planet.Position {
			target = MapObjectTarget{}
		}
		candidate := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, target, automaticRoutePolicy{AllowThreat: true, MinWarp: automaticCruiseWarp(p, f), MaxTurns: 8})
		if candidate == nil || candidate.WaitForFuel {
			continue
		}
		value := float64(candidate.Turns)*1000 + f.Position.DistanceTo(planet.Position)
		// Routine refresh must not monopolize scouts still discovering worlds.
		if !p.Automation.PriorityPlanets[planet.Num] {
			value += 100000 / float64(min(100, planet.ReportAge))
		}
		for _, bomber := range t.game.getFleets(p.Num) {
			if !bomber.Delete && p.Automation.FleetRoles[bomber.Num] == "bomber" && bomberGoal(bomber) == planet.Num {
				value /= 1e6
				break
			}
		}
		if value < score || value == score && (best == nil || planet.Num < best.Num) {
			best, route, score = planet, candidate, value
		}
	}
	if best == nil {
		delete(f.Tags, priorityReconTag)
		return false
	}
	delete(f.Tags, scoutReconTag)
	f.SetTag(priorityReconTag, strconv.Itoa(best.Num))
	f.SetTag(scoutTag, "Refreshing intelligence at enemy colony "+best.Name+"; return fuel reserved")
	applyExpeditionRoute(p, f, route)
	return true
}
