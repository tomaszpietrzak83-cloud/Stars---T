package cs

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const battleshipRallyTag = "battleship-rally"
const battleshipNextAssaultTag = "battleship-next-assault"

func battleshipRally(f *Fleet) (dock, target int) {
	a, b, _ := strings.Cut(f.GetTag(battleshipRallyTag), ":")
	dock, _ = strconv.Atoi(a)
	target, _ = strconv.Atoi(b)
	return
}

func hasAssaultBattleship(p *Player, f *Fleet) bool {
	for _, token := range f.Tokens {
		if d := p.GetDesign(token.DesignNum); d != nil && d.Hull == Battleship.Name {
			return true
		}
	}
	return false
}

func pureAssaultBattleships(p *Player, f *Fleet) bool {
	for _, token := range f.Tokens {
		if d := p.GetDesign(token.DesignNum); d == nil || d.Hull != Battleship.Name {
			return false
		}
	}
	return len(f.Tokens) > 0
}

func (t *turnGenerator) planBattleshipAssaults(p *Player, fleets []*Fleet) {
	reserve := []*Fleet{}
	rallyTargets := map[int]int{}
	rallyMembers := map[int][]*Fleet{}
	for _, f := range fleets {
		if f.Delete {
			continue
		}
		role := p.Automation.FleetRoles[f.Num]
		if f.GetTag(battleshipNextAssaultTag) != "" && atOwnFuelDock(p, f) && f.Fuel >= f.Spec.FuelCapacity {
			delete(f.Tags, fuelRequestTag)
		}
		dock, target := battleshipRally(f)
		if target != 0 {
			delete(f.Tags, battleshipRallyTag)
			if (role == "patrol" || role == "escort") && pureAssaultBattleships(p, f) && patrolAvailable(p, f) && !defencePatrolCommitted(p, f) {
				q := p.GetPlanetIntel(target)
				d := t.game.getPlanet(dock)
				if q != nil && q.Spec.HasStarbase && q.Owned() && q.PlayerNum != p.Num && !p.IsFriend(q.PlayerNum) && d != nil && d.OwnedBy(p.Num) && d.Spec.HasStarbase {
					rallyTargets[target] = dock
					rallyMembers[target] = append(rallyMembers[target], f)
					continue
				}
				holdAutomaticFleet(p, f, "Orbital rally ended; available for patrol")
			}
		}
		if (role != "patrol" && role != "escort") || !pureAssaultBattleships(p, f) ||
			!patrolAvailable(p, f) || defencePatrolCommitted(p, f) || patrolNeededForColony(p, f) {
			continue
		}
		reserve = append(reserve, f)
	}

	// Landing and patrol forecasts may already have spent the ordinary budget.
	evaluator := t.recaptureEvaluator(p)
	ordinaryWork := evaluator.Work
	ordinaryLimit := evaluator.Limit
	evaluator.Work, evaluator.Limit = 0, 2048
	defer func() { evaluator.Work, evaluator.Limit = ordinaryWork, ordinaryLimit }()
	for _, target := range p.PlanetIntels {
		if target == nil || len(rallyMembers[target.Num]) == 0 {
			continue
		}
		group := rallyMembers[target.Num]
		dock := t.game.getPlanet(rallyTargets[target.Num])
		base := t.reportedBattleshipTarget(p, target, fleets, rallyTargets)
		if base != nil && t.prepareBattleshipAssault(p, group, dock, target, base) {
			for _, f := range group {
				if !f.Delete && f.GetTag(recaptureAssaultTag) == "" && f.GetTag(battleshipRallyTag) == "" {
					holdAutomaticFleet(p, f, "Orbital rally surplus; available for patrol")
					reserve = append(reserve, f)
				}
			}
			continue
		}
		for _, f := range group {
			f.SetTag(battleshipRallyTag, fmt.Sprintf("%d:%d", dock.Num, target.Num))
			if !t.routeLanding(p, f, dock.Position, dock.ToTarget(), false, 4) {
				continue
			}
			f.SetTag(fuelStatusTag, "Orbital rally retained; awaiting a fresh zero-loss forecast")
		}
	}
	targets := []*Planet{}
	for _, q := range p.PlanetIntels {
		if len(reserve) > 0 && q != nil && len(rallyMembers[q.Num]) == 0 && t.reportedBattleshipTarget(p, q, fleets, rallyTargets) != nil {
			targets = append(targets, q)
		}
	}
	territorial := map[int]bool{}
	distance := map[int]float64{}
	for _, q := range targets {
		territorial[q.Num] = patrolOwnTerritory(p, q.Position, q.Num)
		distance[q.Num] = reserve[0].Position.DistanceTo(q.Position)
		for _, f := range reserve[1:] {
			distance[q.Num] = min(distance[q.Num], f.Position.DistanceTo(q.Position))
		}
	}
	slices.SortStableFunc(targets, func(a, b *Planet) int {
		if territorial[a.Num] != territorial[b.Num] {
			if territorial[a.Num] {
				return -1
			}
			return 1
		}
		if (rallyTargets[a.Num] != 0) != (rallyTargets[b.Num] != 0) {
			if rallyTargets[a.Num] != 0 {
				return -1
			}
			return 1
		}
		if distance[a.Num] < distance[b.Num] {
			return -1
		}
		if distance[a.Num] > distance[b.Num] {
			return 1
		}
		return a.Num - b.Num
	})
	for _, target := range targets {
		if len(reserve) == 0 || evaluator.Work >= evaluator.Limit {
			break
		}
		base := t.reportedBattleshipTarget(p, target, fleets, rallyTargets)
		if t.launchReadyBattleshipFollowup(p, reserve, target, base) {
			reserve = slices.DeleteFunc(reserve, func(f *Fleet) bool { return f.GetTag(recaptureAssaultTag) != "" })
			continue
		}
		docks := slices.Clone(expeditionStations(p, reserve[0]))
		slices.SortStableFunc(docks, func(a, b *Planet) int {
			if dock := rallyTargets[target.Num]; (a.Num == dock) != (b.Num == dock) {
				if a.Num == dock {
					return -1
				}
				return 1
			}
			da, db := a.Position.DistanceTo(target.Position), b.Position.DistanceTo(target.Position)
			if da < db {
				return -1
			}
			if da > db {
				return 1
			}
			return a.Num - b.Num
		})
		for i, dock := range docks {
			if i >= 3 {
				break
			}
			if t.prepareBattleshipAssault(p, reserve, dock, target, base) {
				reserve = slices.DeleteFunc(reserve, func(f *Fleet) bool {
					return f.Delete || f.GetTag(battleshipRallyTag) != "" || f.GetTag(recaptureAssaultTag) != ""
				})
				break
			}
		}
	}
	// Retain the proven formation at a fuel dock while scouts look for the
	// next opening. An exhausted or stale target list must not disperse it.
	for _, f := range fleets {
		if f.Delete || f.GetTag(battleshipNextAssaultTag) == "" || f.GetTag(recaptureAssaultTag) != "" || f.GetTag(battleshipRallyTag) != "" || f.InAutomaticSafety() || f.InDesignMaintenance() || fuelNetworkManaged(f) {
			continue
		}
		if !atOwnFuelDock(p, f) && !t.routeEmergencyReturn(p, f) {
			requestAutomaticRescue(p, f, true)
			f.SetTag(fuelStatusTag, "Battleship assault group awaits fuel for its next target")
		}
	}
}

