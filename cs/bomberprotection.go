package cs

import (
	"math"
	"slices"
	"strconv"
)

// Derive possible speed from public engine components only. Observed warp 2
// does not establish fuel exhaustion. Unknown engines retain a conservative
// ordinary warp-10 envelope; private enemy token/design pointers are never used.
func reportedMaximumSpeed(p *Player, enemy *Fleet) float64 {
	speed := math.Inf(1)
	if len(enemy.Tokens) == 0 {
		return max(100, enemy.observedStrategicSpeed())
	}
	for _, token := range enemy.Tokens {
		d := p.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum)
		found := false
		engine := Engine{MaxSafeSpeed: 10}
		if d != nil && len(d.Slots) > 0 {
			for _, slot := range d.Slots {
				c := StaticTechStore.GetHullComponent(slot.HullComponent)
				if c != nil && c.HullSlotType&HullSlotTypeEngine != 0 {
					if !found || c.MaxSafeSpeed < engine.MaxSafeSpeed {
						engine = c.Engine
					}
					found = true
				}
			}
		}
		speed = min(speed, engine.StrategicSpeed(min(10, max(1, engine.MaxSafeSpeed))))
	}
	return max(speed, enemy.observedStrategicSpeed())
}

func bomberWarningReports(p *Player, f *Fleet) []*Fleet {
	var reports []*Fleet
	for _, e := range p.FleetIntels {
		if e == nil || e.Delete || e.Starbase || e.ReportAge != 0 || e.PlayerNum == p.Num || p.IsFriend(e.PlayerNum) || !patrolReportedCombatant(p, e) {
			continue
		}
		distance := f.Position.DistanceTo(e.Position)
		if p.planetOnlyAI[e.PlayerNum] && f.Position.DistanceTo(patrolInterceptPosition(e)) > 3 {
			continue
		}
		if stationaryBombardmentGuard(p, e) && distance > 3 {
			continue
		}
		speed := reportedMaximumSpeed(p, e)
		toward := f.Position.Subtract(e.Position).Normalized()
		closing := e.Heading.X*toward.X + e.Heading.Y*toward.Y
		if distance <= speed+3 || closing > .5 && distance <= 2*speed+3 {
			reports = append(reports, e)
		}
	}
	return reports
}

func stationaryBombardmentGuard(p *Player, enemy *Fleet) bool {
	return enemy != nil && enemy.WarpSpeed == 0 && observedBombardmentGuard(p, enemy)
}

// Reinforce local bombardment before considering a pre-emptive withdrawal.
// En-route support counts only if its actual funded route arrives this turn;
// the forecast models separate native fleets, not a fictitious merged stack.
func (t *turnGenerator) prepareBombardmentProtection(p *Player, bomber *Fleet) (ready, remoteWarning bool) {
	reports := bomberWarningReports(p, bomber)
	if len(reports) == 0 {
		return false, false
	}
	remote := true
	for _, planet := range p.PlanetIntels {
		if planet != nil && planet.OwnedBy(p.Num) && planet.Position.DistanceTo(bomber.Position) <= 150 {
			remote = false
			break
		}
	}
	type support struct {
		fleet *Fleet
		route *expeditionRoute
	}
	var candidates []support
	for _, f := range t.game.getFleets(p.Num) {
		if landingManaged(f) || f.Starbase || p.FleetInActiveAssembly(f) || fleetHasDedicatedMinesweeper(p, f) || patrolNeededForColony(p, f) {
			continue
		}
		if f == bomber || !patrolOperational(p, f) && !(p.Automation.FleetRoles[f.Num] == "escort" && f.GetTag(escortTag) == strconv.Itoa(bomber.Num) && !f.Delete && !f.InDesignMaintenance() && !f.InAutomaticSafety() && f.GetTag("maintenance-pending") == "" && !fuelNetworkManaged(f) && readTankerJob(f) == nil && f.GetTag(tankerAwaitTag) == "") {
			continue
		}
		if j := readPatrolJob(f); j.Leader != 0 {
			if target := patrolEnemy(p, j); target != nil {
				bombing, deadline := patrolColonyDeadline(p, target)
				if bombing || deadline <= 2 {
					continue
				}
			}
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, bomber.Position, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 1, MinWarp: automaticCruiseWarp(p, f)})
		if route == nil || route.WaitForFuel {
			continue
		}
		candidates = append(candidates, support{f, route})
	}
	slices.SortStableFunc(candidates, func(a, b support) int {
		if a.route.Turns != b.route.Turns {
			return a.route.Turns - b.route.Turns
		}
		if a.fleet.Position.DistanceTo(bomber.Position) < b.fleet.Position.DistanceTo(bomber.Position) {
			return -1
		}
		if a.fleet.Position.DistanceTo(bomber.Position) > b.fleet.Position.DistanceTo(bomber.Position) {
			return 1
		}
		return a.fleet.Num - b.fleet.Num
	})
	group := t.civilianDefense(p, []*Fleet{bomber}, bomber.Position)
	// A bounded defensive reserve cannot be consumed by earlier offensive
	// composition searches. Both budgets remain deterministic and fail closed.
	evaluation := t.defensiveEvaluator(p)
	result := evaluation.assessFormations(p, group, reports, bomber.Position, 2, false)
	selected := []support{}
	for _, c := range candidates[:min(64, len(candidates))] {
		if result.Ready {
			break
		}
		if patrolNeedsRecon(p, reports[0]) {
			break
		}
		if slices.Contains(group, c.fleet) {
			continue
		}
		group = append(group, c.fleet)
		selected = append(selected, c)
		result = evaluation.assessFormations(p, group, reports, bomber.Position, 2, false)
		if evaluation.Work >= evaluation.Limit {
			break
		}
	}
	// Never reserve a crowd indefinitely while equipment is unknown or the
	// forecast cannot confirm defense. Ordinary safety still handles imminent risk.
	if !result.Ready {
		return false, remote
	}
	for _, c := range selected {
		t.detachPatrolMember(p, c.fleet)
		borrowBomberProtection(p, c.fleet, bomber)
		delete(c.fleet.Tags, patrolTag)
		delete(c.fleet.Tags, containmentTag)
		applyPatrolBattlePlan(p, c.fleet, result.Tactic)
		applyExpeditionRoute(p, c.fleet, c.route)
	}
	applyPatrolBattlePlan(p, bomber, result.Tactic)
	holdAutomaticFleet(p, bomber, "Local DD reinforcement arrives this turn; native defense forecast within 5% count/value losses")
	return true, false
}

func bomberImmediateMaximumThreat(p *Player, f *Fleet) int {
	threat := 0
	for _, e := range bomberWarningReports(p, f) {
		if f.Position.DistanceTo(e.Position) <= reportedMaximumSpeed(p, e)+3 {
			threat = min(1_000_000_000, threat+patrolReportedPower(p, e))
		}
	}
	return threat
}
