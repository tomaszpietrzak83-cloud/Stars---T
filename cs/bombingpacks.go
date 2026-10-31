package cs

import (
	"encoding/json"
	"slices"
	"strconv"
)

const bombingPackTag = "bombing-pack"
const bombingRallyTag = "bombing-rally"

type attackPackPart struct {
	Design, Count int
	Role          string
}
type bombingPack struct {
	Target int
	Parts  []attackPackPart
}
type bombingRally struct{ Leader, Target, Until int }

func readBombingPack(f *Fleet) *bombingPack {
	var pack bombingPack
	if json.Unmarshal([]byte(f.GetTag(bombingPackTag)), &pack) != nil {
		return nil
	}
	return &pack
}
func attackPackParts(p *Player, f *Fleet) []attackPackPart {
	if pack := readBombingPack(f); pack != nil {
		return slices.Clone(pack.Parts)
	}
	role := p.Automation.FleetRoles[f.Num]
	var borrowed bomberBorrow
	if json.Unmarshal([]byte(f.GetTag(bomberBorrowTag)), &borrowed) == nil {
		role = borrowed.Role
	}
	parts := []attackPackPart{}
	for _, token := range f.Tokens {
		parts = append(parts, attackPackPart{token.DesignNum, token.Quantity, role})
	}
	return parts
}
func (t *turnGenerator) dissolveBombingPack(p *Player, leader *Fleet, pack *bombingPack) {
	children, err := NewOrderer().SplitAll(&t.game.Rules, p, t.game.getFleets(p.Num), leader)
	if err != nil {
		return
	}
	for _, child := range children {
		t.game.Fleets = append(t.game.Fleets, child)
		_ = t.game.Universe.addFleet(child)
	}
}
func restoreBombingPackRoles(p *Player, leader *Fleet, children []*Fleet, pack *bombingPack) {
	for _, f := range append([]*Fleet{leader}, children...) {
		delete(f.Tags, bombingPackTag)
		delete(f.Tags, bombingRallyTag)
		delete(f.Tags, bomberTag)
		delete(f.Tags, bomberBorrowTag)
		delete(f.Tags, escortTag)
		role := "bomber"
		for _, token := range f.Tokens {
			for i := range pack.Parts {
				part := &pack.Parts[i]
				if part.Design == token.DesignNum && part.Count > 0 {
					role = part.Role
					part.Count -= token.Quantity
					break
				}
			}
		}
		if role == "" {
			delete(p.Automation.FleetRoles, f.Num)
		} else {
			p.Automation.FleetRoles[f.Num] = role
		}
		holdAutomaticFleet(p, f, "Wolf pack dissolved; original automatic role restored")
	}
}
func (t *turnGenerator) finishBombingPacks() {
	for _, p := range t.game.Players {
		for _, f := range slices.Clone(t.game.getFleets(p.Num)) {
			pack := readBombingPack(f)
			if pack == nil || f.Delete || f.InDesignMaintenance() || f.InAutomaticSafety() {
				continue
			}
			target := p.GetPlanetIntel(pack.Target)
			done := target == nil || !target.Owned() || target.PlayerNum == p.Num || p.IsFriend(target.PlayerNum) || p.Automation.PriorityPlanets[pack.Target] && target.Cargo.Colonists <= 1000 && target.Defenses == 0 && !target.Spec.HasStarbase
			if done || p.Automation.FleetRoles[f.Num] != "bomber" {
				t.dissolveBombingPack(p, f, pack)
			}
		}
	}
}
func bomberGoal(f *Fleet) int {
	var orders bomberOrders
	if json.Unmarshal([]byte(f.GetTag(bomberTag)), &orders) == nil && len(orders.Orders.Waypoints) > 1 {
		return orders.Orders.Waypoints[1].TargetNum
	}
	return 0
}
func (t *turnGenerator) followBombingRally(p *Player, f *Fleet) bool {
	var rally bombingRally
	if json.Unmarshal([]byte(f.GetTag(bombingRallyTag)), &rally) != nil {
		return false
	}
	leader := t.game.getFleet(p.Num, rally.Leader)
	if rally.Until < t.game.Year || leader == nil || leader.Delete || leader.InAutomaticSafety() {
		delete(f.Tags, bombingRallyTag)
		return false
	}
	if f.Num == leader.Num {
		waiting := false
		for _, other := range t.game.getFleets(p.Num) {
			var job bombingRally
			if !other.Delete && json.Unmarshal([]byte(other.GetTag(bombingRallyTag)), &job) == nil && job.Leader == f.Num && other.Position != f.Position {
				waiting = true
			}
		}
		if waiting {
			holdAutomaticFleet(p, f, "Wolf pack: waiting at the protected rally point")
			return true
		}
		delete(f.Tags, bombingRallyTag)
		return false
	}
	if f.Position == leader.Position {
		delete(f.Tags, bombingRallyTag)
		return false
	}
	group := convoyMembers(p, f, t.game.getFleets(p.Num))
	route := planAutomaticRoute(&t.game.Rules, p, group, leader.Position, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: len(group) > 1, MaxTurns: 2})
	if route == nil {
		delete(f.Tags, bombingRallyTag)
		return false
	}
	for _, member := range group {
		applyExpeditionRoute(p, member, route)
	}
	return true
}
func (t *turnGenerator) planBombingWolfPacks() {
	for _, p := range t.game.Players {
		fleets := slices.Clone(t.game.getFleets(p.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		for _, leader := range fleets {
			goal := bomberGoal(leader)
			if goal == 0 || leader.Delete || leader.InAutomaticSafety() || leader.InDesignMaintenance() || fuelNetworkManaged(leader) || leader.GetTag(tankerAwaitTag) != "" || p.Automation.FleetRoles[leader.Num] != "bomber" {
				continue
			}
			members := []*Fleet{leader}
			for _, f := range fleets {
				if f == leader || f.Delete || f.InAutomaticSafety() || f.InDesignMaintenance() || fuelNetworkManaged(f) || f.GetTag(tankerAwaitTag) != "" {
					continue
				}
				ownBomber := p.Automation.FleetRoles[f.Num] == "bomber" && bomberGoal(f) == goal
				ownEscort := p.Automation.FleetRoles[f.Num] == "escort" && f.GetTag(escortTag) == strconv.Itoa(leader.Num)
				if !ownBomber && !ownEscort {
					continue
				}
				if f.Position == leader.Position {
					members = append(members, f)
					continue
				}
				if !ownBomber || f.Position.DistanceTo(leader.Position) > 100 {
					continue
				}
				protected := t.bomberUnescortedSafe(p, leader, leader.Position)
				for _, planet := range t.game.getPlanets(p.Num) {
					protected = protected || planet.Position == leader.Position
				}
				group := convoyMembers(p, f, t.game.getFleets(p.Num))
				if !protected || len(group) == 1 && !t.bomberUnescortedSafe(p, f, leader.Position) {
					continue
				}
				route := planAutomaticRoute(&t.game.Rules, p, group, leader.Position, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: len(group) > 1, MaxTurns: 2})
				if route == nil {
					continue
				}
				rally := bombingRally{Leader: leader.Num, Target: goal, Until: t.game.Year + 2}
				raw, _ := json.Marshal(rally)
				f.SetTag(bombingRallyTag, string(raw))
				leader.SetTag(bombingRallyTag, string(raw))
				for _, member := range group {
					applyExpeditionRoute(p, member, route)
				}
				holdAutomaticFleet(p, leader, "Wolf pack: protected rally before simultaneous attack")
			}
			// Include escorts assigned to any co-located bomber in this pack.
			for _, bomber := range slices.Clone(members) {
				for _, e := range fleets {
					if !e.Delete && e.Position == leader.Position && p.Automation.FleetRoles[e.Num] == "escort" && e.GetTag(escortTag) == strconv.Itoa(bomber.Num) && !slices.Contains(members, e) {
						members = append(members, e)
					}
				}
			}
			if len(members) < 2 {
				continue
			}
			pack := bombingPack{Target: goal}
			for _, f := range members {
				pack.Parts = append(pack.Parts, attackPackParts(p, f)...)
			}
			merged, err := NewOrderer().Merge(&t.game.Rules, p, members)
			if err != nil {
				continue
			}
			for _, f := range members {
				if f != merged {
					t.game.deleteFleet(f)
				}
			}
			raw, _ := json.Marshal(pack)
			merged.SetTag(bombingPackTag, string(raw))
			delete(merged.Tags, escortTag)
			delete(merged.Tags, bomberBorrowTag)
			p.Automation.FleetRoles[merged.Num] = "bomber"
			target := p.GetPlanetIntel(goal)
			if target != nil && merged.GetTag(bombingRallyTag) == "" {
				route := t.planAutomaticMission(p, []*Fleet{merged}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: merged.Spec.HasWeapons})
				if route != nil {
					applyExpeditionRoute(p, merged, route)
				} else {
					holdAutomaticFleet(p, merged, "Wolf pack waiting for a funded route")
				}
			}
		}
	}
}