func (t *turnGenerator) launchReadyBattleshipFollowup(p *Player, reserve []*Fleet, target *Planet, base *Fleet) bool {
	for _, f := range reserve {
		if f.GetTag(battleshipNextAssaultTag) == "" {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MaxTurns: 4})
		if route == nil || route.WaitForFuel {
			continue
		}
		result := t.assessRecaptureArrival(p, []*Fleet{f}, base, float64(route.Turns))
		if !result.Ready {
			continue
		}
		t.detachPatrolMember(p, f)
		p.Automation.FleetRoles[f.Num] = "patrol"
		f.SetTag(recaptureAssaultTag, strconv.Itoa(target.Num))
		applyPatrolBattlePlan(p, f, result.Tactic)
		applyExpeditionRoute(p, f, route)
		return true
	}
	return false
}

func (t *turnGenerator) reportedBattleshipTarget(p *Player, target *Planet, fleets []*Fleet, rallyTargets map[int]int) *Fleet {
	if target == nil {
		return nil
	}
	for _, f := range fleets {
		if !f.Delete && f.GetTag(recaptureAssaultTag) == strconv.Itoa(target.Num) {
			return nil
		}
	}
	if base := t.reportedLandingBase(p, target); base != nil {
		return base
	}
	if rallyTargets[target.Num] != 0 && !target.Unexplored() {
		report := *target
		report.ReportAge = 0
		return reportedRecaptureBase(p, &report, true)
	}
	return reportedRecaptureBase(p, target, true)
}

