package cs

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const frontierPatrolTag = "frontier-patrol"
const declinedPatrolTag = "patrol-declined-target"

const patrolSectorTag = "patrol-sector"

// Stable fuel bases anchor patrol coverage. The first assignment favors the
// ship's local base; uncovered sectors receive surplus ships, not every ship.
func (t *turnGenerator) patrolFrontier(p *Player, f *Fleet) bool {
	owned := t.game.getPlanets(p.Num)
	bases := []*Planet{}
	for _, planet := range owned {
		if planet.Spec.HasStarbase {
			bases = append(bases, planet)
		}
	}
	if len(bases) == 0 {
		bases = owned
	}
	if len(bases) == 0 {
		return false
	}
	sort.Slice(bases, func(i, j int) bool { return bases[i].Num < bases[j].Num })
	sector, _ := strconv.Atoi(f.GetTag(patrolSectorTag))
	counts := map[int]int{}
	role := p.Automation.FleetRoles[f.Num]
	for _, other := range t.game.getFleets(p.Num) {
		if other == f || other.Delete || p.Automation.FleetRoles[other.Num] != role {
			continue
		}
		n, _ := strconv.Atoi(other.GetTag(patrolSectorTag))
		if n != 0 {
			counts[n]++
		}
	}
	var anchor *Planet
	nearest := math.Inf(1)
	for _, base := range bases {
		d := f.Position.DistanceTo(base.Position)
		if base.Num == sector {
			anchor = base
		}
		if sector == 0 && d < nearest {
			nearest = d
			anchor = base
		}
	}
	if anchor == nil {
		sector = 0
		nearest = math.Inf(1)
		for _, base := range bases {
			if d := f.Position.DistanceTo(base.Position); d < nearest {
				nearest = d
				anchor = base
			}
		}
	}
	localEnemy := false
	for _, enemy := range p.FleetIntels {
		if enemy != nil && !enemy.Delete && enemy.ReportAge == 0 && enemy.PlayerNum != p.Num && !p.IsFriend(enemy.PlayerNum) && enemy.Position.DistanceTo(anchor.Position) <= 100 {
			localEnemy = true
			break
		}
	}
	// Explicit planet priorities also allocate nearby defensive/escort cover.
	// A marked enemy world anchors at our nearest fuel base, never at a hostile
	// orbit merely because it was marked. Local contacts still take precedence.
	priorities := map[int]int{}
	for _, planet := range p.PlanetIntels {
		if planet == nil || !p.Automation.PriorityPlanets[planet.Num] {
			continue
		}
		var closest *Planet
		distance := math.Inf(1)
		for _, base := range bases {
			if d := base.Position.DistanceTo(planet.Position); d < distance {
				distance = d
				closest = base
			}
		}
		if closest != nil {
			priorities[closest.Num]++
		}
	}
	if !localEnemy {
		var uncovered *Planet
		cost := math.Inf(1)
		for _, base := range bases {
			if base == anchor {
				continue
			}
			missingCoverage := counts[anchor.Num] >= 1 && counts[base.Num] == 0
			priorityCoverage := priorities[base.Num] > 0 && float64(counts[base.Num]+1)/float64(1+priorities[base.Num]) < float64(counts[anchor.Num]+1)/float64(1+priorities[anchor.Num])
			if !missingCoverage && !priorityCoverage {
				continue
			}
			d := (1 + f.Position.DistanceTo(base.Position)) / float64(1+priorities[base.Num])
			if d < cost {
				cost = d
				uncovered = base
			}
		}
		if uncovered != nil {
			anchor = uncovered
		}
	}
	changed := sector != anchor.Num
	f.SetTag(patrolSectorTag, strconv.Itoa(anchor.Num))
	if !changed && f.GetTag(frontierPatrolTag) != "" && len(f.Waypoints) > 1 {
		return true
	}
	// A remote anchor is reached first. Once local, fly short economical loops
	// with enough reserve for the complete return to a real fuel dock.
	pos := anchor.Position
	target := anchor.ToTarget()
	if f.Position.DistanceTo(anchor.Position) <= 48 {
		cycle := readPatrolJob(f).Cycle
		if f.Idle() {
			cycle++
			job := readPatrolJob(f)
			job.Cycle = cycle
			savePatrolJob(f, job)
		}
		angle := float64((f.Num*7+cycle)%12) * math.Pi / 6
		pos.X += int(math.Round(24 * math.Cos(angle)))
		pos.Y += int(math.Round(24 * math.Sin(angle)))
		target = MapObjectTarget{}
	}
	route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, pos, target, automaticRoutePolicy{EconomicalPatrol: true, MaxWarp: min(4, automaticMaxWarp(p, f)), MinWarp: 4, MaxTurns: 20, AllowThreat: combatAutonomy(p, f)})
	if route == nil || route.WaitForFuel {
		delete(f.Tags, frontierPatrolTag)
		return false
	}
	applyExpeditionRoute(p, f, route)
	f.SetTag(frontierPatrolTag, "Patrolling sector around "+anchor.Name+"; fuel, repairs and local defense available")
	return true
}
func (t *turnGenerator) declinePatrolTarget(p *Player, f *Fleet) {
	job := readPatrolJob(f)
	for _, member := range t.game.getFleets(p.Num) {
		if !member.Delete && readPatrolJob(member).Leader == f.Num {
			member.SetTag(declinedPatrolTag, fmt.Sprintf("%d:%d:%d", job.Enemy, job.Target, t.game.Year+2))
		}
	}
	f.SetTag(declinedPatrolTag, fmt.Sprintf("%d:%d:%d", job.Enemy, job.Target, t.game.Year+2))
	t.releasePatrol(p, f)
}
func declinedPatrolTarget(f *Fleet, enemy *Fleet, year int) bool {
	var owner, num, until int
	_, err := fmt.Sscanf(f.GetTag(declinedPatrolTag), "%d:%d:%d", &owner, &num, &until)
	return err == nil && owner == enemy.PlayerNum && num == enemy.Num && until >= year
}
