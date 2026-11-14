package cs

import "slices"

func automaticBomberAvailable(p *Player, f *Fleet) bool {
	return p.Automation.FleetRoles[f.Num] == "bomber" && !f.Delete && !f.InDesignMaintenance() &&
		!fuelNetworkManaged(f) && !f.InAutomaticSafety() && f.GetTag(tankerAwaitTag) == ""
}

func bomberShipCount(f *Fleet) int {
	ships := 0
	for _, token := range f.Tokens {
		if token.design != nil && token.design.Spec.Bomber {
			ships += token.Quantity
		}
	}
	return ships
}

func bomberReserveTarget(p *Player, planet *Planet) bool {
	return planet != nil && p.Automation.PriorityPlanets[planet.Num] && planet.Explored() &&
		planet.Owned() && planet.PlayerNum != p.Num && !p.IsFriend(planet.PlayerNum)
}

func bomberReserveReleased(p *Player, orders bomberOrders) *Planet {
	target := p.GetPlanetIntel(orders.ReserveTarget)
	if bomberReserveTarget(p, target) && !target.Spec.HasStarbase {
		return target
	}
	return nil
}

// Hold ready bombers at their current docks. Cover distinct priority orbits
// before allocating more bombers to any one orbit.
func bomberReserve(p *Player, fleets []*Fleet) map[int]*Planet {
	var targets []*Planet
	docks := map[Vector]*Planet{}
	for _, planet := range p.PlanetIntels {
		if planet == nil {
			continue
		}
		if bomberReserveTarget(p, planet) && planet.Spec.HasStarbase {
			targets = append(targets, planet)
		}
		if planet.OwnedBy(p.Num) && planet.Spec.HasStarbase {
			docks[planet.Position] = planet
		}
	}
	if len(targets) == 0 {
		return nil
	}

	type candidate struct {
		fleet, ships int
		target       *Planet
		distance     float64
	}
	var candidates []candidate
	total := 0
	for _, f := range fleets {
		if !automaticBomberAvailable(p, f) || f.GetTag("maintenance-pending") != "" {
			continue
		}
		ships := bomberShipCount(f)
		total += ships
		dock := docks[f.Position]
		if ships == 0 || dock == nil || !f.CanFuel(p, dock) || f.Fuel < f.Spec.FuelCapacity ||
			f.GetTag(bombingRallyTag) != "" || p.FleetInActiveAssembly(f) ||
			knownRouteThreat(p, f.Position, f.Position) > 0 {
			continue
		}
		if bomberReserveReleased(p, readBomberOrders(f)) != nil {
			continue
		}
		for _, target := range targets {
			candidates = append(candidates, candidate{f.Num, ships, target, f.Position.DistanceTo(target.Position)})
		}
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		if a.distance < b.distance {
			return -1
		}
		if a.distance > b.distance {
			return 1
		}
		if a.fleet != b.fleet {
			return a.fleet - b.fleet
		}
		return a.target.Num - b.target.Num
	})

	remaining := (3*total + 5) / 10
	assigned := map[int]*Planet{}
	covered := map[int]bool{}
	for _, distinct := range []bool{true, false} {
		for _, c := range candidates {
			if c.ships > remaining || assigned[c.fleet] != nil || distinct && covered[c.target.Num] {
				continue
			}
			assigned[c.fleet] = c.target
			covered[c.target.Num] = true
			remaining -= c.ships
		}
	}
	return assigned
}
