package cs

import (
	"encoding/json"
	"maps"
	"math"
	"sort"
)

const passengerRotationTag = "passenger-rotation"

type passengerRotation struct {
	Trips           int `json:"trips"`
	LastDestination int `json:"lastDestination"`
}

func readPassengerRotation(fleet *Fleet) passengerRotation {
	var rotation passengerRotation
	_ = json.Unmarshal([]byte(fleet.GetTag(passengerRotationTag)), &rotation)
	return rotation
}

// Population limits are in people; Cargo.Colonists is in units of 100 people.
func populationTarget(planet *Planet, percent int) int {
	return max(0, planet.Spec.MaxPopulation*percent/100/100)
}

func (t *turnGenerator) passengerReservations(player *Player, exclude int) (incoming, outgoing map[int]int) {
	incoming, outgoing = map[int]int{}, map[int]int{}
	for _, fleet := range t.game.getFleets(player.Num) {
		if fleet.Delete || fleet.Num == exclude || player.Automation.FleetRoles[fleet.Num] != "freighter" {
			continue
		}
		job := logisticsWork(fleet)
		if job == nil {
			continue
		}
		if job.Kind == "invade-load" {
			outgoing[job.TargetNum] += job.Passengers
		}
		if job.Kind == "passengers" {
			incoming[job.DeliveryNum] += job.Passengers
			outgoing[job.TargetNum] += job.Passengers
		}
		if job.Kind == "settle" {
			incoming[job.DeliveryNum] += fleet.Cargo.Colonists
		}
	}
	return incoming, outgoing
}

func suitableColony(player *Player, planet *Planet) bool {
	return planet != nil && planet.OwnedBy(player.Num) && planet.Spec.MaxPopulation > 0 && (planet.Spec.Habitability > 0 || player.Race.Spec.InnateResources || (player.Race.Spec.Instaforming || player.Automation.ServeTerraformingWorlds) && prospectivePlanetQuality(player, planet) > 0)
}

func (t *turnGenerator) passengerDestination(player *Player, fleet *Fleet, preferred int) *Planet {
	incoming, _ := t.passengerReservations(player, fleet.Num)
	var best *Planet
	distance := math.Inf(1)
	for _, planet := range t.game.Planets {
		if !suitableColony(player, planet) || populationTarget(planet, player.Automation.destinationPopulationPercent())-planet.Cargo.Colonists-incoming[planet.Num] <= 0 {
			continue
		}
		if planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, planet.Position, planet.MapObject.ToTarget(), automaticRoutePolicy{MinWarp: automaticCruiseWarp(player, fleet)}) == nil {
			continue
		}
		if planet.Num == preferred {
			return planet
		}
		d := passengerScore(planet, fleet.Position.DistanceTo(planet.Position))
		if fleet.Cargo.Colonists > 0 && fleet.Cargo.Colonists*4 < fleet.Spec.CargoCapacity {
			d = fleet.Position.DistanceTo(planet.Position)
		}
		if d < distance {
			best, distance = planet, d
		}
	}
	return best
}

