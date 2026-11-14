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
	bomberTargets := map[int]bool{}
	for _, bomber := range t.game.getFleets(p.Num) {
		if automaticBomberAvailable(p, bomber) {
			orders := readBomberOrders(bomber)
			if orders.Stage == "recon" {
				bomberTargets[bomberGoal(bomber)] = true
			}
			if orders.ReserveTarget > 0 {
				bomberTargets[orders.ReserveTarget] = true
			}
		}
	}
	for _, planet := range p.PlanetIntels {
		if planet == nil {
			continue
		}
		bomberRecon := bomberTargets[planet.Num] && bomberNeedsOrbitalRecon(p, planet)
		if planet.PlayerNum == p.Num || planet.Owned() && p.IsFriend(planet.PlayerNum) || !bomberRecon && planet.ReportAge <= 2 && !recaptureEquipmentMissing(p, planet) || planet.Unexplored() || !planet.Owned() && !p.Automation.PriorityPlanets[planet.Num] {
			continue
		}
		if priorityOnly && !bomberRecon && !p.Automation.PriorityPlanets[planet.Num] {
			continue
		}
		if !bomberRecon && !p.Automation.PriorityPlanets[planet.Num] && (f.Spec.ScanRangePen <= 2 || planet.ReportAge < 5) {
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
		if bomberRecon {
			value /= 1e6
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
