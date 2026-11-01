package cs

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
)

type LogisticsAutomation struct {
	PriorityTraders              map[int]bool          `json:"priorityTraders,omitempty"`
	PriorityPlanets              map[int]bool          `json:"priorityPlanets,omitempty"`
	ServeTerraformingWorlds      bool                  `json:"serveTerraformingWorlds,omitempty"`
	InfrastructureTransport      bool                  `json:"infrastructureTransport,omitempty"`
	ManualPassengerThresholds    bool                  `json:"manualPassengerThresholds,omitempty"`
	DisablePassengerTransport    bool                  `json:"disablePassengerTransport,omitempty"`
	DesignFamilies               map[int]int           `json:"designFamilies,omitempty"`
	ScoutMaxReturnTurns          int                   `json:"scoutMaxReturnTurns,omitempty"`
	ResearchTax                  *ResearchTaxPolicy    `json:"researchTax,omitempty"`
	RepairThresholdPercent       int                   `json:"repairThresholdPercent,omitempty"`
	DisableAutomaticRepair       bool                  `json:"disableAutomaticRepair,omitempty"`
	DisableInterceptionEvasion   bool                  `json:"disableInterceptionEvasion,omitempty"`
	AutomaticPackets             bool                  `json:"automaticPackets,omitempty"`
	FleetAssemblies              map[int]FleetAssembly `json:"fleetAssemblies,omitempty"`
	PassengerTransport           bool                  `json:"passengerTransport,omitempty"`
	SourcePopulationPercent      int                   `json:"sourcePopulationPercent,omitempty"`
	DestinationPopulationPercent int                   `json:"destinationPopulationPercent,omitempty"`
	EscortMarginLY               int                   `json:"escortMarginLy,omitempty"`
	EscortStrengthPercent        int                   `json:"escortStrengthPercent,omitempty"`
	DisableSalvage               bool                  `json:"disableSalvage,omitempty"`
	DisableMineralSupply         bool                  `json:"disableMineralSupply,omitempty"`
	SingleShipyardRebuild        bool                  `json:"singleShipyardRebuild,omitempty"`
	FleetRoles                   map[int]string        `json:"fleetRoles,omitempty"`
	RebuildFleets                map[int]bool          `json:"rebuildFleets,omitempty"`
	BattleSalvage                bool                  `json:"battleSalvage,omitempty"`
	OffloadCombatCargo           bool                  `json:"offloadCombatCargo,omitempty"`
}

func (a LogisticsAutomation) sourcePopulationPercent() int {
	if !a.ManualPassengerThresholds {
		return 35
	}
	if a.SourcePopulationPercent == 0 {
		if a.PassengerTransport {
			return 90
		}
		return 35
	}
	return a.SourcePopulationPercent
}
func (a LogisticsAutomation) destinationPopulationPercent() int {
	if !a.ManualPassengerThresholds {
		return 20
	}
	if a.DestinationPopulationPercent == 0 {
		if a.PassengerTransport {
			return 60
		}
		return 25
	}
	return a.DestinationPopulationPercent
}
func (a LogisticsAutomation) escortMargin() int {
	if a.EscortMarginLY == 0 {
		return 100
	}
	return a.EscortMarginLY
}
func (a LogisticsAutomation) escortStrength() int {
	if a.EscortStrengthPercent == 0 {
		return 100
	}
	return a.EscortStrengthPercent
}
func (a LogisticsAutomation) repairThreshold() int {
	if a.RepairThresholdPercent == 0 {
		return 30
	}
	return a.RepairThresholdPercent
}
func (a LogisticsAutomation) scoutReturnTurns() int {
	if a.ScoutMaxReturnTurns == 0 {
		return 20
	}
	return a.ScoutMaxReturnTurns
}
func (a LogisticsAutomation) Validate() error {
	if a.SourcePopulationPercent < 0 || a.SourcePopulationPercent > 100 || a.DestinationPopulationPercent < 0 || a.DestinationPopulationPercent > 99 || a.SourcePopulationPercent > 0 && a.DestinationPopulationPercent >= a.SourcePopulationPercent {
		return fmt.Errorf("stored population thresholds must satisfy 1 <= destination < source <= 100")
	}
	if a.ScoutMaxReturnTurns < 0 || a.ScoutMaxReturnTurns > 200 {
		return fmt.Errorf("scout return limit must be 1-200 turns")
	}
	if a.ResearchTax != nil {
		if err := a.ResearchTax.Validate(); err != nil {
			return err
		}
	}
	if a.RepairThresholdPercent < 0 || a.RepairThresholdPercent > 100 {
		return fmt.Errorf("repair damage threshold must be 1-100 percent")
	}
	if a.sourcePopulationPercent() < 1 || a.sourcePopulationPercent() > 100 || a.destinationPopulationPercent() < 1 || a.destinationPopulationPercent() >= a.sourcePopulationPercent() {
		return fmt.Errorf("population thresholds must satisfy 1 <= destination < source <= 100")
	}
	if a.escortMargin() < 1 || a.escortMargin() > 1000 || a.escortStrength() < 50 || a.escortStrength() > 1000 {
		return fmt.Errorf("escort margin must be 1-1000 ly and strength 50-1000 percent")
	}
	return nil
}

