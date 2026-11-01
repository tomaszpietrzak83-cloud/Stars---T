package cs

import (
	"encoding/json"
	"fmt"
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
		if fleet.Delete || fleet.Num == exclude || !passengerCarrier(player, fleet) {
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
		if planet.Num == preferred && (incoming[planet.Num] == 0 || fleet.Position.DistanceTo(planet.Position) <= fleet.StrategicSpeed(automaticMaxWarp(player, fleet))) {
			return planet
		}
		d := passengerScore(planet, fleet.Position.DistanceTo(planet.Position))
		if fleet.Cargo.Colonists > 0 && fleet.Cargo.Colonists*4 < fleet.Spec.CargoCapacity {
			d = fleet.Position.DistanceTo(planet.Position)
		}
		if incoming[planet.Num] > 0 {
			d += 1e9
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
		job     *logisticsJob
		score   float64
		crowded bool
		quality int
	}
	candidates := []candidate{}
	owned := t.game.getPlanets(player.Num)
	for _, source := range owned {
		if source.Spec.MaxPopulation <= 0 {
			continue
		}
		surplus := source.Cargo.Colonists - sourcePopulationReserve(player, source) - outgoing[source.Num]
		if surplus <= 0 {
			continue
		}
		for _, dest := range owned {
			if source.Num == dest.Num || !suitableColony(player, dest) {
				continue
			}
			deficit := populationTarget(dest, player.Automation.destinationPopulationPercent()) - dest.Cargo.Colonists - incoming[dest.Num]
			if !t.destinationCanFillTransport(player, fleet, source, dest, deficit) {
				continue
			}
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
			candidates = append(candidates, candidate{job, passengerScore(dest, fleet.Position.DistanceTo(source.Position)+source.Position.DistanceTo(dest.Position)), incoming[dest.Num] > 0, prospectivePlanetQuality(player, dest)})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		ap, bp := player.Automation.PriorityPlanets[a.job.DeliveryNum], player.Automation.PriorityPlanets[b.job.DeliveryNum]
		if ap != bp {
			return ap
		}
		if a.crowded != b.crowded {
			return !a.crowded
		}
		af, bf := a.job.Passengers >= fleet.availableColonistSpace(), b.job.Passengers >= fleet.availableColonistSpace()
		if af != bf {
			return af
		}
		if a.quality != b.quality {
			return a.quality > b.quality
		}
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
	attempts := map[int]int{}
	destinations := 0
	for _, candidate := range candidates {
		num := candidate.job.DeliveryNum
		if attempts[num] >= 4 {
			continue
		}
		if attempts[num] == 0 {
			if destinations >= 64 {
				continue
			}
			destinations++
		}
		attempts[num]++
		if seen[candidate.job.DeliveryNum] || !t.passengerTripSafe(player, fleet, candidate.job) {
			continue
		}
		seen[candidate.job.DeliveryNum] = true
		if chosen == nil {
			chosen = candidate.job
			best = candidate.score
		}
		if player.Automation.PriorityPlanets[candidate.job.DeliveryNum] != player.Automation.PriorityPlanets[chosen.DeliveryNum] {
			continue
		}
		if incoming[candidate.job.DeliveryNum] > 0 && incoming[chosen.DeliveryNum] == 0 {
			continue
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
	available := max(0, source.Cargo.Colonists-sourcePopulationReserve(player, source))
	room := max(0, populationTarget(dest, player.Automation.destinationPopulationPercent())-dest.Cargo.Colonists-incoming[dest.Num])
	probe := *fleet
	probe.Tags = maps.Clone(fleet.Tags)
	origin := *source
	initialLoad := max(0, min(available, room, probe.availableColonistSpace()))
	minimumPassengers := t.preparePassengerMixedCargo(player, &probe, &origin, dest, initialLoad)
	load := max(0, min(available, room, probe.availableColonistSpace()))
	origin.Cargo.Colonists -= load
	probe.Cargo.Colonists += load
	t.topUpLogisticsCargo(player, &probe, &origin, dest)
	loadedRoute := planAutomaticRoute(&t.game.Rules, player, []*Fleet{&probe}, dest.Position, dest.ToTarget(), automaticRoutePolicy{AllowThreat: true, MinWarp: automaticCruiseWarp(player, fleet)})
	safe := t.deliveryEscortReady(player, fleet, dest.Position) && load > 0 && load >= minimumPassengers && loadedRoute != nil && !loadedRoute.WaitForFuel
	if probe.CargoMass() != fleet.Spec.CargoCapacity || !safe {
		job.Passengers = initialLoad
		job.Status = fmt.Sprintf("Waiting empty at source for a full safe load (%d/%d kT available)", probe.CargoMass(), fleet.Spec.CargoCapacity)
		if probe.CargoMass() == fleet.Spec.CargoCapacity && loadedRoute != nil && !loadedRoute.WaitForFuel && !t.deliveryEscortReady(player, fleet, dest.Position) {
			job.Status = "Waiting empty for escort before loading: armed enemy on the delivery route"
		}
		holdAutomaticFleet(player, fleet, job.Status)
		saveLogisticsWork(fleet, job)
		return
	}
	source.Cargo = origin.Cargo
	source.Factories = origin.Factories
	source.Mines = origin.Mines
	source.Defenses = origin.Defenses
	fleet.Cargo = probe.Cargo
	saveInfrastructure(fleet, readInfrastructure(&probe))

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
	saveLogisticsWork(&budget, &logisticsJob{Kind: "passengers", TargetNum: source.Num, DeliveryNum: dest.Num, Position: dest.Position, Passengers: job.Passengers})
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

// For d>1/4 the native growth law is k*d*(1-d)^2. Its maximum
// is d=1/3. Round the reserve up to a whole 100-person cargo unit.
func sourcePopulationReserve(p *Player, planet *Planet) int {
	if p.Automation.ManualPassengerThresholds {
		return populationTarget(planet, p.Automation.sourcePopulationPercent())
	}
	return (planet.Spec.MaxPopulation + 299) / 300
}

// Waiting for a source to grow is useful. Waiting because the destination can
// never accept a full useful load is not: choose another colony immediately.
func (t *turnGenerator) destinationCanFillTransport(p *Player, f *Fleet, source, dest *Planet, room int) bool {
	if room >= f.availableColonistSpace() {
		return true
	}
	probe, origin := *f, *source
	probe.Tags = maps.Clone(f.Tags)
	probe.Position = source.Position
	minimum := t.preparePassengerMixedCargo(p, &probe, &origin, dest, min(room, probe.availableColonistSpace()))
	if room < minimum {
		return false
	}
	probe.Cargo.Colonists += max(0, min(room, probe.availableColonistSpace()))
	t.topUpLogisticsCargo(p, &probe, &origin, dest)
	return probe.CargoMass() == probe.Spec.CargoCapacity
}

func (t *turnGenerator) preparePassengerMixedCargo(p *Player, f *Fleet, source, dest *Planet, passengers int) int {
	if colonizationMode(p.Automation.FleetRoles[f.Num]) != "" {
		return 0
	}
	minimum := 0
	placeholder := 0
	if t.prefetchStarbaseMinerals(p, dest).Total() > 0 {
		minimum = f.Spec.CargoCapacity - f.Spec.CargoCapacity/5
		placeholder = min(minimum, f.availableColonistSpace())
		f.Cargo.Colonists += placeholder
	}
	// Surplus installations and advance mineral stock share the non-passenger
	// allowance, so construction deliveries cannot displace the 80% passenger share.
	t.loadOpportunisticInfrastructure(p, f, source, dest, passengers)
	t.loadStarbasePrefetch(p, f, source, dest)
	f.Cargo.Colonists -= placeholder
	return minimum
}

// Colonizers retain their colonization role while doing passenger-only work.
func passengerCarrier(p *Player, f *Fleet) bool {
	if p.Automation.FleetRoles[f.Num] == "patrol" && f.GetTag(patrolSalvageTag) != "" {
		return true
	}
	if p.Automation.FleetRoles[f.Num] == "freighter" {
		return true
	}
	job := readColonizationJob(f)
	return colonizationMode(p.Automation.FleetRoles[f.Num]) != "" && job != nil && job.Kind == "passengers"
}
func colonizerPassengerCarrier(p *Player, f *Fleet) bool {
	return passengerCarrier(p, f) && colonizationMode(p.Automation.FleetRoles[f.Num]) != ""
}

// A source-growth wait must not reserve an empty transport forever while a
// different source can fill it now. This preview uses only real stock and the
// same mixed-load rules as physical loading; no cargo or planet is changed.
func (t *turnGenerator) passengerPickupReady(p *Player, f *Fleet, job *logisticsJob) bool {
	source, dest := t.game.getPlanet(job.TargetNum), t.game.getPlanet(job.DeliveryNum)
	if source == nil || dest == nil {
		return false
	}
	probe, origin := *f, *source
	probe.Tags = maps.Clone(f.Tags)
	incoming, outgoing := t.passengerReservations(p, f.Num)
	available := max(0, source.Cargo.Colonists-sourcePopulationReserve(p, source)-outgoing[source.Num])
	room := max(0, populationTarget(dest, p.Automation.destinationPopulationPercent())-dest.Cargo.Colonists-incoming[dest.Num])
	minimum := t.preparePassengerMixedCargo(p, &probe, &origin, dest, min(available, room, probe.availableColonistSpace()))
	load := max(0, min(available, room, probe.availableColonistSpace()))
	probe.Cargo.Colonists += load
	origin.Cargo.Colonists -= load
	t.topUpLogisticsCargo(p, &probe, &origin, dest)
	return load > 0 && load >= minimum && probe.CargoMass() == f.Spec.CargoCapacity
}
