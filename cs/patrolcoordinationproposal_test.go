//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// Offline coordination prototype, deliberately not called by turn generation.
type proposalTask struct {
	Key, Kind                      string
	Tier, Deadline, Created, Value int
	Assigned                       int
}
type proposalAgent struct {
	Num          int
	Capabilities map[string]bool
	Locked       bool
}

func proposalDispatch(tasks []proposalTask, agents []proposalAgent, year int) []proposalTask {
	// Deduplicate the same task; target search and debris collection have separate keys.
	unique := map[string]proposalTask{}
	for _, task := range tasks {
		old, exists := unique[task.Key]
		if !exists || task.Tier < old.Tier {
			unique[task.Key] = task
		}
	}
	result := []proposalTask{}
	for _, task := range unique {
		result = append(result, task)
	}
	priority := func(t proposalTask) int {
		if t.Tier == 0 {
			return 0
		}
		return max(1, t.Tier-min(1, max(0, year-t.Created)/4))
	}
	slices.SortFunc(result, func(a, b proposalTask) int {
		for _, diff := range []int{priority(a) - priority(b), a.Deadline - b.Deadline, b.Value - a.Value, a.Created - b.Created} {
			if diff != 0 {
				return diff
			}
		}
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	used := map[int]bool{}
	for i := range result {
		result[i].Assigned = 0
		for _, agent := range agents {
			if !agent.Locked && !used[agent.Num] && agent.Capabilities[result[i].Kind] {
				result[i].Assigned = agent.Num
				used[agent.Num] = true
				break
			}
		}
	}
	return result
}

type proposalMemory struct {
	Enemy, Target, Seen, LastSearch int
	Position                        Vector
	Velocity                        VectorFloat64
	MaxSpeed                        float64
	State                           string
	WeakSlow                        bool
	Misses                          int
}

func (m *proposalMemory) searchArea(year int) (Vector, float64) {
	age := float64(max(0, year-m.Seen))
	return m.Position.ToFloat64().Add(m.Velocity.Scale(age)).ToInt(true), age * (m.MaxSpeed + m.Velocity.Length())
}
func (m *proposalMemory) next(fresh, confirmedDestroyed, realDebris bool) string {
	if confirmedDestroyed {
		m.State = "destroyed"
	}
	if m.State == "destroyed" || m.State == "collect" {
		if realDebris {
			m.State = "collect"
			return "collect"
		}
		m.State = "done"
		return ""
	}
	if fresh {
		m.State = "reassess"
		return "intercept-after-evaluation"
	}
	m.State = "search"
	return "search"
}

func TestPatrolCoordinationSharedQueue(t *testing.T) {
	tasks := []proposalTask{
		{Key: "planet:7", Kind: "scan", Tier: 4, Deadline: 20, Created: 10},
		{Key: "incoming:2:9", Kind: "contact", Tier: 1, Deadline: 12, Created: 10},
		{Key: "lost:2:8", Kind: "search", Tier: 3, Deadline: 16, Created: 10},
		{Key: "large:2:5", Kind: "attack", Tier: 2, Deadline: 14, Created: 10},
		{Key: "debris:4", Kind: "collect", Tier: 4, Deadline: 13, Created: 10},
		{Key: "incoming:2:9", Kind: "contact", Tier: 1, Deadline: 12, Created: 10},
	}
	scouts := map[string]bool{"scan": true, "contact": true, "search": true}
	agents := []proposalAgent{{1, scouts, false}, {2, scouts, false}, {3, map[string]bool{"attack": true}, false}, {4, map[string]bool{"collect": true}, false}, {5, scouts, true}}
	assigned := proposalDispatch(tasks, agents, 11)
	require.Len(t, assigned, 5)
	byKey := map[string]proposalTask{}
	for _, task := range assigned {
		byKey[task.Key] = task
	}
	require.Equal(t, 1, byKey["incoming:2:9"].Assigned)
	require.Equal(t, 2, byKey["lost:2:8"].Assigned)
	require.Equal(t, 3, byKey["large:2:5"].Assigned)
	require.Equal(t, 4, byKey["debris:4"].Assigned)
	require.Zero(t, byKey["planet:7"].Assigned)
	t.Log("QUEUE: incoming contact and lost-target search beat planet refresh; attack and collection proceed in parallel; locked scout untouched")
}

func TestPatrolCoordinationTargetMemorySurvivesReload(t *testing.T) {
	original := proposalMemory{Enemy: 2, Target: 9, Seen: 10, Position: Vector{100, 0}, Velocity: VectorFloat64{4, 0}, MaxSpeed: 36, WeakSlow: true}
	raw, err := json.Marshal(original)
	require.NoError(t, err)
	var memory proposalMemory
	require.NoError(t, json.Unmarshal(raw, &memory))
	require.Equal(t, "search", memory.next(false, false, false))
	center, radius := memory.searchArea(12)
	require.Equal(t, Vector{108, 0}, center)
	require.Equal(t, 80., radius)
	// Slow observation must not become proof of an empty tank or an immutable speed.
	require.Greater(t, radius, 8.)
	require.Equal(t, "intercept-after-evaluation", memory.next(true, false, false))
	require.Equal(t, "collect", memory.next(false, true, true))
	require.Empty(t, memory.next(false, false, false))
	require.Equal(t, "done", memory.State)
	t.Log("MEMORY: reload -> uncertain search region -> fresh contact and reassessment -> confirmed kill -> real debris -> completion")
}

func TestPatrolCoordinationScoutSearchUsesRealRouteAndScan(t *testing.T) {
	scout, _ := proposalUtilityDesigns()
	enemy := generatedPatrolDesign(Privateer.Name, 0, 771)
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Player: proposalTestPlayer(), Designs: Designs(scout), Fleets: []ScenarioFleet{{Name: "Searcher", Design: scout.Name}}},
		{Player: proposalTestPlayer(), Designs: Designs(enemy), Fleets: []ScenarioFleet{{Name: "Lost target", Design: enemy.Name, Position: Vector{108, 0}}}},
	}})
	p, f, e := u.Player(1), u.Fleet("Searcher"), u.Fleet("Lost target")
	p.FleetIntels = nil
	p.ShipDesignIntels = nil
	p.shipIntelIndex = nil
	addExpeditionTestDock(p, Vector{})
	memory := proposalMemory{Enemy: 2, Target: e.Num, Seen: 10, Position: Vector{100, 0}, Velocity: VectorFloat64{4, 0}, MaxSpeed: 36, WeakSlow: true}
	require.Equal(t, "search", memory.next(false, false, false))
	point, _ := memory.searchArea(12)
	// Route comes exclusively from the remembered report, not real enemy position.
	route := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{f}, point, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 20, MinWarp: 4, MaxWarp: 4})
	require.NotNil(t, route)
	require.False(t, route.WaitForFuel)
	applyExpeditionRoute(p, f, route)
	for i := 0; i < route.Turns; i++ {
		u.Run((*turnGenerator).fleetMove)
	}
	scan := newPlayerScanner(u.Game.Universe, u.Game.Players, &u.Game.Rules, p)
	require.True(t, scan.fleetInScannerRange(e, scanner{Position: f.Position, Range: f.Spec.ScanRange, RangePen: f.Spec.ScanRangePen, CloakReductionFactor: f.Spec.ReduceCloaking}))
	if p.discoverer == nil {
		p.discoverer = newDiscoverer(testLogger, p)
	}
	p.discoverer.discoverFleet(e, false)
	require.NotNil(t, p.GetFleetIntel(2, e.Num))
	require.Equal(t, "intercept-after-evaluation", memory.next(true, false, false))
	t.Logf("SEARCH: remembered target -> funded %d-turn scout route -> actual scanner detection -> fresh report and attack reassessment", route.Turns)
}

