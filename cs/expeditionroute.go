package cs

import (
	"math"
	"slices"
)

type expeditionRoute struct {
	NetworkRequest *automaticFuelRequest
	Waypoints      []Waypoint
	Turns          int
	Fuel           int
	ArrivalFuel    []int
	WaitForFuel    bool
}

// Reserve uses the cheapest safe engine setting, rather than ideal warp.
// Round per turn conservatively; ignore free fuel generation when budgeting.
func expeditionFuel(player *Player, fleet *Fleet, speed int, distance float64) int {
	turns := int(math.Ceil(distance / fleet.StrategicSpeed(speed)))
	cost := max(0, fleet.GetFuelCost(player, speed, math.Ceil(distance)))
	if cost > 0 {
		cost += max(0, turns-1)
	}
	return cost
}

func expeditionStations(player *Player, fleet *Fleet) []*Planet {
	var stations []*Planet
	for _, planet := range player.PlanetIntels {
		if planet != nil && planet.OwnedBy(player.Num) && fleet.CanFuel(player, planet) {
			stations = append(stations, planet)
		}
	}
	return stations
}

func expeditionReserve(rules *Rules, player *Player, fleet *Fleet, position Vector, stations []*Planet) int {
	return automaticReturnFuel(rules, player, fleet, position, stations, false)
}

// Every intermediate node is a real known refuelling dock. Arriving at a dock
// refills at end of turn; no fuel is invented before the first departure.
// The final non-dock waypoint must leave enough fuel to reach a dock again.
func planExpeditionRoute(rules *Rules, player *Player, fleet *Fleet, destination *Planet) *expeditionRoute {
	return planAutomaticRoute(rules, player, []*Fleet{fleet}, destination.Position, destination.MapObject.ToTarget(), automaticRoutePolicy{})
}

func applyExpeditionRoute(player *Player, fleet *Fleet, route *expeditionRoute) {
	delete(fleet.Tags, emergencyReturnTag)
	delete(fleet.Tags, emergencyRiskTag)
	if route.WaitForFuel {
		holdAutomaticFleet(player, fleet, "Waiting at dock for actual refuelling")
		return
	}
	delete(fleet.Tags, fuelStatusTag)
	if route.NetworkRequest != nil {
		saveAutomaticFuelRequest(fleet, *route.NetworkRequest)
		fleet.SetTag(fuelStatusTag, "Planning a reserved multi-stop fuel route")
	}
	fleet.FleetOrders = FleetOrders{BattlePlanNum: fleet.BattlePlanNum, Purpose: fleet.Purpose, Waypoints: slices.Clone(route.Waypoints)}
	fleet.ComputeFuelUsage(player)
}

func expeditionRefuel(rules *Rules, player *Player, fleet *Fleet) bool {
	var best *expeditionRoute
	for _, station := range expeditionStations(player, fleet) {
		if station.Position == fleet.Position {
			routeAutomatically(rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
			return true
		}
		if route := planExpeditionRoute(rules, player, fleet, station); route != nil && (best == nil || route.Turns < best.Turns || route.Turns == best.Turns && route.Fuel < best.Fuel) {
			best = route
		}
	}
	if best == nil {
		return false
	}
	applyExpeditionRoute(player, fleet, best)
	return true
}
