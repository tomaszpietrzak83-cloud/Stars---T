package cs

import "math"

func galleonLandingNeedsScan(f *Fleet, job *logisticsJob, target *Planet) bool {
	if job.Kind != "invade" || target == nil || target.ReportAge <= 2 ||
		job.LandingCleared && job.LandingTarget == target.Num && job.LandingOwner == target.PlayerNum {
		return false
	}
	return carriesGalleon(f)
}

func carriesGalleon(f *Fleet) bool {
	for _, token := range f.Tokens {
		if token.design != nil && token.design.Hull == Galleon.Name {
			return true
		}
	}
	return false
}

// Predict colonist casualties from published defenses and our own bombs only.
// Round kills down, leaving one extra cargo unit; forecasts never consume RNG.
func (t *turnGenerator) forecastLandingAfterBombing(p *Player, target *Planet) *Planet {
	if target == nil || target.ReportAge > 1 || target.Spec.HasStarbase || reportedGroundDefenses(target) || target.Cargo.Colonists <= 0 {
		return nil
	}
	normal, floor, smart := 0.0, 0, 1.0
	present := false
	for _, f := range t.game.getFleets(p.Num) {
		if f.Delete || p.Automation.FleetRoles[f.Num] != "bomber" || f.InAutomaticSafety() || f.InDesignMaintenance() {
			continue
		}
		arriving := f.Position == target.Position
		if !arriving && len(f.Waypoints) > 1 {
			wp := f.Waypoints[1]
			if wp.Position == target.Position && wp.WarpSpeed > 0 && wp.WarpSpeed <= automaticMaxWarp(p, f) && f.Position.DistanceTo(target.Position) <= f.StrategicSpeed(wp.WarpSpeed) {
				_, _, _, arriving = automaticLegFuel(p, f, wp.WarpSpeed, f.Position.DistanceTo(target.Position), f.Fuel, 0)
			}
		}
		if !arriving {
			continue
		}
		present = true
		for _, b := range f.Spec.Bombs {
			normal += b.KillRate * 10 * float64(b.Quantity)
			floor += b.MinKillRate * b.Quantity / 100
		}
		for _, b := range f.Spec.SmartBombs {
			smart *= math.Pow(1-b.KillRate/100, float64(b.Quantity))
		}
	}
	if !present {
		return nil
	}
	pop := int(math.Ceil(float64(target.Cargo.Colonists) * 1.25))
	smartRate := int(math.Round((1 - smart) * 1000 * (1 - target.Spec.DefenseCoverageSmart)))
	sk := min(pop-1, pop*smartRate/1000)
	rate := int(math.Round(normal * (1 - target.Spec.DefenseCoverage)))
	killed := max(sk+(pop-sk)*rate/1000, int(float64(floor)*(1-target.Spec.DefenseCoverage)))
	predicted := *target
	predicted.Cargo.Colonists = max(1, pop-killed+1)
	predicted.ReportAge = 0
	if predicted.Cargo.Colonists > 1000 {
		return nil
	}
	return &predicted
}

// Staging stays outside the hostile orbit until a real refreshed report makes
// the landing eligible. A forecast is never permission to drop troops early.
func (t *turnGenerator) invasionDestination(p *Player, f *Fleet, job *logisticsJob, target *Planet) MapObjectTarget {
	if job.Kind != "invade" || job.LandingTarget != target.Num || job.LandingOwner != target.PlayerNum {
		job.LandingCleared = false
	}
	// A fresh report clears departure. Merely ageing during a funded voyage
	// must not reverse loaded transports every third year; new hostile evidence
	// still cancels that clearance, and native arrival checks remain mandatory.
	blocked := target.Spec.HasStarbase || reportedGroundDefenses(target) || target.ReportAge <= 2 && target.Cargo.Colonists > 1000
	if galleonLandingNeedsScan(f, job, target) || p.Automation.PriorityPlanets[target.Num] && (blocked || target.ReportAge > 2 && !job.LandingCleared) {
		job.BombingStage = true
		job.LandingCleared = false
	}
	if target.ReportAge <= 2 && (!priorityNeedsBombing(target) || t.percentageBombingLanding(p, target)) {
		job.BombingStage = false
		job.LandingCleared = true
		job.LandingTarget = target.Num
		job.LandingOwner = target.PlayerNum
	}
	if !job.BombingStage {
		job.Position = target.Position
		return target.ToTarget()
	}
	// Keep a loaded mission at a reachable own colony while reconnaissance or
	// bombardment is pending. An arbitrary point next to the enemy is not a rally.
	var rally *Planet
	best := math.Inf(1)
	for _, colony := range t.game.getPlanets(p.Num) {
		if d := colony.Position.DistanceTo(target.Position); d < best {
			if colony.Position == f.Position || planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, colony.Position, colony.ToTarget(), automaticRoutePolicy{}) != nil {
				rally, best = colony, d
			}
		}
	}
	if rally != nil {
		job.Position = rally.Position
		job.Status = "Staging at an owned colony for reconnaissance or bombardment"
		return rally.ToTarget()
	}
	direction := f.Position.Subtract(target.Position).Normalized()
	if direction.LengthSquared() == 0 {
		direction = VectorFloat64{X: 1}
	}
	job.Position = target.Position.ToFloat64().Add(direction.Scale(35)).ToInt(true)
	job.Status = "Staging outside the target for a landing after bombardment; awaiting fresh reconnaissance"
	return MapObjectTarget{}
}
