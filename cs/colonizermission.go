package cs

import (
	"fmt"
	"math"
)

const colonizationMaxYears = 25

// A reservation belongs to an actual voyage, not to a stale status tag.
func colonizerTravelling(p *Player, f *Fleet, job *colonizationJob) bool {
	if job == nil || job.Kind != "colonize" || f.Delete || f.Cargo.Colonists == 0 || f.InAutomaticSafety() || f.InDesignMaintenance() || landingManaged(f) {
		return false
	}
	for _, wp := range f.Waypoints {
		if wp.TargetType == MapObjectTypePlanet && wp.TargetNum == job.TargetNum && wp.Task == WaypointTaskColonize {
			return true
		}
	}
	return false
}

func colonizerHasTask(p *Player, f *Fleet) bool {
	if colonizationMode(p.Automation.FleetRoles[f.Num]) == "" {
		return false
	}
	j := readColonizationJob(f)
	return landingManaged(f) || f.Cargo.Colonists > 0 || j != nil && (j.Kind == "colonize" && j.TargetNum != 0 || j.Kind == "retire" || j.Kind == "deliver-retire")
}

// Judge reuse by the funded journey home, including real fixed docks, rather
// than by an engine name. Probe empty because the current delivery finishes first.
func (t *turnGenerator) colonizerReturn(p *Player, f *Fleet, maxYears int) (*Planet, *expeditionRoute) {
	probe := *f
	probe.Cargo = Cargo{}
	var dock *Planet
	var route *expeditionRoute
	for _, planet := range t.game.getPlanets(p.Num) {
		if !planet.Spec.HasStarbase {
			continue
		}
		r := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&probe}, planet.Position, planet.ToTarget(), automaticRoutePolicy{OneWay: true, EmergencyReturn: true, MaxTurns: maxYears})
		if r != nil && (route == nil || r.Turns < route.Turns || r.Turns == route.Turns && planet.Num < dock.Num) {
			dock, route = planet, r
		}
	}
	return dock, route
}

func (t *turnGenerator) completedColonizerTask(p *Player, f *Fleet, job *colonizationJob) {
	job.TargetNum, job.TravelYears, job.Quality, job.Score = 0, 0, 0, 0
	_, route := t.colonizerReturn(p, f, colonizationMaxYears)
	job.Kind = "passengers"
	job.Status = "Delivery completed; available for colonist transport, landings or a new colony"
	if route == nil {
		job.Kind = "retire"
		job.Status = "Delivery completed; long return, heading to the nearest reachable starbase for recycling"
	}
	delete(f.Tags, logisticsTag)
	delete(f.Tags, "maintenance-pending")
	writeColonizationJob(f, job)
}

func (t *turnGenerator) retireColonizer(p *Player, f *Fleet, job *colonizationJob) {
	dock, route := t.colonizerReturn(p, f, 10000)
	if dock == nil {
		job.Status = "Recycling return needs fuel; requesting nearby support"
		holdAutomaticFleet(p, f, job.Status)
		requestAutomaticRescue(p, f, true)
	} else {
		applyExpeditionRoute(p, f, route)
		f.Waypoints[len(f.Waypoints)-1].Task = WaypointTaskScrapFleet
		job.Status = fmt.Sprintf("Returning to %s for recycling; ETA %d years; nearby patrol fuel welcome", dock.Name, route.Turns)
		job.TravelYears = route.Turns
		if route.Turns > colonizationMaxYears {
			requestAutomaticRescue(p, f, true)
		}
	}
	writeColonizationJob(f, job)
}

// A stranded old ship first puts its passengers on an owned world. It must
// not wait forever for a profitable normal transport job with its hold full.
func (t *turnGenerator) deliverRetiringColonizer(p *Player, f *Fleet, job *colonizationJob) {
	var destination *Planet
	var route *expeditionRoute
	for _, planet := range t.game.getPlanets(p.Num) {
		r := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, planet.Position, planet.ToTarget(), automaticRoutePolicy{OneWay: true, EmergencyReturn: true, MaxTurns: 10000})
		if r != nil && (route == nil || r.Turns < route.Turns || r.Turns == route.Turns && planet.Num < destination.Num) {
			destination, route = planet, r
		}
	}
	if destination == nil {
		job.Status = "Passengers retained; no funded delivery route, requesting nearby fuel support"
		holdAutomaticFleet(p, f, job.Status)
		requestAutomaticRescue(p, f, true)
	} else if f.Position == destination.Position {
		destination.Cargo.Colonists += f.Cargo.Colonists
		f.Cargo.Colonists = 0
		job.Kind, job.TargetNum = "retire", 0
		t.retireColonizer(p, f, job)
		return
	} else {
		job.TargetNum = destination.Num
		job.TravelYears = route.Turns
		applyExpeditionRoute(p, f, route)
		job.Status = fmt.Sprintf("Delivering remaining colonists to %s before recycling; ETA %d years", destination.Name, route.Turns)
		if route.Turns > colonizationMaxYears {
			requestAutomaticRescue(p, f, true)
		}
	}
	writeColonizationJob(f, job)
}

