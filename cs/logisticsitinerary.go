package cs

import (
	"maps"
	"math"
	"slices"
	"strconv"
)

const logisticsReturnTag = "logistics-return-dock"

// Include the funded return in actual native waypoints, not just map artwork.
// Budget the return with the full payload too, so a crowded destination cannot
// strand the transport if it has to keep some passengers on board.
func (t *turnGenerator) appendLogisticsReturn(p *Player, group []*Fleet, job *logisticsJob, route *expeditionRoute) {
	if job.Kind != "settle" && job.Kind != "return" || len(group) == 0 || route == nil || route.WaitForFuel || route.NetworkRequest != nil || len(route.ArrivalFuel) != len(group) {
		return
	}
	virtual := make([]*Fleet, len(group))
	dest := p.GetPlanetIntel(job.TargetNum)
	for i, f := range group {
		copy := *f
		copy.Tags = maps.Clone(f.Tags)
		copy.Position = job.Position
		copy.Fuel = route.ArrivalFuel[i]
		if dest != nil && dest.OwnedBy(p.Num) && dest.Spec.HasStarbase && dest.Spec.DockCapacity != 0 {
			copy.Fuel = copy.Spec.FuelCapacity
		}
		virtual[i] = &copy
	}
	var best *expeditionRoute
	var chosen *Planet
	for _, dock := range expeditionStations(p, group[0]) {
		if dock.Position == job.Position {
			continue
		}
		back := planAutomaticRoute(&t.game.Rules, p, virtual, dock.Position, dock.ToTarget(), automaticRoutePolicy{MaxTurns: 20, MinWarp: automaticCruiseWarp(p, group[0])})
		if back == nil || back.WaitForFuel {
			continue
		}
		// Prefer returning to the loading dock when it is a reachable real dock.
		if dock.Position == group[0].Position {
			best, chosen = back, dock
			break
		}
		if best == nil || back.Turns < best.Turns {
			best, chosen = back, dock
		}
	}
	if best == nil {
		return
	}
	for _, f := range group {
		f.Waypoints = append(slices.Clone(f.Waypoints), best.Waypoints[1:]...)
		f.SetTag(logisticsReturnTag, strconv.Itoa(chosen.Num))
		f.ComputeFuelUsage(p)
	}
}

func fundedLogisticsWaypoints(rules *Rules, p *Player, f *Fleet) bool {
	if f.GetTag(logisticsReturnTag) == "" || len(f.Waypoints) < 2 {
		return false
	}
	last := f.Waypoints[len(f.Waypoints)-1]
	dock := p.GetPlanetIntel(last.TargetNum)
	if dock == nil || !dock.OwnedBy(p.Num) || !dock.Spec.HasStarbase || dock.Spec.DockCapacity == 0 {
		return false
	}
	fuel, from := f.Fuel, f.Position
	for _, wp := range f.Waypoints[1:] {
		years := max(1, int(math.Ceil(from.DistanceTo(wp.Position)/max(1, f.StrategicSpeed(wp.WarpSpeed)))))
		if wp.WarpSpeed < 1 || wp.WarpSpeed > automaticMaxWarp(p, f) || years > 20 || wp.WarpSpeed < 4 && years > 2 || !automaticLegSafe(rules, p, from, wp.Position, wp.WarpSpeed, false, f.StrategicSpeed(wp.WarpSpeed)) {
			return false
		}
		_, left, _, ok := automaticLegFuel(p, f, wp.WarpSpeed, from.DistanceTo(wp.Position), fuel, 0)
		if !ok {
			return false
		}
		fuel = left
		from = wp.Position
		planet := p.GetPlanetIntel(wp.TargetNum)
		if planet != nil && planet.OwnedBy(p.Num) && planet.Spec.HasStarbase && planet.Spec.DockCapacity != 0 {
			fuel = f.Spec.FuelCapacity
		}
	}
	return true
}
