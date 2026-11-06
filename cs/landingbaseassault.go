package cs

import (
	"slices"
	"strconv"
)

type landingBaseGuard struct {
	fleet *Fleet
	probe *Fleet
	route *expeditionRoute
}

func (t *turnGenerator) releaseLandingBaseSurplus(p *Player, leader *Fleet, keep map[int]bool) {
	for _, f := range t.landingMembers(p, leader) {
		role := p.Automation.FleetRoles[f.Num]
		if keep[f.Num] || role != "patrol" && role != "escort" {
			continue
		}
		delete(f.Tags, landingMemberTag)
		delete(f.Tags, "automatic-landing-screen")
		if defencePatrolCommitted(p, f) {
			continue
		}
		delete(f.Tags, patrolTag)
		if !f.InAutomaticSafety() && !f.InDesignMaintenance() && !fuelNetworkManaged(f) && !defencePatrolCommitted(p, f) {
			restorePatrolBattlePlan(p, f)
			holdAutomaticFleet(p, f, "Orbital assault surplus released to automatic duties")
		}
	}
}

func (t *turnGenerator) landingBaseProtectors(p *Player, leader *Fleet, op *landingOperation) map[int]bool {
	existing := t.landingMembers(p, leader)
	slices.SortStableFunc(existing, func(a, b *Fleet) int {
		da, db := a.Position.DistanceTo(op.Rally), b.Position.DistanceTo(op.Rally)
		if da < db {
			return -1
		}
		if da > db {
			return 1
		}
		return a.Num - b.Num
	})
	core := map[int]bool{}
	ships := 0
	for _, f := range existing {
		role := p.Automation.FleetRoles[f.Num]
		if (role == "patrol" || role == "escort") && ships+f.Spec.TotalShips <= 2 && !defencePatrolCommitted(p, f) {
			core[f.Num] = true
			ships += f.Spec.TotalShips
		}
	}
	return core
}

