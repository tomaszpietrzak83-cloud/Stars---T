package cs

import (
	"math"
	"slices"
)

// A single public-information priority view is consumed by combat, scouts,
// search and collection. Capabilities remain distinct, so reconnaissance and
// concentration proceed concurrently without stealing deliveries or repairs.
type patrolMission struct {
	Key, Kind                            string
	Enemy, Target, Tier, Deadline, Value int
	Territorial                          bool
}

// Match the colony influence shown on the scanner. For a hostile base, use
// the surrounding colonies so the target's own influence doesn't mask a salient.
func patrolOwnTerritory(p *Player, point Vector, excludePlanet int) bool {
	influence := map[int]float64{}
	for _, colony := range p.PlanetIntels {
		if colony == nil || !colony.Owned() || colony.Unexplored() || colony.Num == excludePlanet {
			continue
		}
		distance := colony.Position.DistanceTo(point)
		distance *= distance
		if distance >= 105*105 {
			continue
		}
		broad := 1 - float64(distance)/(105*105)
		core := max(0, 1-float64(distance)/(23*23))
		influence[colony.PlayerNum] += broad*broad + 2.5*core*core
	}
	rival := 0.0
	for owner, strength := range influence {
		if owner != p.Num {
			rival = max(rival, strength)
		}
	}
	return influence[p.Num] > rival+.12
}

func patrolColonyDeadline(p *Player, f *Fleet) (bool, int) {
	return patrolColonyDeadlineAmong(p, f, p.PlanetIntels, patrolDefendedColony(p, f))
}

func patrolColonyDeadlineAmong(p *Player, f *Fleet, colonies []*Planet, defended *Planet) (bool, int) {
	for _, planet := range colonies {
		if planet != nil && planet.OwnedBy(p.Num) && planet.Position == f.Position {
			if patrolReportedBomber(p, f) {
				return true, 0
			}
			// A transport already over our colony can unload an invasion even
			// when stationary. Use reported capacity, never hidden enemy cargo.
			if patrolReportedTransport(p, f) || patrolReportedCombatant(p, f) {
				return false, 0
			}
		}
	}
	if planet := defended; planet != nil {
		return false, int(math.Ceil(f.Position.DistanceTo(planet.Position) / max(1, f.observedStrategicSpeed())))
	}
	return false, 1000
}

func makePatrolMissionQueue(p *Player, year int) []patrolMission {
	result := []patrolMission{}
	seen := map[string]bool{}
	for _, report := range p.FleetIntels {
		if report == nil || report.Delete || report.Starbase || report.ReportAge != 0 || report.PlayerNum == p.Num || p.IsFriend(report.PlayerNum) {
			continue
		}
		key := priorityFleetKey(report.PlayerNum, report.Num)
		if seen[key] {
			continue
		}
		seen[key] = true
		bombing, deadline := patrolColonyDeadline(p, report)
		tier := 3
		if bombing {
			tier = 0
		} else if deadline < 1000 {
			tier = 1
		}
		territorial := patrolOwnTerritory(p, report.Position, 0)
		if tier == 3 && priorityFleet(p, report) {
			tier = 2
		}
		base := patrolMission{Key: key, Kind: "attack", Enemy: report.PlayerNum, Target: report.Num, Tier: tier, Deadline: deadline, Value: reportedFleetShips(report), Territorial: territorial}
		result = append(result, base)
		if unknownFleetEquipment(p, report) {
			base.Kind = "recon"
			result = append(result, base)
		}
	}
	for key, memory := range p.Automation.PatrolMemory {
		if !seen[key] {
			result = append(result, patrolMission{Key: key, Kind: "search", Enemy: memory.Enemy, Target: memory.Target, Tier: 4, Deadline: memory.Seen + 12 - year})
		}
	}
	for _, field := range p.SalvageIntels {
		if field != nil && !field.Delete && field.Cargo.ToMineral().Total() > 0 {
			result = append(result, patrolMission{Kind: "collect", Target: field.Num, Tier: 4, Deadline: 1, Value: field.Cargo.ToMineral().Total()})
		}
	}
	slices.SortStableFunc(result, func(a, b patrolMission) int {
		if a.Tier == 3 && b.Tier == 3 && a.Territorial != b.Territorial {
			if a.Territorial {
				return -1
			}
			return 1
		}
		for _, d := range []int{a.Tier - b.Tier, a.Deadline - b.Deadline, b.Value - a.Value, a.Enemy - b.Enemy, a.Target - b.Target} {
			if d != 0 {
				return d
			}
		}
		if a.Kind < b.Kind {
			return -1
		}
		if a.Kind > b.Kind {
			return 1
		}
		return 0
	})
	return result
}

func patrolMissionRank(p *Player, enemy *Fleet) int {
	for i, task := range p.patrolQueue {
		if task.Enemy == enemy.PlayerNum && task.Target == enemy.Num && task.Kind == "attack" {
			return i
		}
	}
	return len(p.patrolQueue)
}
