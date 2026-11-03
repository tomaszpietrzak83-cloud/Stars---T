package cs

import (
	"math"
	"slices"
)

type PatrolContactMemory struct {
	Enemy, Target, Seen, LastSearch, Misses int
	Position                                Vector
	Velocity                                VectorFloat64
	Designs                                 []int
}

func rememberPatrolReports(p *Player, year int) {
	for key, memory := range p.Automation.PatrolMemory {
		if year-memory.Seen > 12 || p.IsFriend(memory.Enemy) {
			delete(p.Automation.PatrolMemory, key)
		}
	}
	for _, report := range p.FleetIntels {
		if report == nil || report.Delete || report.Starbase || report.ReportAge != 0 || report.PlayerNum == p.Num || p.IsFriend(report.PlayerNum) || report.WarpSpeed > 2 || reportedFleetShips(report) < 8 {
			continue
		}
		if p.Automation.PatrolMemory == nil {
			p.Automation.PatrolMemory = map[string]PatrolContactMemory{}
		}
		memory := PatrolContactMemory{Enemy: report.PlayerNum, Target: report.Num, Seen: year, Position: report.Position, Velocity: report.Heading.Normalized().Scale(report.observedStrategicSpeed())}
		for _, token := range report.Tokens {
			memory.Designs = append(memory.Designs, token.DesignNum)
		}
		p.Automation.PatrolMemory[priorityFleetKey(report.PlayerNum, report.Num)] = memory
	}
}

const patrolSearchTag = "patrol-contact-search"

// A memory authorizes only a funded safe scanner search. Fresh reports are
// always required again for equipment assessment and interception.
func (t *turnGenerator) planLostPatrolSearch(p *Player, scout *Fleet) bool {
	if !scoutReconEligible(p, scout) || scout.Spec.ScanRange <= 0 {
		return false
	}
	keys := make([]string, 0, len(p.Automation.PatrolMemory))
	for key := range p.Automation.PatrolMemory {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var selected string
	var best *expeditionRoute
	value := math.Inf(1)
	for _, key := range keys {
		memory := p.Automation.PatrolMemory[key]
		if report := p.GetFleetIntel(memory.Enemy, memory.Target); report != nil && !report.Delete && report.ReportAge == 0 {
			continue
		}
		claimed := false
		for _, f := range t.game.getFleets(p.Num) {
			if f != scout && !f.Delete && f.GetTag(patrolSearchTag) == key {
				claimed = true
				break
			}
		}
		if claimed {
			continue
		}
		age := max(1, t.game.Year-1-memory.Seen)
		center := memory.Position.ToFloat64().Add(memory.Velocity.Scale(float64(age))).ToInt(true)
		// Expand a deterministic search ring. Observed slow speed never proves
		// zero fuel; missing contact remains uncertain rather than destroyed.
		radius := min(float64(age)*81, float64(max(16, scout.Spec.ScanRange))*float64(memory.Misses+1)/2)
		angle := float64(memory.Misses%8) * math.Pi / 4
		point := center
		if memory.Misses > 0 {
			point = Vector{center.X + int(radius*math.Cos(angle)), center.Y + int(radius*math.Sin(angle))}
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{scout}, point, MapObjectTarget{}, automaticRoutePolicy{Fast: true, MaxTurns: 8})
		if route == nil || route.WaitForFuel {
			continue
		}
		score := float64(route.Turns)*100 + float64(age)*10
		if score < value {
			selected, best, value = key, route, score
		}
	}
	if selected == "" {
		delete(scout.Tags, patrolSearchTag)
		return false
	}
	memory := p.Automation.PatrolMemory[selected]
	if scout.Position.DistanceTo(best.Waypoints[len(best.Waypoints)-1].Position) <= float64(scout.Spec.ScanRange)/2 && memory.LastSearch < t.game.Year {
		memory.Misses++
		memory.LastSearch = t.game.Year
		p.Automation.PatrolMemory[selected] = memory
	}
	delete(scout.Tags, scoutReconTag)
	scout.SetTag(patrolSearchTag, selected)
	scout.SetTag(scoutTag, "Searching the last reported region of a lost slow formation")
	applyExpeditionRoute(p, scout, best)
	return true
}

func unknownDesignOverlap(p *Player, a, b *Fleet) bool {
	if a == nil || b == nil || a.PlayerNum != b.PlayerNum {
		return false
	}
	for _, x := range a.Tokens {
		if design := p.GetShipDesignIntel(a.PlayerNum, x.DesignNum); design != nil && len(design.Slots) > 0 {
			continue
		}
		for _, y := range b.Tokens {
			if x.DesignNum == y.DesignNum {
				return true
			}
		}
	}
	return false
}
