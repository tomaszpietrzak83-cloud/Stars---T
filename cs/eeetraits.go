package cs

import (
	"fmt"
	"slices"
)

const (
	EEEEfficientDrives = 1 << iota
	EEEExtendedTanks
	EEECargoEngineering
	EEEExtractionSpecialists
	EEEMaterialRecovery
	EEEMaintenanceCrews
	EEEFieldWorkshops
	EEEOrbitalAssembly
	EEEStationEngineers
	EEESurveyNetworks
	EEEMineNavigation
	EEEResearchSpecialization
	EEEInefficientPropulsion
	EEELightweightHulls
	EEEExpensiveShipbuilding
	EEESlowMaintenance
)

var eeeCosts = []int{35, 25, 40, 45, 20, 30, 45, 70, 30, 55, 45, 45, -25, -35, -40, -20}
var eeeOpposites = [][2]int{
	{EEEEfficientDrives, EEEInefficientPropulsion},
	{EEEMaintenanceCrews, EEESlowMaintenance},
	{EEEFieldWorkshops, EEESlowMaintenance},
	{EEEOrbitalAssembly, EEEExpensiveShipbuilding},
}

func (r *Race) HasEEE(trait int) bool { return r.EEETraits&trait != 0 }

// Displayed points, before the existing standard-LRT count adjustment.
// IFE reduces the disadvantage refund; ARM and UR increase the value of
// their matching advantages. These surcharges are visible in the wizard.
func (r *Race) EEETraitCost(trait int) int {
	cost := 0
	for i, base := range eeeCosts {
		if trait == 1<<i {
			cost = base
			break
		}
	}
	if r.HasLRT(IFE) {
		if trait == EEEEfficientDrives {
			cost += 10
		}
		if trait == EEEInefficientPropulsion {
			cost += 10
		}
	}
	if r.HasLRT(ARM) && trait == EEEExtractionSpecialists {
		cost += 15
	}
	if r.HasLRT(UR) && trait == EEEMaterialRecovery {
		cost += 10
	}
	return cost
}

func (r *Race) EEECost() int {
	cost := 0
	for i := range eeeCosts {
		trait := 1 << i
		if r.HasEEE(trait) {
			cost += r.EEETraitCost(trait)
		}
	}
	return cost
}

func (r *Race) ValidateEEE() error {
	if r.EEETraits < 0 || r.EEETraits & ^((1<<len(eeeCosts))-1) != 0 {
		return fmt.Errorf("unknown EEE racial traits")
	}
	for _, pair := range eeeOpposites {
		if r.HasEEE(pair[0]) && r.HasEEE(pair[1]) {
			return fmt.Errorf("opposing EEE traits cannot be combined")
		}
	}
	if r.HasEEE(EEEResearchSpecialization) {
		if !slices.Contains(TechFields, r.EEEResearchField) {
			return fmt.Errorf("EEE research specialization requires one research field")
		}
	} else if r.EEEResearchField != TechFieldNone {
		return fmt.Errorf("EEE research field requires research specialization")
	}
	return nil
}

func (r *Race) applyEEESpec(spec *RaceSpec) {
	spec.EEETraits = r.EEETraits
	if r.HasEEE(EEEEfficientDrives) {
		spec.FuelEfficiencyOffset = (1+spec.FuelEfficiencyOffset)*0.9 - 1
	}
	if r.HasEEE(EEEInefficientPropulsion) {
		spec.FuelEfficiencyOffset = (1+spec.FuelEfficiencyOffset)*1.15 - 1
	}
	if r.HasEEE(EEEMaintenanceCrews) {
		spec.RepairFactor *= 1.25
		spec.StarbaseRepairFactor *= 1.25
	}
	if r.HasEEE(EEESlowMaintenance) {
		spec.RepairFactor *= 0.75
		spec.StarbaseRepairFactor *= 0.75
	}
	if r.HasEEE(EEESurveyNetworks) {
		spec.ScanRangeFactor *= 1.15
	}
	if r.HasEEE(EEEMineNavigation) {
		spec.MinefieldSafeWarpBonus++
	}
}

func eeeResourceCost(cost CostFloat64, spec RaceSpec, starbase bool) CostFloat64 {
	percent := 100
	if starbase {
		if spec.EEETraits&EEEStationEngineers != 0 {
			percent = 90
		}
	} else {
		if spec.EEETraits&EEEOrbitalAssembly != 0 {
			percent = 90
		}
		if spec.EEETraits&EEEExpensiveShipbuilding != 0 {
			percent = 110
		}
	}
	if percent != 100 {
		cost.Resources = cost.Resources * float64(percent) / 100
	}
	return cost
}

func (r *Race) eeeResearchCost(cost int, field TechField) int {
	if r.HasEEE(EEEResearchSpecialization) && r.EEEResearchField == field && cost > 0 {
		return max(1, (cost*9+9)/10)
	}
	return cost
}