const logisticsTag = "logistics-job"
const escortTag = "escort-for"

type logisticsJob struct {
	Passengers  int     `json:"passengers,omitempty"`
	DeliveryNum int     `json:"deliveryNum,omitempty"`
	Load        Mineral `json:"load,omitzero"`
	Kind        string  `json:"kind"`
	TargetNum   int     `json:"targetNum"`
	Position    Vector  `json:"position"`
	Status      string  `json:"status"`
}

func logisticsWork(fleet *Fleet) *logisticsJob {
	job := &logisticsJob{}
	if json.Unmarshal([]byte(fleet.GetTag(logisticsTag)), job) != nil {
		return nil
	}
	return job
}
func saveLogisticsWork(fleet *Fleet, job *logisticsJob) {
	data, _ := json.Marshal(job)
	fleet.SetTag(logisticsTag, string(data))
}

func (p *Player) ValidateLogisticsAutomation(automation LogisticsAutomation, fleets []*Fleet) error {
	if err := automation.Validate(); err != nil {
		return err
	}
	owned := map[int]*Fleet{}
	for _, fleet := range fleets {
		if fleet.PlayerNum == p.Num && !fleet.Starbase && !fleet.Delete {
			owned[fleet.Num] = fleet
		}
	}
	for num, role := range automation.FleetRoles {
		fleet := owned[num]
		if fleet != nil && p.FleetInActiveAssembly(fleet) {
			return fmt.Errorf("finish or cancel the fleet composition before assigning an automatic role")
		}
		if fleet == nil || role != "freighter" && role != "escort" && role != "scout" && role != "miner" && role != "tanker" && role != "terraformer" && role != "bomber" && role != "combat" && role != "patrol" && colonizationMode(role) == "" {
			return fmt.Errorf("invalid automatic fleet assignment")
		}
		if role == "freighter" && fleet.Spec.CargoCapacity <= 0 && p.Automation.FleetRoles[num] != role {
			return fmt.Errorf("automatic transport requires cargo capacity")
		}
		if role == "escort" && !fleet.Spec.HasWeapons && p.Automation.FleetRoles[num] != role {
			return fmt.Errorf("escort requires armed ships")
		}
		if role == "miner" && fleet.Spec.MiningRate <= 0 && p.Automation.FleetRoles[num] != role {
			return fmt.Errorf("automatic mining requires remote mining equipment")
		}
		if role == "bomber" && !fleet.Spec.Bomber && p.Automation.FleetRoles[num] != role {
			return fmt.Errorf("bomber automation requires bombs")
		}
		if (role == "combat" || role == "patrol") && !fleet.Spec.HasWeapons && p.Automation.FleetRoles[num] != role {
			return fmt.Errorf("combat automation requires weapons")
		}
		if role == "terraformer" && fleet.Spec.TerraformRate <= 0 && p.Automation.FleetRoles[num] != role {
			return fmt.Errorf("automatic terraforming requires orbital terraforming equipment")
		}
		if role == "tanker" && fleet.Spec.FuelCapacity <= 0 && p.Automation.FleetRoles[num] != role {
			return fmt.Errorf("automatic tankers require a fuel tank")
		}
		if colonizationMode(role) != "" && !p.CanAutomateColonizer(fleet) && p.Automation.FleetRoles[num] != role {
			return fmt.Errorf("automatic colonization requires a fleet containing only colony ships; split escorts and transports first")
		}
	}
	for num, requested := range automation.RebuildFleets {
		if requested && owned[num] != nil && p.FleetInActiveAssembly(owned[num]) {
			return fmt.Errorf("finish or cancel the fleet composition before rebuilding it")
		}
		if requested && owned[num] == nil {
			return fmt.Errorf("cannot rebuild a fleet you do not own")
		}
		if requested && !hasShipsToReplace(p, owned[num]) {
			return fmt.Errorf("choose a replacement design before ordering a fleet rebuild")
		}
	}
	return nil
}

func takeMinerals(fleet *Fleet, cargo *Cargo) int {
	room := max(0, fleet.Spec.CargoCapacity-fleet.CargoMass())
	loaded := 0
	for _, kind := range []ResourceType{Ironium, Boranium, Germanium} {
		amount := min(room, cargo.GetAmount(kind))
		*cargo = cargo.SetAmount(kind, cargo.GetAmount(kind)-amount)
		fleet.Cargo = fleet.Cargo.SetAmount(kind, fleet.Cargo.GetAmount(kind)+amount)
		room -= amount
		loaded += amount
	}
	return loaded
}

