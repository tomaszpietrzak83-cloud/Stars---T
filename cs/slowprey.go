package cs

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
)

const containmentTag = "patrol-containment"
const preyWatchTag = "scout-prey-watch"

type containmentJob struct {
	Enemy  int    `json:"enemy"`
	Target int    `json:"target"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

func readContainment(f *Fleet) *containmentJob {
	var job containmentJob
	if json.Unmarshal([]byte(f.GetTag(containmentTag)), &job) != nil {
		return nil
	}
	return &job
}

func reportedFleetShips(f *Fleet) int {
	n := 0
	for _, token := range f.Tokens {
		n += token.Quantity
	}
	return max(n, f.Spec.TotalShips)
}

// A low observed warp is an opportunity, not proof of an empty fuel tank.
func slowPatrolPrey(p *Player, f *Fleet) bool {
	return f != nil && !f.Delete && !f.Starbase && !f.Orbiting() && f.PlayerNum != p.Num && !p.IsFriend(f.PlayerNum) && f.ReportAge >= 0 && f.ReportAge <= 2 && f.WarpSpeed >= 0 && f.WarpSpeed <= 2 && reportedFleetShips(f) >= 8
}

// A public closing course identifies possible fuel delivery or reinforcement.
// No cargo manifests, actual fuel or private joining orders are consulted.
func slowPreySupport(p *Player, f *Fleet) *Fleet {
	if f == nil || f.Delete || f.Starbase || f.ReportAge != 0 || reportedFleetShips(f) > 6 || f.PlayerNum == p.Num || p.IsFriend(f.PlayerNum) {
		return nil
	}
	var chosen *Fleet
	best := math.Inf(1)
	for _, prey := range p.FleetIntels {
		if !slowPatrolPrey(p, prey) || prey == f || prey.PlayerNum != f.PlayerNum {
			continue
		}
		d := f.Position.DistanceTo(prey.Position)
		if d > 220 {
			continue
		}
		velocity := f.Heading.Normalized().Scale(f.observedStrategicSpeed()).Subtract(prey.Heading.Normalized().Scale(prey.observedStrategicSpeed()))
		delta := f.Position.Subtract(prey.Position).ToFloat64()
		closing := delta.X*velocity.X+delta.Y*velocity.Y < 0
		at := 0.0
		if velocity.LengthSquared() > 0 {
			at = Clamp(-(delta.X*velocity.X+delta.Y*velocity.Y)/velocity.LengthSquared(), 0.0, 4.0)
		}
		closest := delta.Add(velocity.Scale(at)).Length()
		if (closing && closest <= 80 || d <= 25) && d < best {
			chosen, best = prey, d
		}
	}
	return chosen
}

func (t *turnGenerator) slowPreyTargets(p *Player) []*Fleet {
	type candidate struct {
		fleet    *Fleet
		distance float64
	}
	var candidates []candidate
	for _, enemy := range p.FleetIntels {
		if !slowPatrolPrey(p, enemy) {
			continue
		}
		distance := math.Inf(1)
		for _, f := range t.game.getFleets(p.Num) {
			if !f.Delete && p.Automation.FleetRoles[f.Num] == "patrol" && !f.InDesignMaintenance() {
				distance = min(distance, f.Position.DistanceTo(enemy.Position))
			}
		}
		if distance <= 300 {
			candidates = append(candidates, candidate{enemy, distance})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if priorityFleet(p, a.fleet) != priorityFleet(p, b.fleet) {
			return priorityFleet(p, a.fleet)
		}
		if a.distance != b.distance {
			return a.distance < b.distance
		}
		if a.fleet.PlayerNum != b.fleet.PlayerNum {
			return a.fleet.PlayerNum < b.fleet.PlayerNum
		}
		return a.fleet.Num < b.fleet.Num
	})
	limit := 2
	if p.patrolEvaluator != nil {
		ships := 0
		for _, f := range t.game.getFleets(p.Num) {
			if patrolOperational(p, f) {
				ships += f.Spec.TotalShips
			}
		}
		limit = max(1, min(8, ships/8))
	}
	result := make([]*Fleet, 0, min(limit, len(candidates)))
	for _, c := range candidates[:min(limit, len(candidates))] {
		result = append(result, c.fleet)
	}
	return result
}

func (t *turnGenerator) refreshSlowPreyOrders() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			job := readContainment(f)
			if job != nil && (!slowPatrolPrey(p, p.GetFleetIntel(job.Enemy, job.Target)) || job.Kind == "collector" && p.Automation.DisableSalvage || p.Automation.FleetRoles[f.Num] != "patrol" && p.Automation.FleetRoles[f.Num] != "freighter") {
				delete(f.Tags, containmentTag)
				if !f.Delete && !f.InAutomaticSafety() && !f.InDesignMaintenance() && logisticsWork(f) == nil {
					holdAutomaticFleet(p, f, "")
				}
			}
			watch := readScoutRecon(f)
			if f.GetTag(preyWatchTag) != "" && (watch == nil || !watch.Watching || !slowPatrolPrey(p, p.GetFleetIntel(watch.Enemy, watch.Target)) || p.Automation.FleetRoles[f.Num] != "scout") {
				delete(f.Tags, preyWatchTag)
				if watch != nil && watch.Watching {
					delete(f.Tags, scoutReconTag)
				}
			}
			restoreCollectionBattlePlan(p, f, false)
		}
	}
}

// Pick the cheapest actual engine setting per distance, with the fastest tied
// setting winning. Radiation's free range therefore selects warp 6, not warp 1.
func economicalContainmentPolicy(p *Player, f *Fleet, distances ...float64) automaticRoutePolicy {
	distance := 100.0
	if len(distances) > 0 {
		distance = distances[0]
	}
	warp, best := 1, math.Inf(1)
	for speed := 1; speed <= automaticMaxWarp(p, f); speed++ {
		if f.StrategicSpeed(speed)*20 < distance {
			continue
		}
		cost := math.Max(0, f.fuelUsage(p, speed, 100))
		if cost <= best {
			warp, best = speed, cost
		}
	}
	return automaticRoutePolicy{EconomicalPatrol: true, MinWarp: warp, MaxWarp: warp, MaxTurns: 20}
}

func preyRingPoint(f *Fleet, enemy *Fleet, radius, angle float64) Vector {
	center := scoutReportedFleetPosition(enemy)
	delta := f.Position.Subtract(center)
	base := math.Atan2(float64(delta.Y), float64(delta.X)) + angle
	return Vector{center.X + int(math.Round(radius*math.Cos(base))), center.Y + int(math.Round(radius*math.Sin(base)))}
}

func (t *turnGenerator) planSlowPreyObservers() {
	for _, p := range t.automaticPlayers() {
		used := map[int]bool{}
		for _, enemy := range t.slowPreyTargets(p) {
			if !unknownFleetEquipment(p, enemy) && reportedContactCovered(p, t.game.getFleets(p.Num), enemy) {
				continue
			}
			firstSlot := 0
			for _, f := range t.game.getFleets(p.Num) {
				if job := readScoutRecon(f); !f.Delete && scoutReconActive(p, f) && job != nil && !job.Watching && job.Enemy == enemy.PlayerNum && job.Target == enemy.Num {
					firstSlot = 1
					used[f.Num] = true
					break
				}
			}
			for slot := firstSlot; slot < 2; slot++ {
				var chosen *Fleet
				var best *expeditionRoute
				bestScore := math.Inf(1)
				for _, f := range t.game.getFleets(p.Num) {
					if used[f.Num] || f.Delete || p.Automation.FleetRoles[f.Num] != "scout" || !scoutReconEligible(p, f) || f.InAutomaticSafety() || f.InDesignMaintenance() || scoutHasRetirement(p, f) || fuelNetworkManaged(f) || f.GetTag(tankerAwaitTag) != "" || p.FleetInActiveAssembly(f) || f.GetTag(priorityReconTag) != "" {
						continue
					}
					if existing := readScoutRecon(f); existing != nil && (!existing.Watching || existing.Minelayer) {
						continue
					}
					rangeAvailable := scoutFleetScanRadius(f, enemy)
					if rangeAvailable < 32 {
						continue
					}
					position := preyRingPoint(f, enemy, min(100, rangeAvailable-8), float64(slot*2-1)*math.Pi/4)
					target := MapObjectTarget{}
					contact := slot == 0 && unknownFleetEquipment(p, enemy) && !p.Race.Spec.DiscoverDesignOnScan
					if contact {
						position = enemy.Position
						target = enemy.ToTarget()
					}
					route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, target, automaticRoutePolicy{AllowThreat: contact, MaxTurns: 8, MinWarp: automaticCruiseWarp(p, f)})
					if route == nil || route.WaitForFuel {
						continue
					}
					score := float64(route.Turns)*1000 + f.Position.DistanceTo(position)
					if old := readScoutRecon(f); old != nil && old.Watching && old.Enemy == enemy.PlayerNum && old.Target == enemy.Num {
						score -= 500
					}
					if score < bestScore {
						chosen, best, bestScore = f, route, score
					}
				}
				if chosen == nil {
					break
				}
				used[chosen.Num] = true
				job := scoutReconJob{Enemy: enemy.PlayerNum, Target: enemy.Num, Watching: true, Status: fmt.Sprintf("Watching slow formation %s from scanner range (%d/2)", enemy.Name, slot+1)}
				if slot == 0 && unknownFleetEquipment(p, enemy) && !p.Race.Spec.DiscoverDesignOnScan {
					job.Watching = false
					job.Status = "Scout equipment reconnaissance before patrol attack on " + enemy.Name
				}
				data, _ := json.Marshal(job)
				chosen.SetTag(scoutReconTag, string(data))
				chosen.SetTag(preyWatchTag, "1")
				chosen.SetTag(scoutTag, job.Status)
				applyExpeditionRoute(p, chosen, best)
				if !job.Watching && len(chosen.Waypoints) > 1 {
					chosen.Waypoints[len(chosen.Waypoints)-1].MapObjectTarget = enemy.ToTarget()
				}
			}
		}
		for _, f := range t.game.getFleets(p.Num) {
			if !used[f.Num] && f.GetTag(preyWatchTag) != "" {
				delete(f.Tags, preyWatchTag)
				delete(f.Tags, scoutReconTag)
			}
		}
	}
}

func (t *turnGenerator) planSlowPreyContainment() {
	for _, p := range t.automaticPlayers() {
		if p.patrolEvaluator != nil {
			// Combat screening shares the patrol mission queue and reservations.
			// A second containment controller must not immobilize ready fighters.
			for _, f := range t.game.getFleets(p.Num) {
				if j := readContainment(f); j != nil && j.Kind == "screen" {
					delete(f.Tags, containmentTag)
				}
			}
			continue
		}
		targets := t.slowPreyTargets(p)
		if len(targets) == 0 {
			continue
		}
		// Keep at least a third of the patrol fleet for colony defense, escorts
		// and interception of small support ships outside the containment group.
		total := 0
		for _, f := range t.game.getFleets(p.Num) {
			if patrolOperational(p, f) && p.Automation.FleetRoles[f.Num] == "patrol" {
				total++
			}
		}
		budget := min(24, max(1, total*2/3))
		used := map[int]bool{}
		for targetIndex, enemy := range targets {
			targetBudget := (budget + len(targets) - targetIndex - 1) / (len(targets) - targetIndex)
			for _, f := range slices.Clone(t.game.getFleets(p.Num)) {
				job := readPatrolJob(f)
				current := patrolEnemy(p, job)
				if job.Leader == f.Num && current == enemy && !priorityFleet(p, current) && !enemyApproachingColony(p, current) && (!patrolCanAttack(p, f) || f.Position.DistanceTo(patrolInterceptPosition(current)) > f.StrategicSpeed(automaticMaxWarp(p, f))) {
					t.releasePatrol(p, f)
				}
			}
			fleets := slices.Clone(t.game.getFleets(p.Num))
			sort.SliceStable(fleets, func(i, j int) bool {
				return fleets[i].Position.DistanceTo(enemy.Position) < fleets[j].Position.DistanceTo(enemy.Position)
			})
			group := []*Fleet{}
			for _, f := range fleets {
				if len(group) >= targetBudget {
					break
				}
				if used[f.Num] || !patrolOperational(p, f) || p.Automation.FleetRoles[f.Num] != "patrol" || readPatrolJob(f).Leader != 0 || f.Position.DistanceTo(enemy.Position) > 300 || f.GetTag(escortTag) != "" {
					continue
				}
				if old := readContainment(f); old != nil && (old.Enemy != enemy.PlayerNum || old.Target != enemy.Num) {
					continue
				}
				group = append(group, f)
			}
			power := 0
			for _, f := range group {
				power += patrolPower(f)
			}
			ready := power >= patrolThreat(p, enemy)*3/2+1 && !unknownFleetEquipment(p, enemy) && enemy.ReportAge == 0
			var rally Vector
			if len(group) > 0 {
				rally = preyRingPoint(group[0], enemy, 75, 0)
			}
			assigned := []*Fleet{}
			for i, f := range group {
				position := preyRingPoint(f, enemy, 80+float64(i%3)*12, float64(i%5-2)*math.Pi/8)
				if ready {
					position = rally
				}
				route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, MapObjectTarget{}, economicalContainmentPolicy(p, f, f.Position.DistanceTo(position)))
				if route == nil || route.WaitForFuel {
					continue
				}
				job := containmentJob{Enemy: enemy.PlayerNum, Target: enemy.Num, Kind: "screen", Status: "Containing slow formation " + enemy.Name + "; gathering strength and cutting off support"}
				if ready {
					job.Status = "Gathering at a safe attack rendezvous for " + enemy.Name
				}
				data, _ := json.Marshal(job)
				f.SetTag(containmentTag, string(data))
				applyExpeditionRoute(p, f, route)
				used[f.Num] = true
				assigned = append(assigned, f)
			}
			budget -= len(assigned)
			if ready && len(assigned) > 0 {
				leader := assigned[0]
				for _, f := range assigned[1:] {
					if f.Position == leader.Position {
						if _, err := NewOrderer().Merge(&t.game.Rules, p, []*Fleet{leader, f}); err == nil {
							t.game.deleteFleet(f)
						}
					}
				}
				if patrolPower(leader) >= patrolThreat(p, enemy)*3/2+1 {
					delete(leader.Tags, containmentTag)
					savePatrolJob(leader, patrolJob{Leader: leader.Num, Enemy: enemy.PlayerNum, Target: enemy.Num, Grouped: leader.Spec.TotalShips > 1, RequiredShips: 1, GatheringSince: t.game.Year, Status: "Coordinated attack on slow formation " + enemy.Name})
					t.advancePatrol(p, leader)
				}
			}
			// Empty collectors stage outside the future battle while the group
			// gathers; the battle collector planner takes over the final approach.
			if !p.Automation.DisableSalvage {
				for _, f := range fleets {
					if f.Delete || p.Automation.FleetRoles[f.Num] != "freighter" || f.InAutomaticSafety() || f.InDesignMaintenance() || f.GetTag("maintenance-pending") != "" || fuelNetworkManaged(f) || readTankerJob(f) != nil || f.GetTag(tankerAwaitTag) != "" || p.FleetInActiveAssembly(f) || f.GetTag(traderConvoyTag) != "" || f.GetTag(anticipatedSalvageTag) != "" || logisticsWork(f) != nil || f.CargoMass() > 0 || f.Spec.CargoCapacity <= 0 || f.Position.DistanceTo(enemy.Position) > 300 {
						continue
					}
					if old := readContainment(f); old != nil && (old.Enemy != enemy.PlayerNum || old.Target != enemy.Num) {
						continue
					}
					position := preyRingPoint(f, enemy, max(28, min(60, f.StrategicSpeed(automaticMaxWarp(p, f))-8)), 0)
					route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, MapObjectTarget{}, economicalContainmentPolicy(p, f, f.Position.DistanceTo(position)))
					if route == nil || route.WaitForFuel {
						continue
					}
					job := containmentJob{Enemy: enemy.PlayerNum, Target: enemy.Num, Kind: "collector", Status: "Staging behind patrols for debris from " + enemy.Name}
					data, _ := json.Marshal(job)
					f.SetTag(containmentTag, string(data))
					setCollectionBattlePlan(p, f)
					applyExpeditionRoute(p, f, route)
					used[f.Num] = true
					break
				}
			}
		}
		for _, f := range t.game.getFleets(p.Num) {
			if f.GetTag(containmentTag) != "" && !used[f.Num] {
				delete(f.Tags, containmentTag)
			}
		}
	}
}