type proposalRally struct {
	Position    Vector
	Turns, Fuel int
	Routes      []*expeditionRoute
}

func proposalChooseRally(rules *Rules, p *Player, group []*Fleet, target Vector, candidates []Vector, warps []int) *proposalRally {
	var best *proposalRally
	for _, point := range candidates {
		candidate := &proposalRally{Position: point}
		merged := *group[0]
		merged.Position = point
		merged.Tokens = nil
		merged.Fuel = 0
		merged.Cargo = Cargo{}
		feasible := true
		for i, f := range group {
			policy := automaticRoutePolicy{AllowThreat: true, Fast: true, MinWarp: warps[i], MaxWarp: warps[i], MaxTurns: 20}
			route := planAutomaticRoute(rules, p, []*Fleet{f}, point, MapObjectTarget{}, policy)
			if route == nil || route.WaitForFuel {
				feasible = false
				break
			}
			candidate.Routes = append(candidate.Routes, route)
			candidate.Turns = max(candidate.Turns, route.Turns)
			candidate.Fuel += route.Fuel
			merged.Tokens = append(merged.Tokens, f.Tokens...)
			merged.Cargo = merged.Cargo.Add(f.Cargo)
			arrival := f.Fuel - route.Fuel
			if len(route.ArrivalFuel) > 0 {
				arrival = route.ArrivalFuel[0]
			}
			merged.Fuel += max(0, arrival)
		}
		if !feasible {
			continue
		}
		merged.Spec = ComputeFleetSpec(rules, p, &merged)
		attack := planAutomaticRoute(rules, p, []*Fleet{&merged}, target, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true, Fast: true, MinWarp: 9, MaxWarp: 9, MaxTurns: 20})
		if attack == nil || attack.WaitForFuel {
			continue
		}
		candidate.Turns += attack.Turns
		candidate.Fuel += attack.Fuel
		if best == nil || candidate.Turns < best.Turns || candidate.Turns == best.Turns && candidate.Fuel < best.Fuel {
			best = candidate
		}
	}
	return best
}

