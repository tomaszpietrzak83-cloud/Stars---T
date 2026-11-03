//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Offline prototype only. None of these policies are wired into the game.
func proposalDesigns() []ShipDesign { return generatedPatrolDesigns() }

func proposalBattle(own, enemy ShipDesign, n, count int, seed int64) (ownLost, enemyLost int) {
	scenario := TestScenario{Players: []ScenarioPlayer{
		{Player: proposalTestPlayer(), Designs: Designs(own), Fleets: []ScenarioFleet{{Name: "Patrol", Design: own.Name, Quantity: n}}},
		{Player: proposalTestPlayer(), Designs: Designs(enemy), Fleets: []ScenarioFleet{{Name: "Enemy", Design: enemy.Name, Quantity: count}}},
	}}
	ownPlayer, enemyPlayer := 1, 2
	if seed%2 != 0 {
		scenario.Players[0], scenario.Players[1] = scenario.Players[1], scenario.Players[0]
		ownPlayer, enemyPlayer = 2, 1
	}
	g := BuildScenario(scenario)
	p, q := g.Players[0], g.Players[1]
	p.Relations = []PlayerRelationship{{Relation: PlayerRelationFriend}, {Relation: PlayerRelationEnemy}}
	q.Relations = []PlayerRelationship{{Relation: PlayerRelationEnemy}, {Relation: PlayerRelationFriend}}
	g.Rules.ResetSeed(seed)
	b := newBattler(slog.New(slog.NewTextHandler(io.Discard, nil)), &g.Rules, 1, map[int]*Player{1: p, 2: q}, g.Fleets, nil)
	r := b.runBattle()
	return r.Stats.ShipsDestroyedByPlayer[ownPlayer], r.Stats.ShipsDestroyedByPlayer[enemyPlayer]
}

// Complete destruction of the enemy, no own wipeout, mean resource loss <=25%.
// Same-design homogeneous fleets make ship fraction equal resource fraction.
func proposalAccept(own, enemy ShipDesign, n, count int) bool {
	return proposalAcceptSeeds(own, enemy, n, count, 16)
}

func proposalAcceptSeeds(own, enemy ShipDesign, n, count, samples int) bool {
	lost := 0
	for seed := int64(0); seed < int64(samples); seed++ {
		a, b := proposalBattle(own, enemy, n, count, seed)
		if a == n || b < count {
			return false
		}
		lost += a
	}
	return float64(lost)/float64(n*samples) <= .25
}

func TestPatrolProposalComponentMatrix(t *testing.T) {
	started := time.Now()
	designs := proposalDesigns()
	// Verify physical component placement independently of technology availability.
	for _, d := range designs {
		h := StaticTechStore.GetHull(d.Hull)
		for _, slot := range d.Slots {
			component := StaticTechStore.GetHullComponent(slot.HullComponent)
			require.NotNil(t, component)
			require.NotZero(t, h.Slots[slot.HullSlotIndex-1].Type&component.HullSlotType, d.Name)
			require.LessOrEqual(t, slot.Quantity, h.Slots[slot.HullSlotIndex-1].Capacity)
		}
	}
	cases, oldWiped, oldUnfinished, newWiped, newUnfinished, accepted, rejected := 0, 0, 0, 0, 0, 0, 0
	for _, own := range designs {
		for _, enemy := range designs {
			for _, count := range []int{1, 5, 20} {
				probe := BuildScenario(TestScenario{Players: []ScenarioPlayer{
					{Player: proposalTestPlayer(), Designs: Designs(own), Fleets: []ScenarioFleet{{Name: "Own", Design: own.Name}}},
					{Player: proposalTestPlayer(), Designs: Designs(enemy), Fleets: []ScenarioFleet{{Name: "Enemy", Design: enemy.Name, Quantity: count}}},
				}})
				a, b := probe.Fleets[0], probe.Fleets[1]
				require.Positive(t, patrolPower(a))
				oldN := (patrolPower(b)*3/2 + 1 + patrolPower(a) - 1) / patrolPower(a)
				selected := 0
				for _, n := range []int{1, 2, 3, 4, 6, 8, 12, 16, 24, 32, 48, 64} {
					if proposalAccept(own, enemy, n, count) {
						buffered := (5*n + 3) / 4
						if proposalAcceptSeeds(own, enemy, buffered, count, 64) {
							selected = buffered
							break
						}
					}
				}
				ow, ou, nw, nu, lossSum := 0, 0, 0, 0, 0
				for seed := int64(2000); seed < 2032; seed++ {
					x, y := proposalBattle(own, enemy, oldN, count, seed)
					if x == oldN {
						ow++
					}
					if y < count {
						ou++
					}
					if selected > 0 {
						x, y = proposalBattle(own, enemy, selected, count, seed)
						if x == selected {
							nw++
						}
						if y < count {
							nu++
						}
						lossSum += x
					}
				}
				if selected > 0 {
					accepted++
				} else {
					rejected++
				}
				cases++
				oldWiped += ow
				oldUnfinished += ou
				newWiped += nw
				newUnfinished += nu
				fraction := 0.
				if selected > 0 {
					fraction = float64(lossSum) / float64(32*selected)
				}
				t.Logf("PROPOSAL own=%s enemy=%s count=%d old_n=%d new_n=%d old_wiped=%d old_unfinished=%d new_wiped=%d new_unfinished=%d new_loss=%.4f", own.Name, enemy.Name, count, oldN, selected, ow, ou, nw, nu, fraction)
			}
		}
	}
	require.Equal(t, len(designs)*len(designs)*3, cases)
	require.Less(t, newWiped, oldWiped)
	t.Logf("SUMMARY cases=%d accepted=%d rejected=%d heldout_per_case=32 old_wiped=%d old_unfinished=%d new_wiped=%d new_unfinished=%d elapsed=%s", cases, accepted, rejected, oldWiped, oldUnfinished, newWiped, newUnfinished, time.Since(started))
}

