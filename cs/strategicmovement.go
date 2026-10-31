package cs

import "math"

func (engine Engine) strategicFactor() float64 {
	if math.IsNaN(engine.StrategicSpeedFactor) || math.IsInf(engine.StrategicSpeedFactor, 0) {
		return 1
	}
	return max(1, engine.StrategicSpeedFactor)
}

// StrategicSpeed returns ly/year, not battle movement. Warp 11 is a gate, not
// an engine setting. Missing factors in existing saves preserve classic warp².
func (engine Engine) StrategicSpeed(warp int) float64 {
	if warp <= 0 || warp >= StargateWarpSpeed {
		return 0
	}
	speed := float64(warp * warp)
	if warp >= 9 {
		speed *= engine.strategicFactor()
	}
	return speed
}

func (fleet *Fleet) StrategicSpeed(warp int) float64 {
	return fleet.Spec.Engine.StrategicSpeed(warp)
}

func (fleet *Fleet) observedStrategicSpeed() float64 {
	if fleet.WarpSpeed <= 0 || fleet.WarpSpeed >= StargateWarpSpeed {
		return 0
	}
	if fleet.ObservedSpeed > 0 {
		return fleet.ObservedSpeed
	}
	return fleet.StrategicSpeed(fleet.WarpSpeed)
}
