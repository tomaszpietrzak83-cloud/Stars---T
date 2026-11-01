package cs

import (
	"math"
	"strconv"
)

const priorityReconTag = "priority-colony-recon"

// One expendable scout refreshes each marked colony. The destination comes
// only from this player's reports; routes still reserve a viable return.
func (t *turnGenerator) planPriorityColonyRecon(p *Player, f *Fleet) bool {
	if !scoutReconEligible(p, f) {
		return false
	}
	var best *Planet
	var route *expeditionRoute
	score := math.Inf(1)
	for _, planet := range p.PlanetIntels {
		if planet == nil || !p.Automation.PriorityPlanets[planet.Num] || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) || planet.ReportAge <= 2 {
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
		candidate := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, planet.Position, planet.ToTarget(), automaticRoutePolicy{AllowThreat: true, MinWarp: automaticCruiseWarp(p, f), MaxTurns: 8})
		if candidate == nil || candidate.WaitForFuel {
			continue
		}
		value := float64(candidate.Turns)*1000 + f.Position.DistanceTo(planet.Position)
		if value < score || value == score && (best == nil || planet.Num < best.Num) {
			best, route, score = planet, candidate, value
		}
	}
	if best == nil {
		delete(f.Tags, priorityReconTag)
		return false
	}
	f.SetTag(priorityReconTag, strconv.Itoa(best.Num))
	f.SetTag(scoutTag, "Refreshing intelligence at priority enemy colony "+best.Name+"; return fuel reserved")
	applyExpeditionRoute(p, f, route)
	return true
}
