package cs

import (
	"math"
	"math/rand"
	"testing"
)

func (t *turnGenerator) referenceTankerAnchorValue(player *Player, tanker *Fleet, point Vector) float64 {
	demand := 1.0
	nearestDock := math.Inf(1)
	for _, planet := range t.game.getPlanets(player.Num) {
		distance := point.DistanceTo(planet.Position)
		if planet.Spec.DockCapacity != 0 {
			nearestDock = min(nearestDock, distance)
		} else {
			demand += 4 / (1 + distance/80)
		}
	}
	for _, fleet := range t.game.getFleets(player.Num) {
		if isAutomaticFleet(player, fleet) && player.Automation.FleetRoles[fleet.Num] != "tanker" && len(fleet.Waypoints) > 1 {
			for _, wp := range fleet.Waypoints[1:] {
				demand += 3 / (1 + segmentDistance(point, fleet.Position, wp.Position)/60)
			}
		}
	}
	// Less benefit next to an unlimited fixed fuel source.
	demand *= min(1.0, 0.1+nearestDock/120)
	for _, other := range t.game.getFleets(player.Num) {
		if other == tanker || other.Delete || player.Automation.FleetRoles[other.Num] != "tanker" {
			continue
		}
		center := other.Position
		if job := readTankerJob(other); job != nil && job.Stage == "patrol" {
			center = job.Anchor
		}
		demand /= 1 + 6*math.Exp(-float64(point.DistanceSquaredTo(center))/float64(120*120))
	}
	return demand
}

func tankerScoreFixture() (*turnGenerator, *Player, *Fleet) {
	g := &FullGame{Game: &Game{}, Universe: &Universe{}}
	p := &Player{}
	p.Num = 1
	p.Automation.FleetRoles = map[int]string{}
	rng := rand.New(rand.NewSource(1113))
	for i := 0; i < 40; i++ {
		planet := &Planet{}
		planet.PlayerNum = 1
		planet.Position = Vector{rng.Intn(2000), rng.Intn(2000)}
		if i%3 == 0 {
			planet.Spec.DockCapacity = 100
		}
		g.Planets = append(g.Planets, planet)
	}
	for i := 0; i < 160; i++ {
		f := &Fleet{}
		f.Num = i + 1
		f.PlayerNum = 1
		f.Position = Vector{rng.Intn(2000), rng.Intn(2000)}
		f.Waypoints = []Waypoint{{Position: f.Position}, {Position: Vector{rng.Intn(2000), rng.Intn(2000)}}, {Position: f.Position}}
		p.Automation.FleetRoles[f.Num] = "transport"
		if i%4 == 0 {
			p.Automation.FleetRoles[f.Num] = "tanker"
			writeTankerJob(f, &automaticTankerJob{Stage: "patrol", Anchor: Vector{rng.Intn(2000), rng.Intn(2000)}})
		}
		if i%13 == 0 {
			f.Delete = true
		}
		if i%17 == 0 {
			f.Starbase = true
		}
		if i%19 == 0 {
			delete(p.Automation.FleetRoles, f.Num)
		}
		g.Fleets = append(g.Fleets, f)
	}
	return &turnGenerator{game: g}, p, g.Fleets[4]
}
func TestTankerAnchorScoreExactAndRefreshed(t *testing.T) {
	gen, p, tanker := tankerScoreFixture()
	for pass := 0; pass < 3; pass++ {
		score := gen.tankerAnchorScorer(p, tanker)
		for x := -50; x <= 2050; x += 37 {
			for y := -50; y <= 2050; y += 79 {
				point := Vector{x, y}
				got, want := score(point), gen.referenceTankerAnchorValue(p, tanker, point)
				if math.Float64bits(got) != math.Float64bits(want) {
					t.Fatalf("pass %d point %v: %.18g != %.18g", pass, point, got, want)
				}
			}
		}
		writeTankerJob(gen.game.Fleets[8], &automaticTankerJob{Stage: "patrol", Anchor: Vector{pass * 700, pass * 900}})
		gen.game.Fleets[9].Waypoints[1].Position = Vector{pass * 200, 300}
		delete(p.Automation.FleetRoles, 10)
	}
	gen.game.Planets = nil
	gen.game.Fleets = nil
	if got := gen.tankerAnchorScorer(p, tanker)(Vector{}); got != 1 {
		t.Fatal(got)
	}
}
func BenchmarkTankerAnchorScore(b *testing.B) {
	gen, p, tanker := tankerScoreFixture()
	for _, optimized := range []bool{false, true} {
		name := "original"
		if optimized {
			name = "prepared"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				score := func(v Vector) float64 { return gen.referenceTankerAnchorValue(p, tanker, v) }
				if optimized {
					score = gen.tankerAnchorScorer(p, tanker)
				}
				for n := 0; n < 500; n++ {
					score(Vector{n * 3, n * 2})
				}
			}
		})
	}
}