func TestPatrolCoordinationNativeRallySixToFour(t *testing.T) {
	// New configurations built directly from components, no preset ship designs.
	slow := ShipDesign{Name: "Rally-slow", Hull: Destroyer.Name, Slots: []ShipDesignSlot{
		{HullSlotIndex: 1, HullComponent: FuelMizer.Name, Quantity: 1},
		{HullSlotIndex: 2, HullComponent: XRayLaser.Name, Quantity: 1},
	}}
	fast := slow
	fast.Name = "Rally-fast"
	fast.Slots = slices.Clone(slow.Slots)
	fast.Slots[0].HullComponent = TransStar10.Name
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Player: proposalTestPlayer(), Designs: Designs(slow, fast), Fleets: []ScenarioFleet{
		{Name: "Slow", Design: slow.Name}, {Name: "Fast", Design: fast.Name, Position: Vector{100, 0}},
	}}}})
	p := u.Player(1)
	group := []*Fleet{u.Fleet("Slow"), u.Fleet("Fast")}
	addExpeditionTestDock(p, Vector{})
	addExpeditionTestDock(p, Vector{140, 0})
	before, err := json.Marshal(group)
	require.NoError(t, err)
	midpoint := proposalChooseRally(&u.Game.Rules, p, group, Vector{140, 0}, []Vector{{50, 0}}, []int{4, 9})
	require.NotNil(t, midpoint)
	candidates := []Vector{}
	for x := 0; x <= 100; x += 4 {
		candidates = append(candidates, Vector{x, 0})
	}
	chosen := proposalChooseRally(&u.Game.Rules, p, group, Vector{140, 0}, candidates, []int{4, 9})
	require.NotNil(t, chosen)
	require.Equal(t, 6, midpoint.Turns)
	require.Equal(t, 4, chosen.Turns)
	after, err := json.Marshal(group)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
	require.LessOrEqual(t, chosen.Fuel, group[0].Fuel+group[1].Fuel)
	// Verify funded gathering legs by actual native movement, then native merge.
	for i, f := range group {
		applyExpeditionRoute(p, f, chosen.Routes[i])
	}
	gatherTurns := 0
	for _, route := range chosen.Routes {
		gatherTurns = max(gatherTurns, route.Turns)
	}
	for i := 0; i < gatherTurns; i++ {
		u.Run((*turnGenerator).fleetMove)
	}
	require.Equal(t, chosen.Position, group[0].Position)
	require.Equal(t, chosen.Position, group[1].Position)
	_, err = NewOrderer().Merge(&u.Game.Rules, p, group)
	require.NoError(t, err)
	u.Game.deleteFleet(group[1])
	attack := planAutomaticRoute(&u.Game.Rules, p, []*Fleet{group[0]}, Vector{140, 0}, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true, Fast: true, MinWarp: 9, MaxWarp: 9, MaxTurns: 20})
	require.NotNil(t, attack)
	applyExpeditionRoute(p, group[0], attack)
	for i := 0; i < attack.Turns; i++ {
		u.Run((*turnGenerator).fleetMove)
	}
	require.Equal(t, Vector{140, 0}, group[0].Position)
	require.Equal(t, 4, gatherTurns+attack.Turns)
	t.Logf("NATIVE_RALLY midpoint=%d optimized=%d position=%v funded_fuel=%d", midpoint.Turns, chosen.Turns, chosen.Position, chosen.Fuel)
}
