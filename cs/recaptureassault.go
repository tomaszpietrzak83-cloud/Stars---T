package cs

import (
	"math"
	"slices"
	"strconv"
)

const recaptureAssaultTag = "recapture-assault"

func reportedRecaptureBase(p *Player, planet *Planet, landingRequest ...bool) *Fleet {
	requested := len(landingRequest) > 0 && landingRequest[0]
	if planet == nil || !p.Automation.PriorityPlanets[planet.Num] && !requested || !planet.Owned() || planet.PlayerNum == p.Num || p.IsFriend(planet.PlayerNum) || planet.ReportAge > 2 || !planet.Spec.HasStarbase {
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
	reports := patrolEncounterReports(p, base, base.Position, horizon)
	for _, report := range reports {
		if unknownFleetEquipment(p, report) {
			return patrolAssessment{Reason: "Orbital assault requires complete equipment reconnaissance"}
		}
	}
	return t.recaptureEvaluator(p).assessEncounter(p, group, reports, base.Position, horizon)
}

// A distinct, bounded assault can clear a known orbital base. Ordinary patrols
// retain their blanket base prohibition. Recruit only unassigned local ships,
// physically merge them, and require the existing native <=5% loss forecasts.
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
				}
			}
			if p.Automation.FleetRoles[f.Num] != "patrol" || f.Delete || f.InDesignMaintenance() || f.InAutomaticSafety() || target == nil || !target.Spec.HasStarbase || target.PlayerNum == p.Num || !p.Automation.PriorityPlanets[num] && !landingBaseRequested(p, num, fleets) || p.IsFriend(target.PlayerNum) {
				delete(f.Tags, recaptureAssaultTag)
				continue
			}
			claimed[num] = true
			delete(f.Tags, patrolFrontTag)
			delete(f.Tags, frontierPatrolTag)
			base := t.reportedLandingBase(p, target)
			if base == nil {
				holdAutomaticFleet(p, f, "Orbital assault awaits fresh equipment reconnaissance")
				continue
			}
			result := t.assessRecapture(p, []*Fleet{f}, base)
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
				holdAutomaticFleet(p, f, "Orbital assault awaiting funded return fuel")
				continue
			}
			applyPatrolBattlePlan(p, f, result.Tactic)
			applyExpeditionRoute(p, f, route)
		}
		targets := []*Planet{}
		for _, planet := range p.PlanetIntels {
			if planet != nil && !claimed[planet.Num] && reportedRecaptureBase(p, planet) != nil {
				targets = append(targets, planet)
			}
		}
		slices.SortFunc(targets, func(a, b *Planet) int { return a.Num - b.Num })
		for _, target := range targets {
			base := reportedRecaptureBase(p, target)
			candidates := slices.Clone(fleets)
			slices.SortStableFunc(candidates, func(a, b *Fleet) int {
				da, db := a.Position.DistanceTo(target.Position), b.Position.DistanceTo(target.Position)
				if da < db {
					return -1
				}
				if da > db {
					return 1
				}
				return a.Num - b.Num
			})
			attempted := map[Vector]bool{}
			for _, leader := range candidates {
				if len(attempted) >= 4 {
					break
				}
				if !patrolAvailable(p, leader) || readPatrolJob(leader).Leader != 0 || recaptureDefenceNeeded(p, leader) || leader.Position.DistanceTo(target.Position) > 250 || attempted[leader.Position] {
					continue
				}
				attempted[leader.Position] = true
				group := []*Fleet{}
				result := patrolAssessment{}
				for _, member := range candidates {
					if member.Position != leader.Position || !patrolAvailable(p, member) || readPatrolJob(member).Leader != 0 || recaptureDefenceNeeded(p, member) {
						continue
					}
					group = append(group, member)
					if len(group) > 32 {
						break
					}
					result = t.assessRecapture(p, group, base)
					if result.Ready {
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
				p.Automation.FleetRoles[merged.Num] = "patrol"
				delete(merged.Tags, patrolFrontTag)
				delete(merged.Tags, frontierPatrolTag)
				delete(merged.Tags, patrolTag)
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
	result := t.assessRecapture(p, []*Fleet{f}, base)
	if result.Ready {
		applyPatrolBattlePlan(p, f, result.Tactic)
	}
	return result.Ready
}
