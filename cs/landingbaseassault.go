package cs

import (
	"fmt"
	"slices"
	"strconv"
)

const landingBaseRallyTurns = 2
const landingBaseReconTag = "automatic-landing-base-recon"

func (t *turnGenerator) landingBaseReconDeparture(p *Player, f *Fleet) bool {
	tag := f.GetTag(landingBaseReconTag)
	if tag == "" || !landingScout(p, f) || defencePatrolCommitted(p, f) || len(f.Waypoints) < 2 {
		return false
	}
	num, _ := strconv.Atoi(tag)
	leader := t.game.getFleet(p.Num, num)
	op := readLandingOperation(leader)
	if op == nil || leader.Delete || op.Phase != "base" || op.Scout != f.Num {
		return false
	}
	target := p.GetPlanetIntel(op.Target)
	return target != nil && target.PlayerNum == op.Owner && !p.IsFriend(op.Owner) &&
		target.Spec.HasStarbase && f.Waypoints[len(f.Waypoints)-1].Position == target.Position
}

func (t *turnGenerator) releaseLandingBaseRecon(p *Player, op *landingOperation) {
	if f := t.game.getFleet(p.Num, op.Scout); f != nil && f.GetTag(landingBaseReconTag) != "" {
		delete(f.Tags, landingBaseReconTag)
		delete(f.Tags, landingMemberTag)
		op.Scout = 0
		if !f.Delete && !f.InAutomaticSafety() && !f.InDesignMaintenance() && !defencePatrolCommitted(p, f) {
			holdAutomaticFleet(p, f, "Orbital reconnaissance finished")
		}
	}
}

func (t *turnGenerator) routeLandingBaseRecon(p *Player, leader *Fleet, op *landingOperation, target *Planet) {
	scout := t.game.getFleet(p.Num, op.Scout)
	if scout != nil && !scout.Delete && !scout.InAutomaticSafety() && !scout.InDesignMaintenance() && landingScout(p, scout) {
		t.routeLanding(p, scout, target.Position, target.ToTarget(), true, 0)
		return
	}
	// Keep the fleet's patrol role. Only a single available Destroyer can
	// stand in for a scout on this contact.
	candidates := []*Fleet{}
	for _, f := range t.game.getFleets(p.Num) {
		if p.Automation.FleetRoles[f.Num] != "patrol" || !landingAvailable(p, f) || !patrolAvailable(p, f) ||
			readPatrolJob(f).Leader != 0 || defencePatrolCommitted(p, f) || patrolNeededForColony(p, f) ||
			f.Spec.TotalShips != 1 || len(f.Tokens) != 1 {
			continue
		}
		if design := p.GetDesign(f.Tokens[0].DesignNum); design != nil && design.Hull == string(StartingFleetHullDestroyer) {
			candidates = append(candidates, f)
		}
	}
	slices.SortFunc(candidates, func(a, b *Fleet) int {
		if power := patrolPower(a) - patrolPower(b); power != 0 {
			return power
		}
		return a.Num - b.Num
	})
	for _, f := range candidates {
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MaxTurns: 4})
		if route == nil || route.WaitForFuel {
			continue
		}
		op.Scout = f.Num
		f.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
		f.SetTag(landingBaseReconTag, strconv.Itoa(leader.Num))
		applyExpeditionRoute(p, f, route)
		saveLandingOperation(leader, op)
		return
	}
}

type landingBaseGuard struct {
	fleet *Fleet
	probe *Fleet
	route *expeditionRoute
}

func compareLandingGuards(a, b landingBaseGuard) int {
	if a.route.Turns != b.route.Turns {
		return a.route.Turns - b.route.Turns
	}
	if power := patrolPower(b.fleet) - patrolPower(a.fleet); power != 0 {
		return power
	}
	return a.fleet.Num - b.fleet.Num
}

func (t *turnGenerator) releaseLandingBaseSurplus(p *Player, leader *Fleet, keep map[int]bool) {
	for _, f := range t.landingMembers(p, leader) {
		role := p.Automation.FleetRoles[f.Num]
		if keep[f.Num] || landingScout(p, f) || role != "patrol" && role != "escort" {
			continue
		}
		delete(f.Tags, landingMemberTag)
		delete(f.Tags, landingRaidTag)
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
		if (role == "patrol" || role == "escort") && !landingScout(p, f) && !f.InAutomaticSafety() && !f.InDesignMaintenance() && !fuelNetworkManaged(f) && !defencePatrolCommitted(p, f) {
			core[f.Num] = true
			ships += f.Spec.TotalShips
			if ships >= p.Automation.transportPolicy().LandingEscortShips {
				break
			}
		}
	}
	return core
}