func (t *turnGenerator) findPassengerTransfer(player *Player, fleet *Fleet) *logisticsJob {
	if fleet.Cargo.Colonists > 0 {
		if dest := t.passengerDestination(player, fleet, 0); dest != nil {
			return &logisticsJob{Kind: "settle", TargetNum: dest.Num, DeliveryNum: dest.Num, Position: dest.Position}
		}
		return nil
	}
	incoming, outgoing := t.passengerReservations(player, fleet.Num)
	type candidate struct {
		job   *logisticsJob
		score float64
	}
	candidates := []candidate{}
	owned := t.game.getPlanets(player.Num)
	for _, source := range owned {
		if source.Spec.MaxPopulation <= 0 {
			continue
		}
		surplus := source.Cargo.Colonists - populationTarget(source, player.Automation.sourcePopulationPercent()) - outgoing[source.Num]
		if surplus <= 0 {
			continue
		}
		for _, dest := range owned {
			if source.Num == dest.Num || !suitableColony(player, dest) {
				continue
			}
			deficit := populationTarget(dest, player.Automation.destinationPopulationPercent()) - dest.Cargo.Colonists - incoming[dest.Num]
			load := min(surplus, deficit, fleet.availableColonistSpace())
			if load <= 0 {
				continue
			}
			// Do not send a large freighter on a long trip for a token load.
			// Already loaded ships always complete delivery; short local runs remain useful.
			if load*2 < fleet.Spec.CargoCapacity {
				needed := t.productionMinerals(player, dest, false)
				protected := t.productionMinerals(player, source, false)
				extra := 0
				for _, kind := range []CargoType{Germanium, Ironium, Boranium} {
					extra += min(max(0, needed.GetAmount(kind)-dest.Cargo.GetAmount(kind)), max(0, source.Cargo.GetAmount(kind)-protected.GetAmount(kind)))
				}
				if load+extra < fleet.Spec.CargoCapacity/2 && source.Position.DistanceTo(dest.Position) > fleet.StrategicSpeed(automaticMaxWarp(player, fleet)) {
					continue
				}
			}
			job := &logisticsJob{Kind: "passengers", TargetNum: source.Num, DeliveryNum: dest.Num, Position: source.Position, Passengers: load}
			candidates = append(candidates, candidate{job, passengerScore(dest, fleet.Position.DistanceTo(source.Position)+source.Position.DistanceTo(dest.Position))})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.score != b.score {
			return a.score < b.score
		}
		if a.job.DeliveryNum != b.job.DeliveryNum {
			return a.job.DeliveryNum < b.job.DeliveryNum
		}
		return a.job.TargetNum < b.job.TargetNum
	})
	rotation := readPassengerRotation(fleet)
	var chosen *logisticsJob
	best := 0.0
	alternatives := []*logisticsJob{}
	seen := map[int]bool{}
	for _, candidate := range candidates[:min(64, len(candidates))] {
		if seen[candidate.job.DeliveryNum] || !t.passengerTripSafe(player, fleet, candidate.job) {
			continue
		}
		seen[candidate.job.DeliveryNum] = true
		if chosen == nil {
			chosen = candidate.job
			best = candidate.score
		}
		if candidate.job.DeliveryNum != rotation.LastDestination && candidate.score <= max(100.0, best*2) {
			alternatives = append(alternatives, candidate.job)
		}
		if len(alternatives) >= 4 {
			break
		}
	}
	if chosen != nil && (rotation.Trips+1)%3 == 0 && len(alternatives) > 0 {
		chosen = alternatives[(fleet.Num+rotation.Trips/3)%len(alternatives)]
	}
	return chosen
}

func (t *turnGenerator) loadAutomaticPassengers(player *Player, fleet *Fleet, job *logisticsJob) {
	source, dest := t.game.getPlanet(job.TargetNum), t.game.getPlanet(job.DeliveryNum)
	if source == nil || !source.OwnedBy(player.Num) || !suitableColony(player, dest) {
		delete(fleet.Tags, logisticsTag)
		return
	}
	incoming, _ := t.passengerReservations(player, fleet.Num)
	available := max(0, source.Cargo.Colonists-populationTarget(source, player.Automation.sourcePopulationPercent()))
	room := max(0, populationTarget(dest, player.Automation.destinationPopulationPercent())-dest.Cargo.Colonists-incoming[dest.Num])
	initialLoad := max(0, min(available, room, job.Passengers, fleet.availableColonistSpace()))
	if initialLoad > 0 && t.passengerTripSafe(player, fleet, &logisticsJob{TargetNum: source.Num, DeliveryNum: dest.Num, Passengers: initialLoad}) {
		t.loadOpportunisticInfrastructure(player, fleet, source, dest, initialLoad)
	}
	load := max(0, min(available, room, job.Passengers, fleet.availableColonistSpace()))
	if load == 0 || !t.passengerTripSafe(player, fleet, &logisticsJob{TargetNum: source.Num, DeliveryNum: dest.Num, Passengers: load}) {
		delete(fleet.Tags, logisticsTag)
		return
	}
	source.Cargo.Colonists -= load
	fleet.Cargo.Colonists += load
	t.topUpLogisticsCargo(player, fleet, source, dest)
	rotation := readPassengerRotation(fleet)
	rotation.Trips++
	rotation.LastDestination = dest.Num
	raw, _ := json.Marshal(rotation)
	fleet.SetTag(passengerRotationTag, string(raw))
	job.Kind, job.TargetNum, job.Position, job.Status = "settle", dest.Num, dest.Position, "Transporting passengers to "+dest.Name
	saveLogisticsWork(fleet, job)
}

