package cs

// Only armed, fresh reports participate in corridor checks. Thousands of
// harmless scout/cargo reports are classified once during an orders phase.
type indexedRouteThreat struct {
	position Vector
	reach    float64
	power    int
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
