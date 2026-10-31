package cs

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
)

const colonizationTag = "colonization-job"

type colonizationJob struct {
	TargetNum   int     `json:"targetNum"`
	Mode        string  `json:"mode"`
	Kind        string  `json:"kind"`
	Quality     int     `json:"quality,omitempty"`
	TravelYears int     `json:"travelYears,omitempty"`
	Score       float64 `json:"score,omitempty"`
	Status      string  `json:"status"`
}

func colonizationMode(role string) string {
	switch role {
	case "colonizer:auto", "colonizer:nearest", "colonizer:farthest", "colonizer:best":
		return strings.TrimPrefix(role, "colonizer:")
	}
	return ""
}
func readColonizationJob(fleet *Fleet) *colonizationJob {
	var job colonizationJob
	if json.Unmarshal([]byte(fleet.GetTag(colonizationTag)), &job) != nil {
		return nil
	}
	return &job
}
func writeColonizationJob(fleet *Fleet, job *colonizationJob) {
	data, _ := json.Marshal(job)
	fleet.SetTag(colonizationTag, string(data))
}
func (player *Player) CanAutomateColonizer(fleet *Fleet) bool {
	if fleet == nil || fleet.Delete || fleet.Starbase || len(fleet.Tokens) == 0 {
		return false
	}
	for _, token := range fleet.Tokens {
		design := player.GetDesign(token.DesignNum)
		if design == nil || !design.Spec.Colonizer {
			return false
		}
	}
	return true
}
func colonizationOneWay(player *Player, fleet *Fleet, goal Waypoint) bool {
	job := readColonizationJob(fleet)
	return colonizationMode(player.Automation.FleetRoles[fleet.Num]) != "" && job != nil && job.Kind == "colonize" && goal.TargetType == MapObjectTypePlanet && goal.TargetNum == job.TargetNum && goal.Task == WaypointTaskColonize && fleet.Cargo.Colonists > 0 && player.CanAutomateColonizer(fleet)
}
func colonyPlanetQuality(player *Player, planet *Planet) int {
	if planet == nil || !planet.Explored() || planet.Owned() {
		return 0
	}
	return max(0, prospectivePlanetQuality(player, planet))
}
func colonyAutoScore(quality, travelYears int) float64 {
	return float64(quality*quality) / float64(4+max(0, travelYears))
}

// Colonization scraps the entire fleet in the original turn engine. Split
// colony ships before assigning targets, and never consume an attached escort.
func (t *turnGenerator) splitAutomaticColonizers(player *Player) {
	for _, fleet := range slices.Clone(t.game.getFleets(player.Num)) {
		role := player.Automation.FleetRoles[fleet.Num]
		if fleet.GetTag("maintenance-pending") != "" || fuelNetworkManaged(fleet) || colonizationMode(role) == "" || fleet.InDesignMaintenance() || !player.CanAutomateColonizer(fleet) || fleet.Spec.TotalShips <= 1 {
			continue
		}
		children, err := NewOrderer().SplitAll(&t.game.Rules, player, t.game.getFleets(player.Num), fleet)
		if err != nil {
			continue
		}
		for _, child := range children {
			t.game.Fleets = append(t.game.Fleets, child)
			_ = t.game.Universe.addFleet(child)
			player.Automation.FleetRoles[child.Num] = role
		}
	}
}

func (t *turnGenerator) loadColonizer(player *Player, fleet *Fleet, job *colonizationJob) bool {
	var source *Planet
	var route *expeditionRoute
	for _, planet := range t.game.getPlanets(player.Num) {
		// Fill available passenger capacity; retain half the population and at least 5,000.
		if planet.Cargo.Colonists <= max(50, (planet.Cargo.Colonists+1)/2) {
			continue
		}
		candidate := planExpeditionRoute(&t.game.Rules, player, fleet, planet)
		if candidate != nil && (route == nil || candidate.Turns < route.Turns || candidate.Turns == route.Turns && planet.Num < source.Num) {
			source, route = planet, candidate
		}
	}
	if source == nil {
		holdAutomaticFleet(player, fleet, "Waiting for an owned colony with spare settlers")
		requestAutomaticRescue(player, fleet)
		job.Status = "Waiting for settlers and a safe route"
		return false
	}
	job.TargetNum, job.Kind = source.Num, "load"
	if fleet.Position != source.Position {
		applyExpeditionRoute(player, fleet, route)
		job.Status = "Collecting settlers from " + source.Name
		return false
	}
	keep := max(50, (source.Cargo.Colonists+1)/2)
	// Minerals are unloaded at the owned source to make room for settlers.
	source.Cargo = source.Cargo.AddMineral(fleet.Cargo.ToMineral())
	fleet.Cargo.Ironium, fleet.Cargo.Boranium, fleet.Cargo.Germanium = 0, 0, 0
	amount := min(max(0, fleet.Spec.CargoCapacity-fleet.CargoMass()), max(0, source.Cargo.Colonists-keep))
	if amount == 0 {
		holdAutomaticFleet(player, fleet, "")
		job.Status = "No free cargo space or spare settlers"
		return false
	}
	source.Cargo.Colonists -= amount
	fleet.Cargo.Colonists += amount
	job.TargetNum = 0
	job.Kind = "colonize"
	return true
}