func (t *turnGenerator) unloadAutomaticPassengers(player *Player, fleet *Fleet, job *logisticsJob) {
	dest := t.game.getPlanet(job.TargetNum)
	if !suitableColony(player, dest) {
		return
	}
	// Once a delivery is committed, empty the hold up to peak-growth density
	// rather than shuttling a tiny remainder after another convoy arrived first.
	room := max(0, populationTarget(dest, max(35, player.Automation.destinationPopulationPercent()))-dest.Cargo.Colonists)
	amount := min(room, fleet.Cargo.Colonists)
	dest.Cargo = dest.Cargo.AddMineral(fleet.Cargo.ToMineral())
	fleet.Cargo.Ironium = 0
	fleet.Cargo.Boranium = 0
	fleet.Cargo.Germanium = 0
	dest.Cargo.Colonists += amount
	fleet.Cargo.Colonists -= amount
	if fleet.Cargo.Colonists == 0 {
		delete(fleet.Tags, logisticsTag)
		t.releaseEscorts(player, fleet)
	} else {
		job.DeliveryNum = 0
		job.Status = "Looking for another colony with room for remaining passengers"
		saveLogisticsWork(fleet, job)
	}
}

func passengerScore(planet *Planet, distance float64) float64 {
	quality := max(0, planet.Spec.Habitability)
	return (1 + distance) / (0.25 + math.Pow(float64(quality)/100, 2))
}
func (t *turnGenerator) passengerTripSafe(player *Player, fleet *Fleet, job *logisticsJob) bool {
	source, dest := t.game.getPlanet(job.TargetNum), t.game.getPlanet(job.DeliveryNum)
	if source == nil || dest == nil {
		return false
	}
	budget := *fleet
	budget.Tags = maps.Clone(fleet.Tags)
	saveLogisticsWork(&budget, &logisticsJob{Kind: "passengers"})
	first := planAutomaticRoute(&t.game.Rules, player, []*Fleet{&budget}, source.Position, source.MapObject.ToTarget(), automaticRoutePolicy{MinWarp: automaticCruiseWarp(player, fleet), MaxTurns: 20})
	if first == nil {
		return false
	}
	virtual := budget
	virtual.Position = source.Position
	virtual.Fuel = first.ArrivalFuel[0]
	virtual.Cargo.Colonists += job.Passengers
	if source.Spec.HasStarbase && source.Spec.DockCapacity != 0 {
		virtual.Fuel = fleet.Spec.FuelCapacity
	}
	return planAutomaticRoute(&t.game.Rules, player, []*Fleet{&virtual}, dest.Position, dest.MapObject.ToTarget(), automaticRoutePolicy{MinWarp: automaticCruiseWarp(player, fleet), MaxTurns: 20}) != nil
}

func passengerCruiseFleet(fleet *Fleet) bool {
	job := logisticsWork(fleet)
	return job != nil && (job.Kind == "passengers" || job.Kind == "settle" || job.Kind == "invade" || job.Kind == "invade-load")
}