// Reinforcements count only with a visible course converging on the target.
// Relative closest approach also handles a moving target, without private orders.
func proposalConverges(position, velocity, target, targetVelocity VectorFloat64, horizon, radius float64) bool {
	dx, dy := target.X-position.X, target.Y-position.Y
	if velocity.X*dx+velocity.Y*dy <= 0 {
		return false
	}
	vx, vy := velocity.X-targetVelocity.X, velocity.Y-targetVelocity.Y
	speed2 := vx*vx + vy*vy
	if speed2 == 0 {
		return false
	}
	at := (dx*vx + dy*vy) / speed2
	if at < 0 || at > horizon {
		return false
	}
	return math.Hypot(dx-vx*at, dy-vy*at) <= radius
}

func TestPatrolProposalReinforcementCourses(t *testing.T) {
	for _, c := range []struct {
		name             string
		p, v, target, tv VectorFloat64
		h                float64
		want             bool
	}{
		{"incoming", VectorFloat64{40, 0}, VectorFloat64{-20, 0}, VectorFloat64{}, VectorFloat64{}, 2, true},
		{"outgoing", VectorFloat64{10, 0}, VectorFloat64{20, 0}, VectorFloat64{}, VectorFloat64{}, 2, false},
		{"stationary", VectorFloat64{1, 0}, VectorFloat64{}, VectorFloat64{}, VectorFloat64{}, 2, false},
		{"passing", VectorFloat64{10, 0}, VectorFloat64{0, 20}, VectorFloat64{}, VectorFloat64{}, 2, false},
		{"late", VectorFloat64{100, 0}, VectorFloat64{-20, 0}, VectorFloat64{}, VectorFloat64{}, 2, false},
		{"moving meeting", VectorFloat64{40, 0}, VectorFloat64{-10, 0}, VectorFloat64{}, VectorFloat64{10, 0}, 2, true},
		{"target escapes", VectorFloat64{40, 0}, VectorFloat64{-10, 0}, VectorFloat64{}, VectorFloat64{-20, 0}, 2, false},
	} {
		t.Run(c.name, func(t *testing.T) { require.Equal(t, c.want, proposalConverges(c.p, c.v, c.target, c.tv, c.h, 3)) })
	}
}

func TestPatrolProposalZeroFuelEngines(t *testing.T) {
	for _, component := range []string{LongHump6.Name, FuelMizer.Name, TransStar10.Name, SubGalacticFuelScoop.Name} {
		d := ShipDesign{Name: fmt.Sprintf("Engine %s", component), Hull: Destroyer.Name, Slots: []ShipDesignSlot{
			{HullSlotIndex: 1, HullComponent: component, Quantity: 1},
			{HullSlotIndex: 2, HullComponent: XRayLaser.Name, Quantity: 1},
		}}
		g := BuildScenario(TestScenario{Players: []ScenarioPlayer{{Designs: Designs(d), Fleets: []ScenarioFleet{{Name: "Patrol", Design: d.Name}}}}})
		f, p := g.Fleets[0], g.Players[0]
		f.Fuel = 0
		free := f.Spec.Engine.FreeSpeed
		require.Zero(t, f.GetFuelCost(p, free, 100))
		t.Logf("ENGINE name=%s free_warp=%d free_ly=%.0f warp6_cost100=%d warp9_cost100=%d", component, free, f.StrategicSpeed(free), f.GetFuelCost(p, 6, 100), f.GetFuelCost(p, 9, 100))
		f.Waypoints = append(f.Waypoints, NewPositionWaypoint(Vector{100, 0}, free))
		tg := newTurnGenerator(g)
		tg.fleetMove()
		require.Equal(t, Vector{int(f.StrategicSpeed(free)), 0}, f.Position)
		require.Equal(t, f.getFuelGeneration(free, f.StrategicSpeed(free)), f.Fuel)
		t.Logf("ENGINE_MOVED name=%s distance=%d starting_fuel=0 ending_fuel=%d", component, f.Position.X, f.Fuel)
	}
}

