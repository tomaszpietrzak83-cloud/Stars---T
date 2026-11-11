package cs

// Free movement must be useful, and available to the slowest engine in the
// complete fleet. Warp-one drift is not an adequate return or patrol plan.
func usefulFreePatrolMovement(p *Player, f *Fleet) bool {
	warp := patrolFreeWarp(p, f)
	return warp >= 4 && f.StrategicSpeed(warp) >= 16
}

// Use the actual slowest engine represented in the fleet spec.
func patrolFreeWarp(p *Player, f *Fleet) int {
	for warp := min(f.Spec.Engine.FreeSpeed, automaticMaxWarp(p, f)); warp > 0; warp-- {
		if f.GetFuelCost(p, warp, 1000) <= 0 {
			return warp
		}
	}
	return 0
}

func economicalPatrolMaxWarp(p *Player, f *Fleet) int {
	if warp := patrolFreeWarp(p, f); warp >= 4 {
		return warp
	}
	return min(6, automaticMaxWarp(p, f))
}