// Arrival is physical evidence. An unexpected hostile owner changes the task
// into a normal ground drop without consuming the colony hull. Starbases and
// friendly owners retain the normal transport protections.
func (t *turnGenerator) deliverArrivedColonizer(p *Player, f *Fleet, planet *Planet, job *colonizationJob) bool {
	if planet == nil || f.Position != planet.Position || !planet.Owned() || f.Cargo.Colonists == 0 {
		return false
	}
	if planet.OwnedBy(p.Num) {
		planet.Cargo.Colonists += f.Cargo.Colonists
	} else {
		if p.IsFriend(planet.PlayerNum) || planet.Spec.HasStarbase || p.Race.Spec.LivesOnStarbases {
			job.Kind, job.TargetNum = "passengers", 0
			job.Status = "Landing unavailable at occupied destination; retaining colonists for another delivery"
			writeColonizationJob(f, job)
			f.Waypoints = []Waypoint{{Position: f.Position}}
			return true
		}
		drops := newInvader()
		drops.addInvasion(invasion{planet: planet, attacker: p, defender: t.game.getPlayer(planet.PlayerNum), attackers: f.Cargo.Colonists * 100, fleets: []*Fleet{f}})
		t.queueColonistDrops(drops)
	}
	f.Cargo.Colonists = 0
	f.Waypoints = []Waypoint{{Position: f.Position}}
	t.completedColonizerTask(p, f, job)
	return true
}

func landingTransport(p *Player, f *Fleet) bool {
	return p.Automation.FleetRoles[f.Num] == "freighter" || colonizerPassengerCarrier(p, f)
}

func (t *turnGenerator) continueColonizerMission(p *Player, f *Fleet, job *colonizationJob, target *Planet) bool {
	if fundedColonizerWaypoints(&t.game.Rules, p, f) {
		if f.GetTag(tankerAwaitTag) == "" && !fuelNetworkManaged(f) {
			delete(f.Tags, fuelStatusTag)
			delete(f.Tags, fuelRequestTag)
		}
		job.TravelYears = 0
		from := f.Position
		for _, wp := range f.Waypoints[1:] {
			job.TravelYears += max(1, int(math.Ceil(from.DistanceTo(wp.Position)/f.StrategicSpeed(wp.WarpSpeed))))
			from = wp.Position
		}
		job.Status = fmt.Sprintf("Following committed route to %s; ETA %d years; maintenance deferred", target.Name, job.TravelYears)
		job.Quality = colonyTargetQuality(p, target)
		job.Score = colonyTargetScore(job.Quality, job.TravelYears)
		writeColonizationJob(f, job)
		return true
	}
	route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, target.Position, target.ToTarget(), automaticRoutePolicy{OneWay: true, MaxTurns: colonizationMaxYears})
	if route == nil {
		// A spent engine is allowed to finish slowly; never recall it merely
		// because its originally funded voyage now exceeds the new-job limit.
		route = planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, target.Position, target.ToTarget(), automaticRoutePolicy{OneWay: true, EmergencyReturn: true, MaxTurns: 10000})
	}
	job.Quality = colonyTargetQuality(p, target)
	if route == nil {
		job.TravelYears = 0
		job.Status = "Original colony target retained; route blocked, requesting fuel support without reserving the planet"
		holdAutomaticFleet(p, f, job.Status)
		saveAutomaticFuelRequest(f, automaticFuelRequest{Destination: target.Position, Target: target.ToTarget(), Members: []int{f.Num}, OneWay: true, Urgent: true})
	} else {
		applyExpeditionRoute(p, f, route)
		if !route.WaitForFuel {
			f.Waypoints[len(f.Waypoints)-1].Task = WaypointTaskColonize
		}
		job.TravelYears, job.Score = route.Turns, colonyTargetScore(job.Quality, route.Turns)
		job.Status = fmt.Sprintf("Continuing original colony mission to %s; habitability %d%%, ETA %d years; maintenance deferred", target.Name, job.Quality, route.Turns)
		if route.WaitForFuel {
			job.Status = "Refuelling at a fixed base before continuing to " + target.Name
		}
	}
	writeColonizationJob(f, job)
	return route != nil
}

// Validate the whole funded itinerary before retaining it. This avoids annual
// dock-to-dock oscillation while still reacting to fuel loss and fresh threats.
func fundedColonizerWaypoints(rules *Rules, p *Player, f *Fleet) bool {
	if len(f.Waypoints) < 2 || f.InAutomaticSafety() || f.InDesignMaintenance() || landingManaged(f) {
		return false
	}
	last := f.Waypoints[len(f.Waypoints)-1]
	if !colonizationOneWay(p, f, last) {
		return false
	}
	fuel, from := f.Fuel, f.Position
	for _, wp := range f.Waypoints[1:] {
		if wp.WarpSpeed == StargateWarpSpeed {
			return false
		} // use the normal gate planner/guard
		if wp.WarpSpeed < 1 || wp.WarpSpeed > automaticMaxWarp(p, f) || !automaticFleetLegSafe(rules, p, f, from, wp.Position, wp.WarpSpeed, false) {
			return false
		}
		distance := from.DistanceTo(wp.Position)
		_, left, _, ok := automaticLegFuel(p, f, wp.WarpSpeed, distance, fuel, 0)
		if !ok {
			return false
		}
		fuel, from = left, wp.Position
		if planet := p.GetPlanetIntel(wp.TargetNum); planet != nil && planet.Position == wp.Position && planet.OwnedBy(p.Num) && f.CanFuel(p, planet) {
			fuel = f.Spec.FuelCapacity
		}
	}
	return true
}
