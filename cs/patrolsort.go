package cs

import (
	"slices"
	"sort"
)

// Report ownership and headings cannot change during a sort. Cache only within
// this call: later planning passes must observe new reports and planet owners.
func sortedPatrolTargets(player *Player, fleet *Fleet) []*Fleet {
	enemies := slices.Clone(player.FleetIntels)
	type targetRank struct {
		priority, deadline  int
		bomber, approaching bool
		distance            float64
	}
	ranks := make(map[*Fleet]targetRank, len(enemies))
	rank := func(enemy *Fleet) targetRank {
		if value, ok := ranks[enemy]; ok {
			return value
		}
		value := targetRank{approaching: enemyApproachingColony(player, enemy)}
		value.priority = patrolTargetPriority(player, enemy, value.approaching)
		if player.patrolEvaluator != nil {
			_, value.deadline = patrolColonyDeadline(player, enemy)
		}
		value.bomber = patrolReportedBomber(player, enemy)
		value.distance = fleet.Position.DistanceTo(enemy.Position)
		ranks[enemy] = value
		return value
	}
	sort.SliceStable(enemies, func(i, j int) bool {
		if enemies[i] == nil {
			return false
		}
		if enemies[j] == nil {
			return true
		}
		ri, rj := rank(enemies[i]), rank(enemies[j])
		if ri.priority != rj.priority {
			return ri.priority < rj.priority
		}
		if player.patrolEvaluator != nil {
			di, dj := ri.deadline, rj.deadline
			if di != dj {
				return di < dj
			}
		}
		ib, jb := ri.bomber, rj.bomber
		if ib != jb {
			return ib
		}
		ii, ji := ri.approaching, rj.approaching
		if ii != ji {
			return ii
		}
		return ri.distance < rj.distance
	})
	return enemies
}

// Hull reports identify potential landing transports without reading private
// colonist manifests or enemy orders. Their public heading signals the threat.
func patrolReportedTransport(p *Player, f *Fleet) bool {
	if f == nil {
		return false
	}
	for _, token := range f.Tokens {
		if d := p.GetShipDesignIntel(f.PlayerNum, token.DesignNum); d != nil {
			if h := StaticTechStore.GetHull(d.Hull); h != nil && (h.Type == TechHullTypeFreighter || h.Type == TechHullTypeColonizer || h.Type == TechHullTypeMultiPurposeFreighter) {
				return true
			}
		}
	}
	return f.Spec.HullType == TechHullTypeFreighter || f.Spec.HullType == TechHullTypeColonizer || f.Spec.HullType == TechHullTypeMultiPurposeFreighter
}
func patrolTargetPriority(p *Player, f *Fleet, approaching bool) int {
	bombing, deadline := patrolColonyDeadline(p, f)
	if bombing {
		return -4
	}
	if approaching && deadline < 1000 && patrolReportedBomber(p, f) {
		return -3
	}
	if priorityFleet(p, f) {
		return -2
	}
	if slowPreySupport(p, f) != nil {
		return -1
	}
	if patrolReportedTransport(p, f) {
		if approaching {
			return 0
		}
		return 2
	}
	if approaching && patrolReportedBomber(p, f) {
		return 1
	}
	if f.Spec.HullType == TechHullTypeScout {
		return 3
	}
	for _, token := range f.Tokens {
		if d := p.GetShipDesignIntel(f.PlayerNum, token.DesignNum); d != nil {
			if h := StaticTechStore.GetHull(d.Hull); h != nil && h.Type == TechHullTypeScout {
				return 3
			}
		}
	}
	return 4
}
func nearbyLandingThreat(p *Player, f *Fleet) bool {
	for _, enemy := range p.FleetIntels {
		if enemy != nil && !enemy.Delete && enemy.ReportAge == 0 && enemy.PlayerNum != p.Num && !p.IsFriend(enemy.PlayerNum) && f.Position.DistanceTo(enemy.Position) <= 150 && patrolReportedTransport(p, enemy) && enemyApproachingColony(p, enemy) {
			return true
		}
	}
	return false
}