// All long-range threat information comes from the player's own intel.
// The configured corridor is capped by a reported enemy's one-turn reach;
// distant ships must not freeze otherwise useful civilian routes.
func knownRouteThreat(player *Player, from, to Vector) int {
	if index := player.routeThreatIndex; index != nil {
		if !index.built || index.margin != player.Automation.escortMargin() {
			index.built = true
			index.margin = player.Automation.escortMargin()
			index.threats = nil
			for _, intel := range player.FleetIntels {
				if intel == nil || intel.Delete || intel.ReportAge != 0 || intel.PlayerNum == player.Num || player.IsFriend(intel.PlayerNum) || !patrolReportedCombatant(player, intel) {
					continue
				}
				reach := math.Min(float64(index.margin), max(20, intel.observedStrategicSpeed()+5))
				if observedBombardmentGuard(player, intel) {
					reach = math.Min(reach, 3)
				}
				index.threats = append(index.threats, indexedRouteThreat{position: intel.Position, end: observedGuardEndpoint(player, intel), reach: reach, power: patrolReportedPower(player, intel)})
			}
		}
		total := 0
		for _, threat := range index.threats {
			if segmentSegmentDistance(threat.position, threat.end, from, to) <= threat.reach {
				total = min(1_000_000_000, total+threat.power)
			}
		}
		return total
	}
	return originalKnownRouteThreat(player, from, to)
}
func originalKnownRouteThreat(player *Player, from, to Vector) int {
	threat := 0
	for _, intel := range player.FleetIntels {
		if intel == nil || intel.Delete || intel.ReportAge != 0 || intel.PlayerNum == player.Num || player.IsFriend(intel.PlayerNum) || !patrolReportedCombatant(player, intel) {
			continue
		}
		reach := math.Min(float64(player.Automation.escortMargin()), max(20, intel.observedStrategicSpeed()+5))
		if observedBombardmentGuard(player, intel) {
			reach = math.Min(reach, 3)
		}
		if segmentSegmentDistance(intel.Position, observedGuardEndpoint(player, intel), from, to) <= reach {
			threat = min(1_000_000_000, threat+patrolReportedPower(player, intel))
		}
	}
	return threat
}
func segmentDistance(point, from, to Vector) float64 {
	dx, dy := float64(to.X-from.X), float64(to.Y-from.Y)
	if dx*dx+dy*dy == 0 {
		return point.DistanceTo(from)
	}
	fraction := max(0.0, min(1.0, (float64(point.X-from.X)*dx+float64(point.Y-from.Y)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(float64(point.X-from.X)-fraction*dx, float64(point.Y-from.Y)-fraction*dy)
}

func routeAutomatically(rules *Rules, player *Player, fleet *Fleet, position Vector, target MapObjectTarget, speedLimit int) bool {
	if position == fleet.Position {
		holdAutomaticFleet(player, fleet, "")
		return true
	}
	if target.TargetType == MapObjectTypeFleet {
		target = MapObjectTarget{}
	}
	route := planAutomaticRoute(rules, player, []*Fleet{fleet}, position, target, automaticRoutePolicy{MaxWarp: speedLimit})
	if route == nil {
		holdAutomaticFleet(player, fleet, "No safe fuel route; waiting for assistance")
		saveAutomaticFuelRequest(fleet, automaticFuelRequest{Destination: position, Target: target, Members: []int{fleet.Num}, Urgent: true})
		return false
	}
	applyExpeditionRoute(player, fleet, route)
	return true
}

func (t *turnGenerator) releaseEscorts(player *Player, freighter *Fleet) {
	for _, escort := range t.game.Fleets {
		if escort.PlayerNum == player.Num && escort.GetTag(escortTag) == strconv.Itoa(freighter.Num) {
			delete(escort.Tags, escortTag)
			if restoreBomberProtection(player, escort) {
				continue
			}
			routeAutomatically(&t.game.Rules, player, escort, escort.Position, MapObjectTarget{}, 0)
		}
	}
}

func (t *turnGenerator) prepareConvoy(player *Player, fleet *Fleet, job *logisticsJob, target MapObjectTarget) {
	t.prepareConvoyWithMinimum(player, fleet, job, target, 0)
}
func (t *turnGenerator) prepareConvoyWithMinimum(player *Player, fleet *Fleet, job *logisticsJob, target MapObjectTarget, minimum int) {

	threat := int(math.Ceil(float64(knownRouteThreat(player, fleet.Position, job.Position)) * float64(player.Automation.escortStrength()) / 100))
	if knownWithdrawalRoute(player, fleet.Position, job.Position) {
		threat = 0
	}
	if (job.Kind == "planet" || job.Kind == "passengers" || job.Kind == "invade-load") && fleet.Position == job.Position && job.DeliveryNum != 0 {
		if dest := t.game.getPlanet(job.DeliveryNum); dest != nil {
			threat = max(threat, int(math.Ceil(float64(knownRouteThreat(player, fleet.Position, dest.Position))*float64(player.Automation.escortStrength())/100)))
		}
	}
	threat = max(threat, minimum)
	var escorts []*Fleet
	power := 0
	if fleet.Spec.HasWeapons {
		power = patrolPower(fleet)
	}
	escortCandidates := t.game.getFleets(player.Num)
	slices.SortStableFunc(escortCandidates, func(a, b *Fleet) int {
		return cmp.Compare(a.Position.DistanceTo(fleet.Position), b.Position.DistanceTo(fleet.Position))
	})
	for _, escort := range escortCandidates {
		if threat > power && player.Automation.FleetRoles[escort.Num] == "patrol" && readPatrolJob(escort).Leader == 0 && escort.Position.DistanceTo(fleet.Position) <= 100 && escort.Spec.HasWeapons && !escort.Delete && !escort.InDesignMaintenance() && !escort.InAutomaticSafety() && !player.FleetInActiveAssembly(escort) && !fuelNetworkReserved(escort) && readTankerJob(escort) == nil && escort.GetTag("maintenance-pending") == "" {
			borrowBomberProtection(player, escort, fleet)
		}
		if readTankerJob(escort) != nil || fuelNetworkManaged(escort) || escort.InAutomaticSafety() || escort.Delete || escort.InDesignMaintenance() || player.Automation.FleetRoles[escort.Num] != "escort" || !escort.Spec.HasWeapons {
			continue
		}
		assigned := escort.GetTag(escortTag)
		if assigned == strconv.Itoa(fleet.Num) && power >= threat {
			delete(escort.Tags, escortTag)
			continue
		}
		if assigned == strconv.Itoa(fleet.Num) || threat > power && assigned == "" && (escort.Idle() || escort.GetTag(escortStagingTag) != "") {
			delete(escort.Tags, escortStagingTag)
			delete(escort.Tags, frontierPatrolTag)
			escort.SetTag(escortTag, strconv.Itoa(fleet.Num))
			escorts = append(escorts, escort)
			power += patrolPower(escort)
		}
	}
	if threat > power {
		t.releaseEscorts(player, fleet)
		if t.withdrawEmptyFreighter(player, fleet, job) {
			return
		}
		job.Status = "Waiting for enough escort strength"
		routeAutomatically(&t.game.Rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
		return
	}
	group := []*Fleet{fleet}
	ready := true
	for _, escort := range escorts {
		if escort.Position != fleet.Position {
			if route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{escort}, fleet.Position, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true}); route != nil {
				applyExpeditionRoute(player, escort, route)
			} else {
				t.returnPatrolToFuel(player, escort)
			}
			ready = false
		}
		group = append(group, escort)
	}
	if !ready {
		job.Status = "Waiting for escort to assemble"
		holdAutomaticFleet(player, fleet, "")
		return
	}
	route := t.planAutomaticMission(player, group, job.Position, target, automaticRoutePolicy{AllowThreat: threat > 0})
	if route == nil {
		if len(escorts) == 0 && t.withdrawEmptyFreighter(player, fleet, job) {
			return
		}
		job.Status = "Waiting for convoy fuel or a safe route"
		for _, member := range group {
			holdAutomaticFleet(player, member, job.Status)
		}
		saveAutomaticFuelRequest(fleet, automaticFuelRequest{Destination: job.Position, Target: target, Members: fleetNumbers(group), Urgent: true, AllowThreat: threat > 0})
		return
	}
	for _, member := range group {
		applyExpeditionRoute(player, member, route)
	}
	if route.WaitForFuel {
		job.Status = "Convoy waiting for actual refuelling at dock"
		return
	}

	t.appendLogisticsReturn(player, group, job, route)
	job.Status = "Travelling with assigned escort"
	if len(escorts) == 0 {
		job.Status = "Travelling on a route with no known armed threats"
	}
}

func (t *turnGenerator) planLogisticsAutomation() {
	for _, player := range t.automaticPlayers() {
		for _, escort := range t.game.getFleets(player.Num) {
			if escort.GetTag(escortStagingTag) == "" && readTankerJob(escort) == nil && !fuelNetworkManaged(escort) && player.Automation.FleetRoles[escort.Num] == "escort" && escort.GetTag(escortTag) == "" && !escort.InAutomaticSafety() && !escort.InDesignMaintenance() {
				holdAutomaticFleet(player, escort, "")
			}
			if assigned := escort.GetTag(escortTag); assigned != "" {
				num, _ := strconv.Atoi(assigned)
				target := t.game.getFleet(player.Num, num)
				if player.Automation.FleetRoles[escort.Num] != "escort" || target == nil || target.Delete || (logisticsWork(target) == nil && target.GetTag(bomberTag) == "") || (!passengerCarrier(player, target) && player.Automation.FleetRoles[num] != "bomber") {
					delete(escort.Tags, escortTag)
					routeAutomatically(&t.game.Rules, player, escort, escort.Position, MapObjectTarget{}, 0)
				}
			}
		}
		fleets := slices.Clone(t.game.getFleets(player.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		claimed := map[string]bool{}
		for _, fleet := range fleets {
			if job := logisticsWork(fleet); job != nil && job.Kind != "return" {
				claimed[fmt.Sprintf("%s:%d", job.Kind, job.TargetNum)] = true
			}
			if job := logisticsWork(fleet); job != nil && job.DeliveryNum != 0 && job.Kind != "passengers" && job.Kind != "settle" {
				claimed[fmt.Sprintf("yard:%d", job.DeliveryNum)] = true
			}
		}
		for _, fleet := range fleets {
			if fuelNetworkManaged(fleet) || fleet.InAutomaticSafety() || fleet.Delete || fleet.Starbase || fleet.InDesignMaintenance() {
				continue
			}
			if fleet.GetTag(anticipatedSalvageTag) != "" || fleet.GetTag(traderConvoyTag) != "" {
				continue
			}
			job := t.currentLogisticsWork(player, fleet)
			if job != nil && job.Kind == "passengers" && fleet.CargoMass() == 0 && strings.HasPrefix(job.Status, "Waiting") && !t.passengerPickupReady(player, fleet, job) {
				if replacement := t.findPassengerTransfer(player, fleet); replacement != nil && replacement.Kind == "passengers" && t.passengerPickupReady(player, fleet, replacement) {
					t.releaseEscorts(player, fleet)
					job = replacement
					saveLogisticsWork(fleet, job)
				}
			}

			if fleet.GetTag(patrolSalvageTag) != "" && job == nil {
				delete(fleet.Tags, patrolSalvageTag)
				continue
			}
			if fleet.GetTag("maintenance-pending") != "" && job == nil {
				continue
			}
			if !passengerCarrier(player, fleet) {
				if job != nil {
					delete(fleet.Tags, logisticsTag)
					t.releaseEscorts(player, fleet)
				}
				continue
			}

			if job != nil && job.Kind == "salvage" && fleet.Cargo.Colonists == 0 && fleet.Cargo.ToMineral().Total() == 0 && strings.HasPrefix(job.Status, "Waiting") && !player.Automation.DisablePassengerTransport {
				if replacement := t.findPassengerTransfer(player, fleet); replacement != nil {
					t.releaseEscorts(player, fleet)
					job = replacement
					saveLogisticsWork(fleet, job)
				}
			}
			if fleet.Cargo.ToMineral().Total() > 0 && (job == nil || job.Kind != "return" && job.Kind != "settle") {
				delivery := 0
				if job != nil {
					delivery = job.DeliveryNum
				}
				job = &logisticsJob{Kind: "return", DeliveryNum: delivery}
			}
			var target MapObjectTarget
			if job != nil && job.Kind == "settle" {
				destination := t.passengerDestination(player, fleet, job.DeliveryNum)
				if destination == nil {
					job.Status = "Waiting for a suitable colony with room for passengers"
					routeAutomatically(&t.game.Rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
					saveLogisticsWork(fleet, job)
					continue
				}
				job.TargetNum, job.DeliveryNum, job.Position = destination.Num, destination.Num, destination.Position
				target = destination.MapObject.ToTarget()
			} else if job != nil && job.Kind == "return" {
				var nearest *Planet
				best := math.Inf(1)
				for _, planet := range t.game.Planets {
					if planet.OwnedBy(player.Num) && ((planet.Spec.HasStarbase && planet.Spec.DockCapacity != 0) || job.DeliveryNum == planet.Num && queuedStarbase(planet)) {
						if job.DeliveryNum != 0 && planet.Num != job.DeliveryNum {
							continue
						}
						if distance := fleet.Position.DistanceTo(planet.Position); distance < best {
							nearest, best = planet, distance
						}
					}
				}
				if nearest == nil && job.DeliveryNum != 0 {
					job.DeliveryNum = 0
					saveLogisticsWork(fleet, job)
					routeAutomatically(&t.game.Rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
					continue
				}
				if nearest == nil {
					job.Status = "Waiting for an owned shipyard"
					routeAutomatically(&t.game.Rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
					saveLogisticsWork(fleet, job)
					continue
				}
				job.Position, job.TargetNum = nearest.Position, nearest.Num
				target = nearest.MapObject.ToTarget()
			} else if job == nil {

				if job == nil && !colonizerPassengerCarrier(player, fleet) && !player.Automation.DisablePassengerTransport {
					job = t.findAutomaticInvasion(player, fleet)
					if job != nil {
						target = t.game.getPlanet(job.TargetNum).MapObject.ToTarget()
					}
				}
				if job == nil && !colonizerPassengerCarrier(player, fleet) && !player.Automation.DisableMineralSupply {
					job = t.findShipyardSupply(player, fleet, claimed)
					if job != nil {
						target = t.game.getPlanet(job.TargetNum).ToTarget()
						claimed[fmt.Sprintf("yard:%d", job.DeliveryNum)] = true
					}
				}
				if job == nil && !player.Automation.DisablePassengerTransport {
					job = t.findPassengerTransfer(player, fleet)
					if job != nil {
						target = t.game.getPlanet(job.TargetNum).MapObject.ToTarget()
					}
				}
				best := math.Inf(1)
				for _, salvage := range player.SalvageIntels {
					if job != nil || colonizerPassengerCarrier(player, fleet) || player.Automation.DisableSalvage {
						break
					}
					key := fmt.Sprintf("salvage:%d", salvage.Num)
					if salvage.Cargo.ToMineral().Total() == 0 || claimed[key] || fleet.GetTag("empty-salvage") == strconv.Itoa(salvage.Num) {
						continue
					}
					if distance := fleet.Position.DistanceTo(salvage.Position); distance < best {
						job = &logisticsJob{Kind: "salvage", TargetNum: salvage.Num, Position: salvage.Position}
						target = salvage.MapObject.ToTarget()
						best = distance
					}
				}
				if job == nil && !colonizerPassengerCarrier(player, fleet) && player.Automation.OffloadCombatCargo {
					for _, combat := range fleets {
						if combat.Num == fleet.Num || !combat.Spec.HasWeapons || combat.Cargo.ToMineral().Total() == 0 || player.Automation.FleetRoles[combat.Num] == "freighter" || claimed[fmt.Sprintf("fleet:%d", combat.Num)] {
							continue
						}
						if distance := fleet.Position.DistanceTo(combat.Position); distance < best {
							job = &logisticsJob{Kind: "fleet", TargetNum: combat.Num, Position: combat.Position}
							target = combat.MapObject.ToTarget()
							best = distance
						}
					}
				}
				if job == nil && !colonizerPassengerCarrier(player, fleet) && !player.Automation.DisableMineralSupply {
					job = t.findShipyardSupply(player, fleet, claimed)
					if job != nil {
						target = t.game.getPlanet(job.TargetNum).MapObject.ToTarget()
						claimed[fmt.Sprintf("yard:%d", job.DeliveryNum)] = true
					}
				}
				if job == nil && fundedLogisticsWaypoints(&t.game.Rules, player, fleet) {
					fleet.SetTag(fuelStatusTag, "Following the planned return to refuel")
					continue
				}
				if job == nil {
					holdAutomaticFleet(player, fleet, "Automatic transport enabled: waiting for an eligible mineral or passenger job")
					t.releaseEscorts(player, fleet)
					continue
				}
				claimed[fmt.Sprintf("%s:%d", job.Kind, job.TargetNum)] = true
			} else if job.Kind == "invade" {
				intel := player.GetPlanetIntel(job.DeliveryNum)
				if !invasionColony(player, intel) {
					delete(fleet.Tags, logisticsTag)
					t.releaseEscorts(player, fleet)
					continue
				}
				job.Position = intel.Position
				target = intel.MapObject.ToTarget()
			} else if job.Kind == "planet" || job.Kind == "passengers" || job.Kind == "invade-load" {
				source := t.game.getPlanet(job.TargetNum)
				if source == nil || (!source.OwnedBy(player.Num) && (job.Kind != "planet" || !t.canCollectMiningPlanet(player, source))) {
					delete(fleet.Tags, logisticsTag)
					t.releaseEscorts(player, fleet)
					continue
				}
				job.Position = source.Position
				target = source.MapObject.ToTarget()
			} else if job.Kind == "fleet" {
				source := t.game.getFleet(player.Num, job.TargetNum)
				if source == nil || source.Delete {
					delete(fleet.Tags, logisticsTag)
					t.releaseEscorts(player, fleet)
					continue
				}
				job.Position = source.Position
				target = source.MapObject.ToTarget()
			} else {
				target = MapObjectTarget{TargetType: MapObjectTypeSalvage, TargetNum: job.TargetNum, TargetPosition: job.Position}
			}
			t.prepareConvoy(player, fleet, job, target)
			saveLogisticsWork(fleet, job)
		}
	}
}

func (t *turnGenerator) armedEnemyAt(player *Player, position Vector) bool {
	for _, fleet := range t.game.Fleets {
		if !fleet.Delete && fleet.Position == position && fleet.PlayerNum != player.Num && !player.IsFriend(fleet.PlayerNum) && fleet.Spec.HasWeapons {
			return true
		}
	}
	return false
}

func (t *turnGenerator) collectBattleSalvage() {
	for _, player := range t.automaticPlayers() {
		if !player.Automation.BattleSalvage {
			continue
		}
		for _, record := range player.BattleRecords {
			enemyLosses := 0
			for owner, lost := range record.Stats.ShipsDestroyedByPlayer {
				if owner != player.Num && !player.IsFriend(owner) {
					enemyLosses += lost
				}
			}
			if enemyLosses == 0 || t.armedEnemyAt(player, record.Position) {
				continue
			}
			salvage := t.game.salvagesByPosition[record.Position]
			if salvage == nil || salvage.Delete {
				continue
			}
			for _, fleet := range t.game.getFleets(player.Num) {
				if !fleet.Delete && fleet.Position == record.Position && fleet.Spec.HasWeapons && !fleet.InDesignMaintenance() {
					takeMinerals(fleet, &salvage.Cargo)
				}
			}
			if salvage.Cargo.ToMineral().Total() == 0 {
				t.game.deleteSalvage(salvage)
			}
		}
	}
}

func (t *turnGenerator) resolveLogisticsAutomation() {
	for _, player := range t.automaticPlayers() {
		for _, fleet := range t.game.getFleets(player.Num) {
			if fleet.InAutomaticSafety() || fleet.Delete || !passengerCarrier(player, fleet) || fleet.InDesignMaintenance() {
				continue
			}
			job := logisticsWork(fleet)
			if job == nil || fleet.Position != job.Position || t.armedEnemyAt(player, fleet.Position) {
				continue
			}
			switch job.Kind {
			case "return":
				planet := t.game.getPlanet(job.TargetNum)
				if planet == nil || !planet.OwnedBy(player.Num) {
					continue
				}
				planet.Cargo = planet.Cargo.AddMineral(fleet.Cargo.ToMineral())
				fleet.Cargo.Ironium, fleet.Cargo.Boranium, fleet.Cargo.Germanium = 0, 0, 0
				delete(fleet.Tags, logisticsTag)
				t.releaseEscorts(player, fleet)
			case "salvage":
				salvage := t.game.getSalvage(job.TargetNum)
				if salvage != nil && !salvage.Delete {
					takeMinerals(fleet, &salvage.Cargo)
					if salvage.Cargo.ToMineral().Total() == 0 {
						t.game.deleteSalvage(salvage)
					}
				} else {
					fleet.SetTag("empty-salvage", strconv.Itoa(job.TargetNum))
				}
				delete(fleet.Tags, logisticsTag)
			case "fleet":
				source := t.game.getFleet(player.Num, job.TargetNum)
				if source != nil && !source.Delete && source.Position == fleet.Position {
					takeMinerals(fleet, &source.Cargo)
				}
				delete(fleet.Tags, logisticsTag)
			case "planet":
				source := t.game.getPlanet(job.TargetNum)
				if source == nil || (!source.OwnedBy(player.Num) && (job.Kind != "planet" || !t.canCollectMiningPlanet(player, source))) {
					delete(fleet.Tags, logisticsTag)
					continue
				}
				dest := t.game.getPlanet(job.DeliveryNum)
				if dest == nil {
					continue
				}
				probe, origin := *fleet, *source
				probe.Tags = maps.Clone(fleet.Tags)
				t.loadOpportunisticInfrastructure(player, &probe, &origin, dest, 0)
				spare := origin.Cargo.ToMineral().ToCost().Subtract(t.productionMinerals(player, &origin, false).ToCost()).ToMineral()
				load := Cargo{Ironium: max(0, min(spare.Ironium, job.Load.Ironium)), Boranium: max(0, min(spare.Boranium, job.Load.Boranium)), Germanium: max(0, min(spare.Germanium, job.Load.Germanium))}
				before := load
				takeMinerals(&probe, &load)
				origin.Cargo = origin.Cargo.Subtract(before.Subtract(load))
				t.topUpLogisticsCargo(player, &probe, &origin, dest)
				if suitableColony(player, dest) && !player.Automation.DisablePassengerTransport {
					incoming, _ := t.passengerReservations(player, fleet.Num)
					passengers := max(0, min(probe.availableColonistSpace(), origin.Cargo.Colonists-sourcePopulationReserve(player, &origin), populationTarget(dest, player.Automation.destinationPopulationPercent())-dest.Cargo.Colonists-incoming[dest.Num]))
					origin.Cargo.Colonists -= passengers
					probe.Cargo.Colonists += passengers
				}
				route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{&probe}, dest.Position, dest.ToTarget(), automaticRoutePolicy{AllowThreat: true, MinWarp: automaticCruiseWarp(player, fleet)})
				if probe.CargoMass() != fleet.Spec.CargoCapacity || route == nil || route.WaitForFuel || !t.deliveryEscortReady(player, fleet, dest.Position) {
					job.Status = fmt.Sprintf("Waiting empty at source for a full safe load (%d/%d kT available)", probe.CargoMass(), fleet.Spec.CargoCapacity)
					if probe.CargoMass() == fleet.Spec.CargoCapacity && route != nil && !route.WaitForFuel && !t.deliveryEscortReady(player, fleet, dest.Position) {
						job.Status = "Waiting empty for escort before loading: armed enemy on the delivery route"
					}
					holdAutomaticFleet(player, fleet, job.Status)
					saveLogisticsWork(fleet, job)
					continue
				}
				source.Cargo, source.Factories, source.Mines, source.Defenses = origin.Cargo, origin.Factories, origin.Mines, origin.Defenses
				fleet.Cargo = probe.Cargo
				saveInfrastructure(fleet, readInfrastructure(&probe))
				job.Kind = "return"
				job.TargetNum, job.Position = dest.Num, dest.Position
				if fleet.Cargo.Colonists > 0 {
					job.Kind = "settle"
				}
				saveLogisticsWork(fleet, job)
			case "invade-load":
				t.loadAutomaticInvaders(player, fleet, job)
			case "passengers":
				t.loadAutomaticPassengers(player, fleet, job)
			case "settle":
				t.unloadAutomaticPassengers(player, fleet, job)
			}
		}
	}
}

func (t *turnGenerator) pruneAutomationAssignments() {
	for _, player := range t.automaticPlayers() {
		for num := range player.Automation.FleetRoles {
			if fleet := t.game.getFleet(player.Num, num); fleet == nil || fleet.Delete {
				delete(player.Automation.FleetRoles, num)
			}
		}
		for num := range player.Automation.RebuildFleets {
			if fleet := t.game.getFleet(player.Num, num); fleet == nil || fleet.Delete {
				delete(player.Automation.RebuildFleets, num)
			}
		}
	}
}

// Loading at waypoint zero must be followed by departure in the same turn.
// Refresh only ready deliveries, not the expensive global source/destination search.
func (t *turnGenerator) dispatchReadyLogistics() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if f.Delete || fuelNetworkManaged(f) || f.InAutomaticSafety() || f.InDesignMaintenance() || !passengerCarrier(p, f) {
				continue
			}
			job := logisticsWork(f)
			if job == nil || (job.Kind != "settle" && job.Kind != "return" && job.Kind != "invade") {
				continue
			}
			destination := t.game.getPlanet(job.DeliveryNum)
			if destination == nil {
				continue
			}
			if len(f.Waypoints) > 1 && f.Waypoints[1].Position == destination.Position {
				continue
			}
			job.Position = destination.Position
			t.prepareConvoy(p, f, job, destination.ToTarget())
			saveLogisticsWork(f, job)
		}
	}
}

// Pickups may request their delivery escort before loading. Loading and physical
// departure must agree on the same threat and co-located protection budget.
func (t *turnGenerator) deliveryEscortReady(p *Player, f *Fleet, to Vector) bool {
	need := int(math.Ceil(float64(knownRouteThreat(p, f.Position, to)) * float64(p.Automation.escortStrength()) / 100))
	if knownWithdrawalRoute(p, f.Position, to) {
		need = 0
	}
	power := 0
	if f.Spec.HasWeapons {
		power = patrolPower(f)
	}
	for _, e := range t.game.getFleets(p.Num) {
		if !e.Delete && e.Position == f.Position && e.Spec.HasWeapons && p.Automation.FleetRoles[e.Num] == "escort" && e.GetTag(escortTag) == strconv.Itoa(f.Num) {
			power += patrolPower(e)
		}
	}
	return power >= need
}

// A mixed bomber formation visibly committed to an owned planet is a siege
// group, not a roaming interceptor. Only its observed travel corridor blocks
// civilian routes; its real firepower still enters planetary defense planning.
func observedBombardmentGuard(p *Player, f *Fleet) bool {
	if f == nil || f.ReportAge != 0 || !patrolReportedBomber(p, f) {
		return false
	}
	for _, planet := range p.PlanetIntels {
		if planet == nil || !planet.OwnedBy(p.Num) {
			continue
		}
		if planet.Position == f.Position {
			return f.WarpSpeed == 0
		}
		if f.WarpSpeed <= 0 || f.WarpSpeed > 10 {
			continue
		}
		heading := f.Heading
		toward := planet.Position.Subtract(f.Position)
		if heading.X*float64(toward.X)+heading.Y*float64(toward.Y) <= 0 {
			continue
		}
		end := f.Position.ToFloat64().Add(heading.Scale(2 * f.observedStrategicSpeed())).ToInt(true)
		if segmentDistance(planet.Position, f.Position, end) <= 3 {
			return true
		}
	}
	return false
}
func observedGuardEndpoint(p *Player, f *Fleet) Vector {
	if observedBombardmentGuard(p, f) {
		return patrolInterceptPosition(f)
	}
	return f.Position
}
func segmentSegmentDistance(a, b, c, d Vector) float64 {
	if a == b {
		return segmentDistance(a, c, d)
	}
	if c == d {
		return segmentDistance(c, a, b)
	}
	cross := func(p, q, r Vector) float64 {
		return float64(q.X-p.X)*float64(r.Y-p.Y) - float64(q.Y-p.Y)*float64(r.X-p.X)
	}
	abC, abD, cdA, cdB := cross(a, b, c), cross(a, b, d), cross(c, d, a), cross(c, d, b)
	if (abC > 0 && abD < 0 || abC < 0 && abD > 0) && (cdA > 0 && cdB < 0 || cdA < 0 && cdB > 0) {
		return 0
	}
	return min(segmentDistance(a, c, d), segmentDistance(b, c, d), segmentDistance(c, a, b), segmentDistance(d, a, b))
}
