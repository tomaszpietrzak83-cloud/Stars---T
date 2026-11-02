package cs

import (
	"slices"
	"sort"
)

// Report ownership and headings cannot change during a sort. Cache only within
// this call: later planning passes must observe new reports and planet owners.
func sortedPatrolTargets(player *Player, fleet *Fleet) []*Fleet {
	enemies := slices.Clone(player.FleetIntels)
	approaching := make(map[*Fleet]bool)
	approaches := func(enemy *Fleet) bool {
		if value, ok := approaching[enemy]; ok {
			return value
		}
		value := enemyApproachingColony(player, enemy)
		approaching[enemy] = value
		return value
	}
	sort.SliceStable(enemies, func(i, j int) bool {
		if enemies[i] == nil {
			return false
		}
		if enemies[j] == nil {
			return true
		}
		ri, rj := patrolTargetPriority(player, enemies[i], approaches(enemies[i])), patrolTargetPriority(player, enemies[j], approaches(enemies[j]))
		if ri != rj {
			return ri < rj
		}
		ib, jb := patrolReportedBomber(player, enemies[i]), patrolReportedBomber(player, enemies[j])
		if ib != jb {
			return ib
		}
		ii, ji := approaches(enemies[i]), approaches(enemies[j])
		if ii != ji {
			return ii
		}
		return fleet.Position.DistanceTo(enemies[i].Position) < fleet.Position.DistanceTo(enemies[j].Position)
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
