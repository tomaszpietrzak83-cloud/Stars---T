package cs

import (
	"fmt"
	"math"
	"strconv"
)

const repairTankerTag = "repair-tanker"

func (t *turnGenerator) nearestTankerRepair(p *Player, f *Fleet, stationRoute *expeditionRoute, station *Planet) (*Fleet, *expeditionRoute) {
	bestTime := math.Inf(1)
	damage := fleetDamagePercent(f)
	if stationRoute != nil && station != nil {
		rate := t.game.Rules.RepairRates[RepairRateOrbitingOwnPlanet]*p.Race.Spec.RepairFactor + max(f.Spec.RepairBonus, station.StarbaseBonus())
		bestTime = float64(stationRoute.Turns) + math.Ceil(float64(damage)/max(.01, 100*rate))
	}
	var best *Fleet
	var route *expeditionRoute
	for _, donor := range t.game.getFleets(p.Num) {
		if donor == f || donor.Delete || p.Automation.FleetRoles[donor.Num] != "tanker" || donor.InAutomaticSafety() || donor.InDesignMaintenance() || fuelNetworkManaged(donor) || fuelNetworkReserved(donor) || donor.Spec.RepairBonus <= 0 || donor.noHeal || knownRouteThreat(p, donor.Position, donor.Position) > 0 {
			continue
		}
		if job := readTankerJob(donor); job != nil && job.TargetNum > 0 {
			continue
		}
		candidate := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, donor.Position, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 3, MinWarp: automaticCruiseWarp(p, f)})
		if candidate == nil {
			continue
		}
		rate := t.game.Rules.RepairRates[RepairRateStopped]*p.Race.Spec.RepairFactor + max(f.Spec.RepairBonus, donor.Spec.RepairBonus)
		eta := float64(candidate.Turns) + math.Ceil(float64(damage)/max(.01, 100*rate))
		if eta < bestTime {
			best, bestTime, route = donor, eta, candidate
		}
	}
	return best, route
}
func (p *Planet) StarbaseBonus() float64 {
	if p.Starbase != nil && !p.Starbase.Delete && !p.Starbase.noHeal {
		return p.Starbase.Spec.RepairBonus
	}
	return 0
}

// A mobile workshop pauses its sector patrol only for a real nearby patient.
func (t *turnGenerator) holdTankerForRepairs(p *Player, donor *Fleet) bool {
	if donor.Spec.RepairBonus <= 0 || donor.InAutomaticSafety() || bomberChaseThreat(p, donor) > 0 {
		return false
	}
	if job := readTankerJob(donor); job != nil && job.TargetNum > 0 {
		return false
	}
	for _, f := range t.game.getFleets(p.Num) {
		if !f.Delete && f.InAutomaticSafety() && fleetDamagePercent(f) > 0 && f.GetTag(repairTankerTag) == strconv.Itoa(donor.Num) {
			holdAutomaticFleet(p, donor, "Mobile repair service: waiting for and repairing nearby friendly ships")
			return true
		}
	}
	return false
}
func (t *turnGenerator) mobileRepairBonus(p *Player, f *Fleet) float64 {
	bonus := f.Spec.RepairBonus
	for _, donor := range t.game.getFleets(p.Num) {
		if donor != f && !donor.Delete && !donor.noHeal && donor.Position == f.Position && p.Automation.FleetRoles[donor.Num] == "tanker" {
			bonus = max(bonus, donor.Spec.RepairBonus)
		}
	}
	return bonus
}
func repairTankerStatus(donor *Fleet) string {
	return fmt.Sprintf("Repair rendezvous with %s; mobile workshop bonus does not stack", donor.Name)
}

func (t *turnGenerator) refuelRepairPatients(p *Player, donor *Fleet) {
	if donor.Delete || donor.InAutomaticSafety() || p.Automation.FleetRoles[donor.Num] != "tanker" {
		return
	}
	reserve := automaticReturnFuel(&t.game.Rules, p, donor, donor.Position, expeditionStations(p, donor), false)
	available := max(0, donor.Fuel-reserve)
	for _, patient := range t.game.getFleets(p.Num) {
		if patient.Delete || patient.Position != donor.Position || patient.GetTag(repairTankerTag) != strconv.Itoa(donor.Num) {
			continue
		}
		amount := min(available, max(0, patient.Spec.FuelCapacity-patient.Fuel))
		patient.Fuel += amount
		donor.Fuel -= amount
		available -= amount
		patient.ComputeFuelUsage(p)
	}
}
