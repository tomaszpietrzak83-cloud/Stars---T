package cs

import (
	"fmt"
	"math"
	"slices"
)

type scoutPetalCandidate struct {
	planet   *Planet
	estimate float64
}

func scoutAngleDifference(a, b float64) float64 {
	return math.Abs(math.Atan2(math.Sin(a-b), math.Cos(a-b)))
}
func scoutBearing(origin, position Vector) float64 {
	return math.Atan2(float64(position.Y-origin.Y), float64(position.X-origin.X))
}
func scoutVirtualArrival(fleet *Fleet, planet *Planet, route *expeditionRoute) *Fleet {
	copy := *fleet
	copy.Position = planet.Position
	copy.OrbitingPlanetNum = planet.Num
	if len(route.ArrivalFuel) > 0 {
		copy.Fuel = route.ArrivalFuel[0]
	}
	return &copy
}
func scoutHome(player *Player, fleet *Fleet, mission *scoutMission) *Planet {
	var home *Planet
	best := math.Inf(1)
	for _, station := range expeditionStations(player, fleet) {
		if station.Num == mission.Home {
			return station
		}
		if distance := station.Position.DistanceTo(mission.Origin); distance < best {
			home, best = station, distance
		}
	}
	return home
}
func scoutHasRetirement(player *Player, fleet *Fleet) bool {
	for _, token := range fleet.Tokens {
		if player.DesignMaintenance[token.DesignNum] != 0 {
			return true
		}
	}
	return false
}
func scoutHomeRoute(rules *Rules, player *Player, fleet *Fleet, home *Planet) *expeditionRoute {
	if home == nil {
		return nil
	}
	return planAutomaticRoute(rules, player, []*Fleet{fleet}, home.Position, home.MapObject.ToTarget(), automaticRoutePolicy{})
}

