package cs

// Only armed, fresh reports participate in corridor checks. Thousands of
// harmless scout/cargo reports are classified once during an orders phase.
type indexedRouteThreat struct {
	position  Vector
	end       Vector
	reach     float64
	power     int
	departure VectorFloat64
}

// Reports of a fleet leaving the entire corridor need no escort on an ordinary
// civilian leg. Keep the contact margin, including at the origin and destination.
func (threat indexedRouteThreat) leavesCivilianCorridor(from, to Vector) bool {
	if from == to || threat.departure.LengthSquared() == 0 || segmentDistance(threat.position, from, to) <= 3 {
		return false
	}
	behind := func(point Vector) bool {
		offset := point.Subtract(threat.position)
		return threat.departure.X*float64(offset.X)+threat.departure.Y*float64(offset.Y) < 0
	}
	return behind(from) && behind(to)
}

func routeThreatReport(p *Player, intel *Fleet, margin float64) indexedRouteThreat {
	reach := min(margin, max(20, intel.observedStrategicSpeed()+5))
	end := intel.Position
	if observedBombardmentGuard(p, intel) {
		reach = min(reach, 3)
		end = patrolInterceptPosition(intel)
	}
	report := indexedRouteThreat{position: intel.Position, end: end, reach: reach}
	if !intel.Starbase && intel.WarpSpeed > 0 && intel.WarpSpeed <= 10 {
		report.departure = intel.Heading
	}
	return report
}

type routeThreatIndex struct {
	built   bool
	margin  int
	threats []indexedRouteThreat
}

func (t *turnGenerator) enablePlanningThreatIndex() {
	for _, p := range t.game.Players {
		p.routeThreatIndex = &routeThreatIndex{}
	}
}
func (t *turnGenerator) disablePlanningThreatIndex() {
	for _, p := range t.game.Players {
		p.routeThreatIndex = nil
	}
}

// Standalone recovery checks also search thousands of edges. Their report
// snapshot is immutable for the duration of one route search. Dispose of the
// temporary index before returning, so later movement/intel changes are visible.
func scopedRouteThreatIndex(p *Player) func() {
	if p.routeThreatIndex != nil {
		return func() {}
	}
	p.routeThreatIndex = &routeThreatIndex{}
	return func() { p.routeThreatIndex = nil }
}
