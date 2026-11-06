package cs

// Only armed, fresh reports participate in corridor checks. Thousands of
// harmless scout/cargo reports are classified once during an orders phase.
type indexedRouteThreat struct {
	position Vector
	end      Vector
	next     Vector
	reach    float64
	power    int
}

func (threat indexedRouteThreat) threatensRoute(from, to Vector, civilian bool) bool {
	if civilian {
		// Passing a fleet in space does not initiate orbital combat. A stop on
		// its reported position or course can still bring the ships together.
		return routeBoundsOverlap(to, to, threat.position, threat.next, 3) && segmentDistance(to, threat.position, threat.next) <= 3
	}
	return routeBoundsOverlap(from, to, threat.position, threat.end, threat.reach) && segmentSegmentDistance(threat.position, threat.end, from, to) <= threat.reach
}

// Disjoint expanded bounding boxes cannot contain segments within reach.
// Keep the exact geometry for overlapping boxes, including boundary contact.
func routeBoundsOverlap(a, b, c, d Vector, reach float64) bool {
	return float64(min(a.X, b.X)) <= float64(max(c.X, d.X))+reach &&
		float64(min(c.X, d.X)) <= float64(max(a.X, b.X))+reach &&
		float64(min(a.Y, b.Y)) <= float64(max(c.Y, d.Y))+reach &&
		float64(min(c.Y, d.Y)) <= float64(max(a.Y, b.Y))+reach
}

func routeThreatReport(p *Player, intel *Fleet, margin float64) indexedRouteThreat {
	reach := min(margin, max(20, intel.observedStrategicSpeed()+5))
	end, next := intel.Position, patrolInterceptPosition(intel)
	if observedBombardmentGuard(p, intel) {
		reach = min(reach, 3)
		end = next
	}
	return indexedRouteThreat{position: intel.Position, end: end, next: next, reach: reach}
}

type routeThreatIndex struct {
	built         bool
	margin        int
	threats       []indexedRouteThreat
	hostileOrbits map[Vector]*Planet
	routes        map[routeThreatKey]int
}

type routeThreatKey struct {
	from, to Vector
	civilian bool
}

// A single route search reads an immutable report snapshot, often testing the
// same corridor at many warp speeds. Never retain these results between searches.
func scopedRouteThreatCache(p *Player) func() {
	index := p.routeThreatIndex
	previous := index.routes
	index.routes = make(map[routeThreatKey]int)
	return func() { index.routes = previous }
}

func hostileOrbitalBase(p *Player, point Vector) bool {
	planet := reportedHostileOrbit(p, point)
	return planet != nil && planet.Spec.HasStarbase
}

func reportedHostileOrbit(p *Player, point Vector) *Planet {
	if index := p.routeThreatIndex; index != nil {
		if index.hostileOrbits == nil {
			index.hostileOrbits = map[Vector]*Planet{}
			for _, planet := range p.PlanetIntels {
				if hostileOrbit(p, planet) {
					index.hostileOrbits[planet.Position] = planet
				}
			}
		}
		return index.hostileOrbits[point]
	}
	for _, planet := range p.PlanetIntels {
		if planet != nil && planet.Position == point && hostileOrbit(p, planet) {
			return planet
		}
	}
	return nil
}

func hostileOrbit(p *Player, planet *Planet) bool {
	return planet != nil && planet.Owned() && !planet.OwnedBy(p.Num) && !p.IsFriend(planet.PlayerNum)
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