func TestPatrolProposalRendezvousMinimizesTimeToBattle(t *testing.T) {
	// Two gathering fleets at x=0/100, cruising 16/81 ly/year; target x=140.
	// Attack speed after merging is 81 ly/year. Fuel/hazards must be checked
	// separately by the real route planner before accepting a candidate.
	timeToBattle := func(x float64) int {
		gather := max(math.Ceil(x/16), math.Ceil((100-x)/81))
		return int(gather + math.Ceil((140-x)/81))
	}
	best, point := 1000, 0
	for x := 0; x <= 100; x++ {
		if turns := timeToBattle(float64(x)); turns < best {
			best, point = turns, x
		}
	}
	require.Equal(t, 6, timeToBattle(50))
	require.Equal(t, 4, best)
	t.Logf("RALLY arithmetic mean: 6 turns to battle; candidate search: %d turns, x=%d", best, point)
}

func TestPatrolProposalIncomingReconPrecedesPlanetRefresh(t *testing.T) {
	scout, dock := proposalUtilityDesigns()
	enemy := generatedPatrolDesign(Cruiser.Name, 0, 991)
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Player: proposalTestPlayer(), Designs: Designs(scout, dock), Fleets: []ScenarioFleet{{Name: "Scout", Design: scout.Name, At: "Home"}}},
		{Player: proposalTestPlayer(), Designs: Designs(enemy), Fleets: []ScenarioFleet{{Name: "Incoming", Design: enemy.Name, Position: Vector{20, 0}}}},
	}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: dock.Name}, {Name: "Priority", Owner: 2, Position: Vector{40, 40}}}})
	p, f, e := u.Player(1), u.Fleet("Scout"), u.Fleet("Incoming")
	p.Race.Spec.DiscoverDesignOnScan = false
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.Automation.PriorityPlanets = map[int]bool{u.Planet("Priority").Num: true}
	planet := *u.Planet("Priority")
	planet.ReportAge = 6
	p.PlanetIntels = []*Planet{u.Planet("Home"), &planet}
	intel := *e
	intel.Heading = VectorFloat64{-1, 0}
	intel.WarpSpeed = 5
	p.FleetIntels = []*Fleet{&intel}
	p.ShipDesignIntels = nil
	p.shipIntelIndex = nil
	u.Run(func(tg *turnGenerator) { require.True(t, tg.planScoutRecon(p, f)) })
	require.True(t, scoutReconActive(p, f))
	u.Run((*turnGenerator).planScoutAutomation)
	require.Empty(t, f.GetTag(priorityReconTag))
	require.True(t, scoutReconActive(p, f))
	t.Log("Historical priority defect fixed: incoming fleet contact retains the scout before planet refresh")
}

func TestPatrolProposalScoutTransmitsDespiteDestruction(t *testing.T) {
	scout, _ := proposalUtilityDesigns()
	enemy := ShipDesign{Name: "Contact interceptor", Hull: Destroyer.Name, Slots: []ShipDesignSlot{
		{HullSlotIndex: 1, HullComponent: TransStar10.Name, Quantity: 1},
		{HullSlotIndex: 2, HullComponent: "Anti-Matter Pulverizer", Quantity: 1},
		{HullSlotIndex: 3, HullComponent: "Anti-Matter Pulverizer", Quantity: 1},
		{HullSlotIndex: 6, HullComponent: Overthruster.Name, Quantity: 1},
	}}
	require.NoError(t, enemy.Validate(&rules, proposalTestPlayer()))
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Player: proposalTestPlayer(), Designs: Designs(scout), Fleets: []ScenarioFleet{{Name: "Scout", Design: scout.Name}}},
		{Player: proposalTestPlayer(), Designs: Designs(enemy), Fleets: []ScenarioFleet{{Name: "Enemy", Design: enemy.Name}}},
	}})
	p, q, f, e := u.Player(1), u.Player(2), u.Fleet("Scout"), u.Fleet("Enemy")
	p.Race.Spec.DiscoverDesignOnScan = false
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.ShipDesignIntels = nil
	p.shipIntelIndex = nil
	f.SetTag(scoutReconTag, fmt.Sprintf(`{"Enemy":2,"Target":%d,"status":"contact"}`, e.Num))
	u.Run((*turnGenerator).scanScoutReconContacts)
	require.NotEmpty(t, p.GetShipDesignIntel(2, e.Tokens[0].DesignNum).Slots)
	p.Relations = []PlayerRelationship{{Relation: PlayerRelationFriend}, {Relation: PlayerRelationEnemy}}
	q.Relations = []PlayerRelationship{{Relation: PlayerRelationEnemy}, {Relation: PlayerRelationFriend}}
	u.Game.Rules.ResetSeed(0)
	b := newBattler(slog.New(slog.NewTextHandler(io.Discard, nil)), &u.Game.Rules, 1, map[int]*Player{1: p, 2: q}, []*Fleet{f, e}, nil)
	r := b.runBattle()
	require.Equal(t, 1, r.Stats.ShipsDestroyedByPlayer[1])
	require.NotEmpty(t, p.GetShipDesignIntel(2, e.Tokens[0].DesignNum).Slots)
	t.Log("scout destroyed in native battle; discovered enemy equipment remains available")
}
