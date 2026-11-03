package cs

// Public orbital status and owned sensors are sufficient here. Hidden enemy
// cloaking is not consulted; use a range margin when predicting coverage.
func reportedContactCovered(p *Player, fleets []*Fleet, enemy *Fleet) bool {
	point := patrolInterceptPosition(enemy)
	covered := func(position Vector, normal, pen int) bool {
		radius := normal
		if enemy.Orbiting() {
			radius = pen
		}
		return radius > 0 && position.DistanceTo(point) <= float64(radius)*.8
	}
	for _, planet := range p.PlanetIntels {
		if planet != nil && planet.OwnedBy(p.Num) && covered(planet.Position, planet.Spec.ScanRange, planet.Spec.ScanRangePen) {
			return true
		}
	}
	for _, f := range fleets {
		if f != nil && !f.Delete && f.PlayerNum == p.Num && f.GetTag(preyWatchTag) == "" && f.Spec.Scanner && covered(f.Position, f.Spec.ScanRange, f.Spec.ScanRangePen) {
			return true
		}
	}
	return false
}
