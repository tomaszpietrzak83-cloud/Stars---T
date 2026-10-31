package cs

import (
	"fmt"
	"math"
	"sort"
)

const frontierPatrolTag = "frontier-patrol"
const declinedPatrolTag = "patrol-declined-target"

func (t *turnGenerator) patrolFrontier(p *Player, f *Fleet) bool {
	if f.GetTag(frontierPatrolTag) != "" && len(f.Waypoints) > 1 {
		return true
	}
	owned := t.game.getPlanets(p.Num)
	if len(owned) == 0 {
		return false
	}
	home := owned[0].Position
	for _, planet := range owned {
		if planet.Homeworld {
			home = planet.Position
			break
		}
	}
	type candidate struct {
		pos   Vector
		score float64
	}
	candidates := []candidate{}
	seen := map[Vector]bool{}
	add := func(pos Vector) {
		if pos == f.Position || seen[pos] {
			return
		}
		seen[pos] = true
		value := 1 + home.DistanceTo(pos)/50
		for _, enemy := range p.FleetIntels {
			if enemy != nil && enemy.ReportAge == 0 && enemy.PlayerNum != p.Num && !p.IsFriend(enemy.PlayerNum) {
				value += 4 / (1 + pos.DistanceTo(enemy.Position)/100)
			}
		}
		for _, other := range t.game.getFleets(p.Num) {
			if other != f && !other.Delete && p.Automation.FleetRoles[other.Num] == "patrol" && len(other.Waypoints) > 1 {
				goal := other.Waypoints[len(other.Waypoints)-1].Position
				value /= 1 + 5*math.Exp(-float64(pos.DistanceSquaredTo(goal))/6400)
			}
		}
		candidates = append(candidates, candidate{pos, value / (1 + f.Position.DistanceTo(pos)/800)})
	}
	for _, planet := range owned {
		add(planet.Position)
	}
	for _, planet := range p.PlanetIntels {
		if planet == nil || planet.Owned() || home.DistanceTo(planet.Position) < 60 {
			continue
		}
		near := false
		for _, colony := range owned {
			if colony.Position.DistanceTo(planet.Position) <= 120 {
				near = true
				break
			}
		}
		if near {
			add(planet.Position)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	f.SetTag(frontierPatrolTag, "Economical frontier patrol; whole return budget retained")
	for _, candidate := range candidates[:min(32, len(candidates))] {
		allow := patrolPower(f) > knownRouteThreat(p, f.Position, candidate.pos)*3/2
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, candidate.pos, MapObjectTarget{}, automaticRoutePolicy{EconomicalPatrol: true, MaxWarp: min(4, automaticMaxWarp(p, f)), MinWarp: 4, MaxTurns: 20, AllowThreat: allow})
		if route != nil && !route.WaitForFuel {
			applyExpeditionRoute(p, f, route)
			return true
		}
	}
	delete(f.Tags, frontierPatrolTag)
	return false
}
func (t *turnGenerator) declinePatrolTarget(p *Player, f *Fleet) {
	job := readPatrolJob(f)
	f.SetTag(declinedPatrolTag, fmt.Sprintf("%d:%d:%d", job.Enemy, job.Target, t.game.Year+2))
	t.releasePatrol(p, f)
}
func declinedPatrolTarget(f *Fleet, enemy *Fleet, year int) bool {
	var owner, num, until int
	_, err := fmt.Sscanf(f.GetTag(declinedPatrolTag), "%d:%d:%d", &owner, &num, &until)
	return err == nil && owner == enemy.PlayerNum && num == enemy.Num && until >= year
}
