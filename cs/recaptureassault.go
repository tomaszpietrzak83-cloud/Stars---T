package cs

import (
	"math"
	"slices"
	"strconv"
)

const recaptureAssaultTag = "recapture-assault"

func recaptureBaseRequested(p *Player, planet *Planet, landing bool) bool {
	return landing || planet.GetTag(landingPreparationTag) != "" || p.Automation.PriorityPlanets[planet.Num] || patrolOwnTerritory(p, planet.Position, planet.Num)
}

func completedPatrolGroup(p *Player, f *Fleet) bool {
	job := readPatrolJob(f)
	return job.Leader == f.Num && job.Grouped && f.Spec.TotalShips > 1 && patrolEnemy(p, job) == nil &&
		!defencePatrolCommitted(p, f) && !recaptureDefenceNeeded(p, f)
}

func availableRecapturePatrol(p *Player, f *Fleet, territorial bool) bool {
	return !hasAssaultBattleship(p, f) && patrolAvailable(p, f) && (readPatrolJob(f).Leader == 0 || territorial && completedPatrolGroup(p, f))
}

func improvesRecaptureForecast(check, current patrolAssessment) bool {
	return check.Ready && (check.LostShips < current.LostShips ||
		check.LostShips == current.LostShips && check.PeakDamage < current.PeakDamage && check.Loss <= current.Loss)
}

func reportedRecaptureBase(p *Player, planet *Planet, landingRequest ...bool) *Fleet {
	requested := len(landingRequest) > 0 && landingRequest[0]
	if planet == nil || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) ||
		planet.Unexplored() || planet.ReportAge > 2 || !planet.Spec.HasStarbase || !recaptureBaseRequested(p, planet, requested) {
		return nil
	}
	design := p.GetShipDesignIntel(planet.PlayerNum, planet.Spec.StarbaseDesignNum)
	if design == nil || len(design.Slots) == 0 {
		return nil
	}
	// This synthetic report uses only observed design/planet data. Its number is
	// separate from mobile fleet IDs; navigation still targets the actual planet.
	return &Fleet{MapObject: MapObject{Type: MapObjectTypeFleet, Num: -planet.Num, PlayerNum: planet.PlayerNum, Position: planet.Position, Name: planet.Name + " orbital base"}, Starbase: true, Tokens: []ShipToken{{DesignNum: design.Num, Quantity: 1}}, Spec: FleetSpec{ShipDesignSpec: design.Spec, TotalShips: 1}}
}

func (t *turnGenerator) recaptureEvaluator(p *Player) *patrolEvaluator {
	if t.recaptureEvaluation == nil {
		t.recaptureEvaluation = map[int]*patrolEvaluator{}
	}
	if t.recaptureEvaluation[p.Num] == nil {
		t.recaptureEvaluation[p.Num] = &patrolEvaluator{rules: &t.game.Rules, cache: map[[32]byte]patrolAssessment{}, Limit: 1024}
	}
	return t.recaptureEvaluation[p.Num]
}

func recaptureDefenceNeeded(p *Player, f *Fleet) bool {
	for _, task := range p.patrolQueue {
		if task.Kind != "attack" || task.Tier > 1 || task.Deadline > 1 {
			continue
		}
		if e := p.GetFleetIntel(task.Enemy, task.Target); e != nil && f.Position.DistanceTo(e.Position) <= 2*f.StrategicSpeed(automaticMaxWarp(p, f)) {
			return true
		}
	}
	return false
}

func (t *turnGenerator) assessRecapture(p *Player, group []*Fleet, base *Fleet) patrolAssessment {
	horizon := 1.0
	for _, f := range group {
		horizon = max(horizon, math.Ceil(f.Position.DistanceTo(base.Position)/max(1, f.StrategicSpeed(automaticCruiseWarp(p, f)))))
	}
	return t.assessRecaptureArrival(p, group, base, horizon)
}

func (t *turnGenerator) assessRecaptureArrival(p *Player, group []*Fleet, base *Fleet, horizon float64) patrolAssessment {
	reports := patrolEncounterReports(p, base, base.Position, max(1, horizon))
	for _, report := range reports {
		if unknownFleetEquipment(p, report) {
			return patrolAssessment{Reason: "Orbital assault requires complete equipment reconnaissance"}
		}
	}
	return t.recaptureEvaluator(p).assessEncounter(p, group, reports, base.Position, horizon)
}