func (t *turnGenerator) planColonizationAutomation() {
	for _, player := range t.automaticPlayers() {
		if !t.ordersOnly {
			t.splitAutomaticColonizers(player)
		}
		fleets := slices.Clone(t.game.getFleets(player.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		claimed := map[int]int{}
		for _, fleet := range fleets {
			if !fleet.Delete && colonizationMode(player.Automation.FleetRoles[fleet.Num]) == "" && fleet.Cargo.Colonists > 0 && fleet.Spec.Colonizer {
				for _, wp := range fleet.Waypoints {
					if wp.Task == WaypointTaskColonize && wp.TargetType == MapObjectTypePlanet && colonyPlanetQuality(player, player.GetPlanetIntel(wp.TargetNum)) > 0 {
						claimed[wp.TargetNum] = fleet.Num
					}
				}
			}
		}
		// Reserve valid existing assignments before a newly built ship chooses.
		for _, fleet := range fleets {
			if job := readColonizationJob(fleet); job != nil && job.Kind == "colonize" && !fleet.Delete && colonizationMode(player.Automation.FleetRoles[fleet.Num]) != "" && colonyPlanetQuality(player, player.GetPlanetIntel(job.TargetNum)) > 0 && claimed[job.TargetNum] == 0 {
				claimed[job.TargetNum] = fleet.Num
			}
		}
		for _, fleet := range fleets {
			mode := colonizationMode(player.Automation.FleetRoles[fleet.Num])
			if mode != "" && !fleet.Delete && !player.CanAutomateColonizer(fleet) {
				holdAutomaticFleet(player, fleet, "Automatic colonization remains enabled: waiting for an unmixed colony-ship fleet")
				writeColonizationJob(fleet, &colonizationJob{Mode: mode, Status: "Waiting for colony ships only; automatic role remains enabled"})
				continue
			}
			if mode == "" {
				delete(fleet.Tags, colonizationTag)
			}
			if fleet.GetTag("maintenance-pending") != "" || fuelNetworkManaged(fleet) || fleet.InAutomaticSafety() || fleet.Delete || fleet.Starbase || fleet.InDesignMaintenance() || mode == "" || !player.CanAutomateColonizer(fleet) {
				continue
			}
			job := readColonizationJob(fleet)
			if job == nil {
				job = &colonizationJob{Mode: mode}
			}
			topUp := false
			if fleet.Cargo.Colonists < fleet.Spec.CargoCapacity {
				for _, planet := range t.game.getPlanets(player.Num) {
					if planet.Position == fleet.Position && planet.Cargo.Colonists > max(50, (planet.Cargo.Colonists+1)/2) {
						topUp = true
						break
					}
				}
			}
			if fleet.Cargo.Colonists == 0 || topUp {
				if !t.loadColonizer(player, fleet, job) {
					writeColonizationJob(fleet, job)
					continue
				}
			}
			var chosen *Planet
			var route *expeditionRoute
			if job.Mode == mode && job.Kind == "colonize" && (player.Automation.PriorityPlanets[job.TargetNum] || len(player.Automation.PriorityPlanets) == 0) {
				if candidate := player.GetPlanetIntel(job.TargetNum); colonyPlanetQuality(player, candidate) > 0 && (claimed[candidate.Num] == 0 || claimed[candidate.Num] == fleet.Num) {
					route = planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, candidate.Position, candidate.MapObject.ToTarget(), automaticRoutePolicy{OneWay: true})
					if route != nil {
						chosen = candidate
					}
				}
			}
			if chosen == nil {
				if claimed[job.TargetNum] == fleet.Num {
					delete(claimed, job.TargetNum)
				}
				best := -math.MaxFloat64
				bestQuality := 0
				bestDistance := 0.0
				for _, planet := range player.PlanetIntels {
					quality := colonyPlanetQuality(player, planet)
					if quality <= 0 || claimed[planet.Num] != 0 {
						continue
					}
					candidate := planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, planet.Position, planet.MapObject.ToTarget(), automaticRoutePolicy{OneWay: true})
					if candidate == nil {
						continue
					}
					distance := fleet.Position.DistanceTo(planet.Position)
					score := colonyAutoScore(quality, candidate.Turns)
					switch mode {
					case "nearest":
						score = -distance
					case "farthest":
						score = distance
					case "best":
						score = float64(quality)
					}
					priority, currentPriority := player.Automation.PriorityPlanets[planet.Num], chosen != nil && player.Automation.PriorityPlanets[chosen.Num]
					better := chosen == nil || priority && !currentPriority || priority == currentPriority && (score > best || score == best && (candidate.Turns < route.Turns || candidate.Turns == route.Turns && (quality > bestQuality || quality == bestQuality && (distance < bestDistance || distance == bestDistance && planet.Num < chosen.Num))))
					if better {
						chosen, route, best, bestQuality, bestDistance = planet, candidate, score, quality, distance
					}
				}
			}
			if chosen == nil {
				chosen, route = t.assistedColonyTarget(player, fleet, mode, claimed)
			}
			if chosen == nil {
				job.TargetNum = 0
				job.Mode = mode
				job.Status = "Waiting for a known, habitable, unclaimed planet with a safe fuel route"
				holdAutomaticFleet(player, fleet, job.Status)
				if !expeditionRefuel(&t.game.Rules, player, fleet) && !t.routeEmergencyReturn(player, fleet) {
					requestAutomaticRescue(player, fleet, true)
				}
				writeColonizationJob(fleet, job)
				continue
			}
			job.TargetNum, job.Mode, job.Kind = chosen.Num, mode, "colonize"
			claimed[chosen.Num] = fleet.Num
			if faster := t.planAutomaticMission(player, []*Fleet{fleet}, chosen.Position, chosen.MapObject.ToTarget(), automaticRoutePolicy{OneWay: true, MaxTurns: 50}); faster != nil {
				route = faster
			}
			job.Quality = colonyPlanetQuality(player, chosen)
			if route == nil {
				job.Status = "Holding colony reservation: waiting for fuel or a safe route"
				holdAutomaticFleet(player, fleet, job.Status)
				saveAutomaticFuelRequest(fleet, automaticFuelRequest{Destination: chosen.Position, Target: chosen.MapObject.ToTarget(), Members: []int{fleet.Num}, Urgent: true, OneWay: true})
			} else {
				job.TravelYears = route.Turns
				job.Score = colonyAutoScore(job.Quality, route.Turns)
				applyExpeditionRoute(player, fleet, route)
				if !route.WaitForFuel {
					last := len(fleet.Waypoints) - 1
					fleet.Waypoints[last].MapObjectTarget = chosen.MapObject.ToTarget()
					fleet.Waypoints[last].Task = WaypointTaskColonize
				}
				job.Status = fmt.Sprintf("Colonizing %s - habitability %d%%, ETA %d years, auto score %.1f", chosen.Name, job.Quality, route.Turns, job.Score)
				if route.WaitForFuel {
					job.Status = "Refuelling before colonizing " + chosen.Name
				}
			}
			writeColonizationJob(fleet, job)
		}
	}
}

// Inherit roles, not reservations or rendezvous: new fleets choose independent jobs.
func (p *Player) InheritAutomaticRole(source, child *Fleet) {
	role := p.Automation.FleetRoles[source.Num]
	if role == "" || child == nil || child.Delete {
		return
	}
	if p.ValidateLogisticsAutomation(LogisticsAutomation{FleetRoles: map[int]string{child.Num: role}}, []*Fleet{child}) != nil {
		return
	}
	p.Automation.FleetRoles[child.Num] = role
	if role == "scout" {
		if mission := readScoutMission(source); mission != nil {
			mission.Target = 0
			mission.Plan = nil
			mission.Leg = 0
			mission.Refuelling = false
			mission.Complete = false
			saveScoutMission(child, mission)
		}
	}
}
