package cs

import "math"

// Predict colonist casualties from published defenses and our own bombs only.
// Round kills down, leaving one extra cargo unit; forecasts never consume RNG.
func (t *turnGenerator) forecastLandingAfterBombing(p *Player, target *Planet) *Planet {
	if target == nil || target.ReportAge > 1 || target.Spec.HasStarbase || target.Defenses > 0 || target.Cargo.Colonists <= 0 {
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
	if job.BombingStage && target.ReportAge <= 2 && !priorityNeedsBombing(target) {
		job.BombingStage = false
	}
	if !job.BombingStage {
		job.Position = target.Position
		return target.ToTarget()
	}
	direction := f.Position.Subtract(target.Position).Normalized()
	if direction.LengthSquared() == 0 {
		direction = VectorFloat64{X: 1}
	}
	job.Position = target.Position.ToFloat64().Add(direction.Scale(35)).ToInt(true)
	job.Status = "Staging outside the target for a landing after bombardment; awaiting fresh reconnaissance"
	return MapObjectTarget{}
}