// Assemble an orbital assault from free patrols and, for territorial targets,
// finished interception groups. Physically merge the ships before departure
// and require the native <=5% loss forecasts.
func (t *turnGenerator) planRecaptureAssaults() {
	for _, p := range t.automaticPlayers() {
		fleets := slices.Clone(t.game.getFleets(p.Num))
		claimed := map[int]bool{}
		for _, f := range fleets {
			num, _ := strconv.Atoi(f.GetTag(recaptureAssaultTag))
			if num == 0 {
				continue
			}
			target := p.GetPlanetIntel(num)
			if target != nil && f.Position == target.Position && !f.Delete {
				// A surviving attacker at this orbit directly observes base destruction.
				if actual := t.game.getPlanet(num); actual != nil && !actual.Spec.HasStarbase {
					target.Spec.HasStarbase = false
					if pureAssaultBattleships(p, f) {
						f.SetTag(battleshipNextAssaultTag, "1")
					}
				}
			}
			if p.Automation.FleetRoles[f.Num] != "patrol" || f.Delete || f.InDesignMaintenance() || f.InAutomaticSafety() || target == nil || !target.Spec.HasStarbase || target.PlayerNum == p.Num || p.IsFriend(target.PlayerNum) ||
				!recaptureBaseRequested(p, target, pureAssaultBattleships(p, f) || landingBaseRequested(p, num, fleets)) {
				delete(f.Tags, recaptureAssaultTag)
				continue
			}
			claimed[num] = true
			delete(f.Tags, patrolFrontTag)
			delete(f.Tags, frontierPatrolTag)
			base := t.reportedLandingBase(p, target)
			if base == nil {
				if !pureAssaultBattleships(p, f) {
					delete(f.Tags, recaptureAssaultTag)
					claimed[num] = false
					restorePatrolBattlePlan(p, f)
				}
				holdAutomaticFleet(p, f, "Orbital assault awaits fresh equipment reconnaissance")
				continue
			}
			evaluator := t.recaptureEvaluator(p)
			limit := evaluator.Limit
			if pureAssaultBattleships(p, f) {
				evaluator.Limit = max(limit, evaluator.Work+2048)
			}
			result := t.assessRecapture(p, []*Fleet{f}, base)
			evaluator.Limit = limit
			if !result.Ready {
				// Let the composition pass recruit actual local reinforcements.
				// Retaining an exclusive reservation here would deadlock this target.
				delete(f.Tags, recaptureAssaultTag)
				claimed[num] = false
				restorePatrolBattlePlan(p, f)
				holdAutomaticFleet(p, f, "Orbital assault: "+result.Reason)
				continue
			}
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MinWarp: automaticCruiseWarp(p, f), MaxTurns: 8})
			if route == nil || route.WaitForFuel {
				if !pureAssaultBattleships(p, f) {
					delete(f.Tags, recaptureAssaultTag)
					claimed[num] = false
					restorePatrolBattlePlan(p, f)
				}
				holdAutomaticFleet(p, f, "Orbital assault awaiting funded return fuel")
				continue
			}
			applyPatrolBattlePlan(p, f, result.Tactic)
			applyExpeditionRoute(p, f, route)
		}
		t.planBattleshipAssaults(p, fleets)
		for _, f := range fleets {
			if num, _ := strconv.Atoi(f.GetTag(recaptureAssaultTag)); num != 0 && !f.Delete {
				claimed[num] = true
			}
		}
		targets := []*Planet{}
		territory := map[int]bool{}
		for _, planet := range p.PlanetIntels {
			if planet != nil && !claimed[planet.Num] && reportedRecaptureBase(p, planet) != nil {
				targets = append(targets, planet)
				territory[planet.Num] = patrolOwnTerritory(p, planet.Position, planet.Num)
			}
		}
		slices.SortFunc(targets, func(a, b *Planet) int {
			if territory[a.Num] != territory[b.Num] {
				if territory[a.Num] {
					return -1
				}
				return 1
			}
			return a.Num - b.Num
		})
		for _, target := range targets {
			base := reportedRecaptureBase(p, target)
			candidates := slices.Clone(fleets)
			finished := map[int]bool{}
			if territory[target.Num] {
				for _, f := range candidates {
					if patrolAvailable(p, f) && completedPatrolGroup(p, f) {
						finished[f.Num] = true
					}
				}
			}
			slices.SortStableFunc(candidates, func(a, b *Fleet) int {
				da, db := a.Position.DistanceTo(target.Position), b.Position.DistanceTo(target.Position)
				if da < db {
					return -1
				}
				if da > db {
					return 1
				}
				if territory[target.Num] {
					if finished[a.Num] != finished[b.Num] {
						if finished[a.Num] {
							return -1
						}
						return 1
					}
					if ships := b.Spec.TotalShips - a.Spec.TotalShips; ships != 0 {
						return ships
					}
				}
				return a.Num - b.Num
			})
			attempted := map[Vector]bool{}
			for _, leader := range candidates {
				if len(attempted) >= 4 {
					break
				}
				if !availableRecapturePatrol(p, leader, territory[target.Num]) || recaptureDefenceNeeded(p, leader) || leader.Position.DistanceTo(target.Position) > 250 || attempted[leader.Position] {
					continue
				}
				attempted[leader.Position] = true
				pool := []*Fleet{}
				for _, member := range candidates {
					if member.Position != leader.Position || !availableRecapturePatrol(p, member, territory[target.Num]) || recaptureDefenceNeeded(p, member) {
						continue
					}
					pool = append(pool, member)
					if len(pool) == 32 {
						break
					}
				}
				var group []*Fleet
				var result patrolAssessment
				for i := range pool {
					group = pool[:i+1]
					result = t.assessRecapture(p, group, base)
					if result.Ready {
						break
					}
					if result.Reason == patrolForecastBudgetExhausted {
						break
					}
				}
				if !result.Ready || len(group) == 0 {
					continue
				}
				route := planAutomaticRoute(&t.game.Rules, p, group, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MinWarp: automaticCruiseWarp(p, leader), MaxTurns: 8})
				if route == nil || route.WaitForFuel {
					continue
				}
				if territory[target.Num] && len(group) < len(pool) && (result.LostShips > 0 || result.PeakDamage > 0) {
					check := t.assessRecapture(p, pool, base)
					if improvesRecaptureForecast(check, result) {
						extraRoute := planAutomaticRoute(&t.game.Rules, p, pool, target.Position, target.ToTarget(), automaticRoutePolicy{AllowThreat: true, Fast: true, MinWarp: automaticCruiseWarp(p, leader), MaxTurns: 8})
						if extraRoute != nil && !extraRoute.WaitForFuel && extraRoute.Turns <= route.Turns {
							group, result, route = pool, check, extraRoute
						}
					}
				}
				merged := leader
				if len(group) > 1 {
					var err error
					merged, err = NewOrderer().Merge(&t.game.Rules, p, group)
					if err != nil {
						continue
					}
					for _, f := range group {
						if f != merged {
							t.game.deleteFleet(f)
						}
					}
				}
				restorePatrolBattlePlan(p, merged)
				t.detachPatrolMember(p, merged)
				p.Automation.FleetRoles[merged.Num] = "patrol"
				delete(merged.Tags, patrolFrontTag)
				delete(merged.Tags, frontierPatrolTag)
				merged.SetTag(recaptureAssaultTag, strconv.Itoa(target.Num))
				applyPatrolBattlePlan(p, merged, result.Tactic)
				applyExpeditionRoute(p, merged, route)
				break
			}
		}
	}
}

func (t *turnGenerator) recaptureDepartureConfirmed(p *Player, f *Fleet, target *Planet) bool {
	if f.GetTag(recaptureAssaultTag) != strconv.Itoa(target.Num) {
		return false
	}
	base := t.reportedLandingBase(p, target)
	if base == nil {
		return false
	}
	evaluator := t.recaptureEvaluator(p)
	if pureAssaultBattleships(p, f) {
		work, limit := evaluator.Work, evaluator.Limit
		evaluator.Limit = max(limit, work+2048)
		defer func() { evaluator.Work, evaluator.Limit = work, limit }()
	}
	result := t.assessRecapture(p, []*Fleet{f}, base)
	if result.Ready {
		applyPatrolBattlePlan(p, f, result.Tactic)
	}
	return result.Ready
}
