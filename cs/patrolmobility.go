package cs

// Free movement must be useful, and available to the slowest engine in the
// complete fleet. Warp-one drift is not an adequate return or patrol plan.
func usefulFreePatrolMovement(p *Player, f *Fleet) bool {
	warp := min(f.Spec.Engine.FreeSpeed, automaticMaxWarp(p, f))
	return warp >= 4 && f.StrategicSpeed(warp) >= 16 && f.GetFuelCost(p, warp, 1000) <= 0
}