func (t *turnGenerator) prepareLandingBaseAssault(p *Player, leader *Fleet, op *landingOperation, target *Planet, _ []*Fleet) {
	saveLandingOperation(leader, op)
	core := t.landingBaseProtectors(p, leader, op)
	for _, f := range t.game.getFleets(p.Num) {
		if !f.Delete && f.GetTag(recaptureAssaultTag) == strconv.Itoa(target.Num) {
			t.releaseLandingBaseSurplus(p, leader, core)
			return // an assessed attack is already in progress
		}
	}
	// Old observed equipment can justify preparing a group, never its launch.
	report := *target
	report.ReportAge = 0
	base := reportedRecaptureBase(p, &report, true)
	if base == nil {
		t.releaseLandingBaseSurplus(p, leader, core)
		// A base warning without its equipment is not a completed scout visit.
		// Waiting for an assessed patrol here would wait forever for missing intel.
		design := p.GetShipDesignIntel(target.PlayerNum, target.Spec.StarbaseDesignNum)
		if target.Spec.HasStarbase && (design == nil || len(design.Slots) == 0) {
			if scout := t.game.getFleet(p.Num, op.Scout); scout != nil && !scout.Delete && landingScout(p, scout) && !scout.InAutomaticSafety() && !scout.InDesignMaintenance() {
				t.routeLanding(p, scout, target.Position, target.ToTarget(), true, 0)
			}
		}
		return
	}
	pool := []landingBaseGuard{}
	for _, f := range t.game.getFleets(p.Num) {
		role := p.Automation.FleetRoles[f.Num]
		member := f.GetTag(landingMemberTag) == strconv.Itoa(leader.Num)
		if role != "patrol" && role != "escort" || f.Delete || !f.Spec.HasWeapons || f.InAutomaticSafety() || f.InDesignMaintenance() || fuelNetworkManaged(f) || defencePatrolCommitted(p, f) || !core[f.Num] && patrolNeededForColony(p, f) {
			continue
		}
		if !member && (!landingAvailable(p, f) || !patrolAvailable(p, f) || readPatrolJob(f).Leader != 0) {
			continue
		}
		if f.Position.DistanceTo(op.Rally) > 2*f.StrategicSpeed(automaticMaxWarp(p, f))+3 {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, op.Rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 2, Fast: true})
		if route == nil || route.WaitForFuel || len(route.ArrivalFuel) == 0 {
			continue
		}
		probe := *f
		probe.Position, probe.Fuel = op.Rally, route.ArrivalFuel[0]
		pool = append(pool, landingBaseGuard{f, &probe, route})
	}
	slices.SortStableFunc(pool, func(a, b landingBaseGuard) int {
		if a.route.Turns != b.route.Turns {
			return a.route.Turns - b.route.Turns
		}
		return a.fleet.Num - b.fleet.Num
	})
	// Probe growing prefixes, then narrow a sufficient prefix. Every accepted
	// group has its own native forecast and funded merged departure; an unknown
	// or impossible battle never reserves all failed candidates.
	assess := func(n int) (patrolAssessment, *expeditionRoute) {
		group := make([]*Fleet, n)
		for i := range n {
			group[i] = pool[i].probe
		}
		merged := patrolMergedFormation(&t.game.Rules, p, group)
		if merged == nil {
			return patrolAssessment{}, nil
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{merged}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MaxTurns: 1})
		if route == nil || route.WaitForFuel {
			return patrolAssessment{}, nil
		}
		return t.assessRecapture(p, group, base), route
	}
	chosen, failed := 0, 0
	result := patrolAssessment{}
	for n := 1; n <= len(pool); {
		var route *expeditionRoute
		result, route = assess(n)
		if result.Ready && route != nil {
			chosen = n
			break
		}
		failed = n
		if n == len(pool) || t.recaptureEvaluator(p).Work >= t.recaptureEvaluator(p).Limit {
			break
		}
		n = min(len(pool), n*2)
	}
	if chosen == 0 {
		t.releaseLandingBaseSurplus(p, leader, core)
		t.retargetBlockedLanding(p, leader, op)
		return
	}
	for lo, hi := failed+1, chosen; lo < hi && t.recaptureEvaluator(p).Work < t.recaptureEvaluator(p).Limit; {
		mid := (lo + hi) / 2
		check, route := assess(mid)
		if check.Ready && route != nil {
			chosen, result, hi = mid, check, mid
		} else {
			lo = mid + 1
		}
	}
	keep := map[int]bool{}
	group := []*Fleet{}
	here := true
	for _, candidate := range pool[:chosen] {
		f := candidate.fleet
		keep[f.Num] = true
		group = append(group, f)
		here = here && f.Position == op.Rally
		f.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
		delete(f.Tags, patrolTag)
		delete(f.Tags, "automatic-landing-screen")
		applyExpeditionRoute(p, f, candidate.route)
	}
	t.releaseLandingBaseSurplus(p, leader, keep)
	if target.ReportAge > 2 && (op.ScanYear == 0 || op.ScanYear < t.game.Year-1) {
		// Refresh only once a sufficient funded force exists. Do not feed scouts
		// into an unwinnable base, or pretend old intelligence is current.
		if scout := t.game.getFleet(p.Num, op.Scout); scout != nil && !scout.Delete && landingScout(p, scout) && scout.Position == op.Rally {
			t.routeLanding(p, scout, target.Position, target.ToTarget(), true, 1)
		}
		return
	}
	if !here {
		return
	}
	attack := group[0]
	if !t.mergeLandingGroup(p, attack, group) {
		return
	}
	delete(attack.Tags, landingMemberTag)
	p.Automation.FleetRoles[attack.Num] = "patrol"
	attack.SetTag(recaptureAssaultTag, strconv.Itoa(target.Num))
	applyPatrolBattlePlan(p, attack, result.Tactic)
	t.routeLanding(p, attack, target.Position, target.ToTarget(), true, 1)
}

// Orbital contact freshness is independent of the surface population report.
func (t *turnGenerator) reportedLandingBase(p *Player, target *Planet) *Fleet {
	if target == nil {
		return nil
	}
	report := *target
	fleets := t.game.getFleets(p.Num)
	for _, f := range fleets {
		op := readLandingOperation(f)
		if !f.Delete && op != nil && op.Target == target.Num && op.ScanYear > 0 && op.ScanYear >= t.game.Year-1 {
			report.ReportAge = 0
			break
		}
	}
	return reportedRecaptureBase(p, &report, landingBaseRequested(p, target.Num, fleets))
}