func (t *turnGenerator) prepareBattleshipAssault(p *Player, reserve []*Fleet, dock, target *Planet, base *Fleet) bool {
	pool := []landingBaseGuard{}
	for _, f := range reserve {
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, dock.Position, dock.ToTarget(), automaticRoutePolicy{Fast: true, MaxTurns: 4})
		if route == nil || route.WaitForFuel {
			continue
		}
		probe := *f
		probe.Position, probe.Fuel = dock.Position, route.ArrivalFuel[0]
		pool = append(pool, landingBaseGuard{f, &probe, route})
	}
	slices.SortFunc(pool, compareLandingGuards)
	probes, group := []*Fleet{}, []*Fleet{}
	for i, c := range pool {
		probes, group = append(probes, c.probe), append(group, c.fleet)
		merged := patrolMergedFormation(&t.game.Rules, p, probes)
		if merged == nil {
			return false
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{merged}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MaxTurns: 4})
		if route == nil || route.WaitForFuel {
			continue
		}
		result := t.assessRecaptureArrival(p, probes, base, float64(c.route.Turns+route.Turns))
		if result.Reason == patrolForecastBudgetExhausted {
			return false
		}
		if !result.Ready {
			continue
		}
		for _, f := range group {
			t.detachPatrolMember(p, f)
			restorePatrolBattlePlan(p, f)
			delete(f.Tags, patrolFrontTag)
			delete(f.Tags, frontierPatrolTag)
		}
		if c.route.Turns > 0 {
			for _, member := range pool[:i+1] {
				member.fleet.SetTag(battleshipRallyTag, fmt.Sprintf("%d:%d", dock.Num, target.Num))
				applyExpeditionRoute(p, member.fleet, member.route)
				member.fleet.SetTag(fuelStatusTag, fmt.Sprintf("Gathering %d Battleships for a zero-loss forecast at %s", merged.Spec.TotalShips, target.Name))
			}
			return true
		}
		if reportedRecaptureBase(p, target, true) == nil && t.reportedLandingBase(p, target) == nil {
			for _, f := range group {
				f.SetTag(battleshipRallyTag, fmt.Sprintf("%d:%d", dock.Num, target.Num))
				holdAutomaticFleet(p, f, "Orbital assault awaits fresh reconnaissance")
			}
			return true
		}
		leader := group[0]
		if !t.mergeLandingGroup(p, leader, group) {
			return false
		}
		p.Automation.FleetRoles[leader.Num] = "patrol"
		leader.SetTag(recaptureAssaultTag, strconv.Itoa(target.Num))
		applyPatrolBattlePlan(p, leader, result.Tactic)
		applyExpeditionRoute(p, leader, route)
		return true
	}
	return false
}