// Plan both arms against a simulated tank. Every stop must still admit a real
// route back to the selected home dock within the player's return-time budget.
func (t *turnGenerator) buildScoutPetal(player *Player, fleet *Fleet, mission *scoutMission, home *Planet, claimed map[int]int, quota int, returnOnly bool) {
	mission.Plan = nil
	mission.Leg = 0
	mission.ReturnAt = 0
	mission.Home = home.Num
	mission.Complete = false
	virtual := *fleet
	radius := virtual.Position.DistanceTo(mission.Origin)
	centre := math.Mod(float64(fleet.Num)*2.399963229728653+float64(mission.Cycle)*.7, 2*math.Pi)
	if radius > 1 {
		centre = scoutBearing(mission.Origin, virtual.Position)
	}
	used := map[int]bool{}
	outTurns := 0
	returnTurns := 0
	limit := player.Automation.scoutReturnTurns()
	choose := func(outward bool, slots int) {
		for range slots {
			radius = virtual.Position.DistanceTo(mission.Origin)
			bearing := centre + .3
			if outward {
				bearing = centre - .3
			}
			shortlist := []scoutPetalCandidate{}
			for _, planet := range player.PlanetIntels {
				if planet == nil || !planet.Unexplored() || used[planet.Num] || (claimed[planet.Num] != 0 && claimed[planet.Num] != fleet.Num) {
					continue
				}
				nextRadius := planet.Position.DistanceTo(mission.Origin)
				if outward && nextRadius <= radius+.01 || !outward && nextRadius >= radius-.01 {
					continue
				}
				angle := scoutAngleDifference(scoutBearing(mission.Origin, planet.Position), bearing)
				// At the hub, allow the first leg to find a viable sector near map edges.
				if angle > 1.0 && (len(mission.Plan) > 0 || radius > 1) {
					continue
				}
				estimate := virtual.Position.DistanceTo(planet.Position)/max(1, virtual.StrategicSpeed(automaticMaxWarp(player, &virtual))) + 6*angle*angle
				shortlist = append(shortlist, scoutPetalCandidate{planet, estimate})
			}
			slices.SortFunc(shortlist, func(a, b scoutPetalCandidate) int {
				if a.estimate < b.estimate {
					return -1
				}
				if a.estimate > b.estimate {
					return 1
				}
				return a.planet.Num - b.planet.Num
			})
			if len(shortlist) > 24 {
				shortlist = shortlist[:24]
			}
			var selected *Planet
			var selectedRoute *expeditionRoute
			best := math.Inf(1)
			directHome := scoutHomeRoute(&t.game.Rules, player, &virtual, home)
			for _, candidate := range shortlist {
				route := planExpeditionRoute(&t.game.Rules, player, &virtual, candidate.planet)
				if route == nil {
					continue
				}
				after := scoutVirtualArrival(&virtual, candidate.planet, route)
				back := scoutHomeRoute(&t.game.Rules, player, after, home)
				if back == nil || back.Turns > limit {
					continue
				}
				if outward && outTurns+route.Turns > limit {
					continue
				}
				if !outward && (returnTurns+route.Turns+back.Turns > limit || directHome != nil && route.Turns+back.Turns > directHome.Turns+3) {
					continue
				}
				angle := scoutAngleDifference(scoutBearing(mission.Origin, candidate.planet.Position), bearing)
				score := float64(route.Turns) + .25*float64(back.Turns) + 4*angle*angle + .002*float64(route.Fuel)
				if score < best {
					selected, selectedRoute, best = candidate.planet, route, score
				}
			}
			if selected == nil {
				break
			}
			if len(mission.Plan) == 0 && radius <= 1 {
				centre = scoutBearing(mission.Origin, selected.Position) + .3
			}
			virtual = *scoutVirtualArrival(&virtual, selected, selectedRoute)
			mission.Plan = append(mission.Plan, selected.Num)
			used[selected.Num] = true
			if outward {
				outTurns += selectedRoute.Turns
			} else {
				returnTurns += selectedRoute.Turns
			}
		}
	}
	if !returnOnly {
		choose(true, max(1, (quota+1)/2))
	}
	mission.ReturnAt = len(mission.Plan)
	choose(false, max(0, quota-len(mission.Plan)))
	mission.Plan = append(mission.Plan, home.Num)
}
func (t *turnGenerator) advanceScoutPetal(player *Player, fleet *Fleet, mission *scoutMission, claimed map[int]int, quota int) {
	home := scoutHome(player, fleet, mission)
	if home == nil {
		holdAutomaticFleet(player, fleet, "Scout remains enabled: waiting for a known fuel dock")
		requestAutomaticRescue(player, fleet, true)
		fleet.SetTag(scoutTag, "Waiting for a fuel dock or tanker assistance")
		return
	}
	if mission.Home != 0 && mission.Home != home.Num {
		mission.Plan = nil
		mission.Leg = 0
		mission.Refuelling = true
	}
	mission.Home = home.Num
	if mission.Refuelling {
		if fleet.Position != home.Position || fleet.Fuel < fleet.Spec.FuelCapacity {
			if route := scoutHomeRoute(&t.game.Rules, player, fleet, home); route != nil {
				applyExpeditionRoute(player, fleet, route)
			} else {
				holdAutomaticFleet(player, fleet, "Scout return needs fuel assistance")
				requestAutomaticRescue(player, fleet, true)
			}
			fleet.SetTag(scoutTag, "Returning for fuel; automatic exploration remains enabled")
			return
		}
		mission.Refuelling = false
		mission.Plan = nil
		mission.Leg = 0
		mission.Cycle++
	}
	if mission.Complete && scoutHasRetirement(player, fleet) {
		mission.Refuelling = true
		fleet.SetTag(scoutTag, "Expedition finished; returning for scrapping or replacement")
		if route := scoutHomeRoute(&t.game.Rules, player, fleet, home); route != nil {
			applyExpeditionRoute(player, fleet, route)
		}
		return
	}
	if len(mission.Plan) == 0 {
		t.buildScoutPetal(player, fleet, mission, home, claimed, quota, false)
	}
	for mission.Leg < len(mission.Plan) {
		target := player.GetPlanetIntel(mission.Plan[mission.Leg])
		if target == nil || target.Num != home.Num && (!target.Unexplored() || target.Position == fleet.Position) {
			mission.Leg++
			continue
		}
		if target.Num == home.Num && fleet.Position == home.Position {
			mission.Complete = true
			mission.Target = 0
			mission.Refuelling = fleet.Fuel < fleet.Spec.FuelCapacity
			holdAutomaticFleet(player, fleet, "Awaiting fuel or the next petal expedition")
			fleet.SetTag(scoutTag, "Petal complete at fuel dock; exploration remains enabled")
			mission.Plan = nil
			mission.Leg = 0
			return
		}
		route := t.planAutomaticMission(player, []*Fleet{fleet}, target.Position, target.MapObject.ToTarget(), automaticRoutePolicy{})
		valid := route != nil
		if valid && target.Num != home.Num {
			back := scoutHomeRoute(&t.game.Rules, player, scoutVirtualArrival(fleet, target, route), home)
			valid = back != nil && back.Turns <= player.Automation.scoutReturnTurns()
		}
		if !valid {
			// Abort only the unsafe itinerary, never the automatic role. Continue a
			// fuel-checked return arm if there are cheap undiscovered worlds on the way.
			t.buildScoutPetal(player, fleet, mission, home, claimed, quota, true)
			if len(mission.Plan) > 1 {
				target = player.GetPlanetIntel(mission.Plan[0])
				route = planExpeditionRoute(&t.game.Rules, player, fleet, target)
			} else {
				target = home
				route = scoutHomeRoute(&t.game.Rules, player, fleet, home)
			}
			if route == nil {
				holdAutomaticFleet(player, fleet, "Scout waiting for fuel or a safe return route")
				requestAutomaticRescue(player, fleet, true)
				fleet.SetTag(scoutTag, "Waiting for a safe return or tanker; role remains enabled")
				return
			}
		}
		applyExpeditionRoute(player, fleet, route)
		mission.Target = target.Num
		mission.Radius = max(mission.Radius, fleet.Position.DistanceTo(mission.Origin))
		stage := "outward"
		if mission.Leg >= mission.ReturnAt {
			stage = "return scanning"
		}
		fleet.SetTag(scoutTag, fmt.Sprintf("Petal %d, %s: %s (%d/%d legs); fuel-checked return to %s", mission.Cycle+1, stage, target.Name, mission.Leg+1, len(mission.Plan), home.Name))
		return
	}
	mission.Plan = nil
	mission.Leg = 0
	mission.Refuelling = true
}