func (t *turnGenerator) prepareLandingBaseAssault(p *Player, leader *Fleet, op *landingOperation, target *Planet, _ []*Fleet) {
	defer scopedRouteThreatIndex(p)()
	saveLandingOperation(leader, op)
	status := func(message string) {
		leader.SetTag(fuelStatusTag, "Orbital assault at "+target.Name+": "+message)
	}
	// A blocked base is not a standing reservation, including for a large
	// legacy stack that used to be retained as the transport's core escort.
	core := map[int]bool{}
	release := func() {
		t.releaseLandingBaseSurplus(p, leader, core)
		op.BaseRallyBy = 0
		saveLandingOperation(leader, op)
	}
	if t.game.Year < op.BaseRallyRetryYear {
		release()
		status("rendezvous expired; patrols remain available this year")
		return
	}
	for _, f := range t.game.getFleets(p.Num) {
		if !f.Delete && f.GetTag(recaptureAssaultTag) == strconv.Itoa(target.Num) {
			release()
			if reason := f.GetTag(fuelStatusTag); reason != "" {
				status(reason)
			} else {
				status(fmt.Sprintf("%d ships assigned to the attack", f.Spec.TotalShips))
			}
			return // an assessed attack is already in progress
		}
	}
	// Reconnaissance can proceed independently; stale reports cannot reserve
	// combat ships for an indefinitely postponed assault.
	if target.ReportAge > 2 && (op.ScanYear == 0 || op.ScanYear < t.game.Year-1) {
		release()
		status("awaiting fresh reconnaissance; patrols remain available")
		t.routeLandingBaseRecon(p, leader, op, target)
		return
	}
	report := *target
	report.ReportAge = 0
	base := reportedRecaptureBase(p, &report, true)
	if base == nil {
		release()
		// A base warning without its equipment is not a completed scout visit.
		// Waiting for an assessed patrol here would wait forever for missing intel.
		design := p.GetShipDesignIntel(target.PlayerNum, target.Spec.StarbaseDesignNum)
		if target.Spec.HasStarbase && (design == nil || len(design.Slots) == 0) {
			status("awaiting reconnaissance of base equipment")
			t.routeLandingBaseRecon(p, leader, op, target)
		}
		return
	}
	evaluator := t.recaptureEvaluator(p)
	if evaluator.Work >= evaluator.Limit {
		release()
		status("safe attack not confirmed; patrols remain available")
		return
	}
	remaining := landingBaseRallyTurns
	if op.BaseRallyBy != 0 {
		remaining = min(remaining, op.BaseRallyBy-t.game.Year)
		if remaining < 0 {
			op.BaseRallyRetryYear = t.game.Year + 1
			release()
			status("rendezvous expired; patrols released")
			return
		}
	}
	pool := []landingBaseGuard{}
	territorial := patrolOwnTerritory(p, target.Position, target.Num)
	for _, f := range t.game.getFleets(p.Num) {
		role := p.Automation.FleetRoles[f.Num]
		member := f.GetTag(landingMemberTag) == strconv.Itoa(leader.Num)
		if role != "patrol" && role != "escort" || f.Delete || !f.Spec.HasWeapons || hasAssaultBattleship(p, f) || f.InAutomaticSafety() || f.InDesignMaintenance() || fuelNetworkManaged(f) || defencePatrolCommitted(p, f) || patrolNeededForColony(p, f) {
			continue
		}
		if !member && (!landingAvailable(p, f) || !availableRecapturePatrol(p, f, territorial)) {
			continue
		}
		if f.GetTag(landingBaseReconTag) != "" || remaining == 0 && f.Position != op.Rally || f.Position.DistanceTo(op.Rally) > float64(remaining)*f.StrategicSpeed(automaticMaxWarp(p, f))+3 {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, op.Rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: max(1, remaining), Fast: true})
		if route == nil || route.WaitForFuel || route.Turns > remaining && f.Position != op.Rally || len(route.ArrivalFuel) == 0 {
			continue
		}
		probe := *f
		probe.Position, probe.Fuel = op.Rally, route.ArrivalFuel[0]
		pool = append(pool, landingBaseGuard{f, &probe, route})
	}
	slices.SortFunc(pool, compareLandingGuards)
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
			return patrolAssessment{Reason: "no compatible assault formation"}, nil
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{merged}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MaxTurns: 1})
		if route == nil || route.WaitForFuel {
			return patrolAssessment{Reason: "no assault route with sufficient fuel"}, nil
		}
		return t.assessRecaptureArrival(p, group, base, float64(pool[n-1].route.Turns+route.Turns)), route
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
		if n == len(pool) || result.Reason == patrolForecastBudgetExhausted || evaluator.Work >= evaluator.Limit {
			break
		}
		n = min(len(pool), n*2)
	}
	if chosen == 0 {
		if remaining == 0 {
			op.BaseRallyRetryYear = t.game.Year + 1
		}
		release()
		if result.Reason == patrolForecastBudgetExhausted {
			status("safe attack not confirmed; patrols remain available")
			return
		}
		if result.Reason == "" {
			result.Reason = "no available reinforcements with a route to the rally"
		}
		status(result.Reason)
		if !t.retargetBlockedLanding(p, leader, op) {
			t.landingEscortRaid(p, leader, op)
		}
		return
	}
	for lo, hi := failed+1, chosen; lo < hi && evaluator.Work < evaluator.Limit; {
		mid := (lo + hi) / 2
		check, route := assess(mid)
		if check.Ready && route != nil {
			chosen, result, hi = mid, check, mid
		} else {
			lo = mid + 1
		}
	}
	// Ships arriving by the same deadline may buy a safer fight without
	// making the transports wait another year.
	sameArrival := chosen
	for sameArrival < len(pool) && pool[sameArrival].route.Turns == pool[chosen-1].route.Turns {
		sameArrival++
	}
	if sameArrival > chosen && (result.LostShips > 0 || result.PeakDamage > 0) && evaluator.Work < evaluator.Limit {
		check, route := assess(sameArrival)
		if route != nil && improvesRecaptureForecast(check, result) {
			chosen, result = sameArrival, check
		}
	}
	keep := map[int]bool{}
	if op.BaseRallyBy == 0 {
		op.BaseRallyBy = t.game.Year + landingBaseRallyTurns
	}
	saveLandingOperation(leader, op)
	group := []*Fleet{}
	ships := 0
	here := true
	for _, candidate := range pool[:chosen] {
		f := candidate.fleet
		keep[f.Num] = true
		group = append(group, f)
		ships += f.Spec.TotalShips
		here = here && f.Position == op.Rally
		t.detachPatrolMember(p, f)
		f.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
		delete(f.Tags, landingRaidTag)
		restorePatrolBattlePlan(p, f)
		delete(f.Tags, "automatic-landing-screen")
		applyExpeditionRoute(p, f, candidate.route)
	}
	t.releaseLandingBaseSurplus(p, leader, keep)
	if !here {
		status(fmt.Sprintf("assembling %d ships, up to %d turns to rally", ships, pool[chosen-1].route.Turns))
		return
	}
	attack := group[0]
	if !t.mergeLandingGroup(p, attack, group) {
		status("waiting for the assault fleet to merge")
		return
	}
	delete(attack.Tags, landingMemberTag)
	p.Automation.FleetRoles[attack.Num] = "patrol"
	attack.SetTag(recaptureAssaultTag, strconv.Itoa(target.Num))
	applyPatrolBattlePlan(p, attack, result.Tactic)
	t.routeLanding(p, attack, target.Position, target.ToTarget(), true, 1)
	status(fmt.Sprintf("%d ships assigned to the attack", ships))
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
	for _, f := range fleets {
		if !f.Delete && f.GetTag(recaptureAssaultTag) == strconv.Itoa(target.Num) && pureAssaultBattleships(p, f) {
			return reportedRecaptureBase(p, &report, true)
		}
	}
	return reportedRecaptureBase(p, &report, landingBaseRequested(p, target.Num, fleets))
}
