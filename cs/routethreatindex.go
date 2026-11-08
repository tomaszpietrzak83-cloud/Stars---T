package cs

// Only armed, fresh reports participate in arrival checks. Thousands of
// harmless scout/cargo reports are classified once during an orders phase.
type indexedRouteThreat struct {
	position Vector
	end      Vector
	next     Vector
	reach    float64
	power    int
}

func (threat indexedRouteThreat) threatensRoute(from, to Vector, civilian bool) bool {
	// Crossing either fleet's course is not combat. Only their predicted
	// end-of-year meeting matters; retain the existing 3 ly report tolerance.
	return routeBoundsOverlap(to, to, threat.next, threat.next, 3) && to.DistanceTo(threat.next) <= 3
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
	return indexedRouteThreat{position: intel.Position, next: patrolInterceptPosition(intel)}
}

type routeThreatIndex struct {
	built         bool
	margin        int
	threats       []indexedRouteThreat
	hostileOrbits map[Vector]*Planet
	routes        map[routeThreatKey]int
	mineLegs      map[routeMineKey][]routeMineObstacle
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
	previousMines := index.mineLegs
	index.routes = make(map[routeThreatKey]int)
	index.mineLegs = make(map[routeMineKey][]routeMineObstacle)
	return func() { index.routes, index.mineLegs = previous, previousMines }
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
