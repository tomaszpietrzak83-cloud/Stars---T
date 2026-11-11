//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A deliberately small world: colonies, nine fields and three sweepers only.
func TestMinesweeperTerritoryFrontAdvancesWithoutDuplicateAssignments(t *testing.T) {
	d := testSweeperDesign()
	d.Slots = append(d.Slots, ShipDesignSlot{HullSlotIndex: 4, HullComponent: EagleEyeScanner.Name, Quantity: 1}, ShipDesignSlot{HullSlotIndex: 6, HullComponent: FuelTank.Name, Quantity: 1})
	u := newTestUniverse(t, TestScenario{Name: "Outward mine clearing", Players: []ScenarioPlayer{
		{Designs: Designs(d, DesignSpaceStation), Fleets: []ScenarioFleet{
			{Name: "North", Design: d.Name, Position: Vector{0, 0}},
			{Name: "Middle", Design: d.Name, Position: Vector{0, 80}},
			{Name: "South", Design: d.Name, Position: Vector{0, 160}},
		}, Relations: []PlayerRelationship{{Relation: PlayerRelationFriend}, {Relation: PlayerRelationFriend}, {Relation: PlayerRelationEnemy}}},
		{}, {},
	}, Planets: []ScenarioPlanet{
		{Name: "Home", Owner: 1, Cargo: Cargo{Colonists: 1000}, Position: Vector{0, 0}, Starbase: DesignSpaceStation.Name},
		{Name: "Ally Middle", Owner: 2, Cargo: Cargo{Colonists: 1000}, Position: Vector{0, 80}},
		{Name: "Ally South", Owner: 2, Cargo: Cargo{Colonists: 1000}, Position: Vector{0, 160}},
		{Name: "Enemy", Owner: 3, Cargo: Cargo{Colonists: 1000}, Position: Vector{300, 80}},
	}})
	p := u.Player(1)
	p.TechLevels.Electronics = 14
	p.TechLevels.Energy = 6
	require.NoError(t, u.Game.computeSpecs())
	p.PlanetIntels = nil
	for _, planet := range u.Game.Planets {
		report := *planet
		p.PlanetIntels = append(p.PlanetIntels, &report)
	}
	p.Automation.FleetRoles = map[int]string{}
	for _, f := range u.Game.Fleets {
		if f.PlayerNum == 1 {
			p.Automation.FleetRoles[f.Num] = "minesweeper"
		}
	}
	for layer, x := range []int{45, 115, 185} {
		for lane, y := range []int{0, 80, 160} {
			m := newMinefield(u.Player(3), MinefieldTypeStandard, 225, layer*3+lane+1, Vector{x, y})
			u.Game.Universe.addMinefield(m)
			report := *m
			p.MinefieldIntels = append(p.MinefieldIntels, &report)
		}
	}
	output := os.Getenv("STARS_SWEEP_WORLD_OUTPUT")
	save := func(name string, v any) {
		if output != "" {
			require.NoError(t, os.MkdirAll(output, 0755))
			raw, e := json.MarshalIndent(v, "", "  ")
			require.NoError(t, e)
			require.NoError(t, os.WriteFile(filepath.Join(output, name), raw, 0644))
		}
	}
	save("initial-world.json", u.Game)
	totals := map[int]int{}
	sweptFields := map[int]bool{}
	observations := []any{}
	clearedAt := map[int]int{}
	for step := 1; step <= 24; step++ {
		p.Messages = nil
		require.NoError(t, NewGamer().GenerateTurn(u.Game.Game, u.Game.Universe, u.Game.Players))
		active := map[string]int{}
		for _, f := range u.Game.Fleets {
			if f.PlayerNum == 1 {
				target := f.GetTag(minesweeperTargetTag)
				if target != "" {
					require.Zero(t, active[target], "two fleets assigned to %s", target)
					active[target] = f.Num
				}
				require.False(t, f.InAutomaticSafety())
				require.GreaterOrEqual(t, f.Fuel, 0)
			}
		}
		for _, m := range p.Messages {
			if m.Type == PlayerMessageFleetMinefieldSweptMines {
				totals[m.TargetNum] += m.Spec.Amount
				sweptFields[m.Spec.TargetNum] = true
			}
		}
		remaining := map[int]int{}
		for _, m := range u.Game.Minefields {
			if !m.Delete && m.NumMines > 0 {
				remaining[(m.Num-1)/3] += m.NumMines
			}
		}
		for layer := 0; layer < 3; layer++ {
			if remaining[layer] == 0 && clearedAt[layer] == 0 {
				clearedAt[layer] = step
			}
		}
		for key := range active {
			var owner, num int
			_, err := fmt.Sscanf(key, "%d:%d", &owner, &num)
			require.NoError(t, err)
			layer := (num - 1) / 3
			for earlier := 0; earlier < layer; earlier++ {
				require.NotZero(t, clearedAt[earlier], "outer field assigned before the inner layer was cleared")
			}
		}
		raw, e := json.Marshal(map[string]any{"step": step, "year": u.Game.Year, "fleets": u.Game.Fleets, "fields": u.Game.Minefields, "messages": p.Messages})
		require.NoError(t, e)
		observations = append(observations, json.RawMessage(raw))
		t.Logf("step=%d remaining=%v totals=%v", step, remaining, totals)
		for _, f := range u.Game.Fleets {
			t.Logf("fleet=%s position=%v tags=%v", f.Name, f.Position, f.Tags)
		}
		u.Game.Minefields = slices.DeleteFunc(u.Game.Minefields, func(m *Minefield) bool { return m.Delete })
		if step == 5 {
			raw, e := json.Marshal(u.Game)
			require.NoError(t, e)
			var restored FullGame
			require.NoError(t, json.Unmarshal(raw, &restored))
			restored.TechStore = &StaticTechStore
			restored.Rules.SetTechStore(&StaticTechStore)
			restored.Rules.ResetSeed(1)
			restored.Universe.setLogger(testLogger)
			require.NoError(t, restored.computeSpecs())
			u.Game = &restored
			p = u.Player(1)
		}
	}
	require.Len(t, clearedAt, 3)
	require.Less(t, clearedAt[0], clearedAt[1])
	require.Less(t, clearedAt[1], clearedAt[2])
	require.Len(t, totals, 3)
	require.Len(t, sweptFields, 9, "every field must be swept, not just disappear through decay")
	total := 0
	for _, n := range totals {
		require.Positive(t, n)
		total += n
	}
	require.Positive(t, total) // Full turns also apply natural mine decay.
	require.LessOrEqual(t, total, 2025)
	save("turns.json", observations)
	save("result.json", map[string]any{"turns": 24, "removed": totals, "clearedAt": clearedAt})
}

func TestMinesweeperChoosesTerritoryBeforeFleetProximity(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f := u.Player(1), u.Fleet("Sweeper")
	f.Position = Vector{180, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}}
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	for i, x := range []int{45, 180} {
		m := newMinefield(u.Player(2), MinefieldTypeStandard, 100, i+1, Vector{x, 0})
		u.Game.Universe.addMinefield(m)
		report := *m
		p.MinefieldIntels = append(p.MinefieldIntels, &report)
	}
	u.Run((*turnGenerator).planMinesweeperAutomation)
	require.Equal(t, "2:1", f.GetTag(minesweeperTargetTag), "protect home first even when the fleet is already at a remote field")
}

func TestMinesweeperReservesFieldAndKeepsAssignment(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f := u.Player(1), u.Fleet("Sweeper")
	spare := *f
	spare.Num = 99
	spare.Name = "Reserve"
	spare.Position = Vector{200, 0}
	spare.Tags = Tags{}
	spare.Waypoints = []Waypoint{{Position: spare.Position}}
	u.Game.Fleets = append(u.Game.Fleets, &spare)
	require.NoError(t, u.Game.Universe.addFleet(&spare))
	p.Automation.FleetRoles[spare.Num] = "minesweeper"
	m := newMinefield(u.Player(2), MinefieldTypeStandard, 100, 1, Vector{100, 0})
	u.Game.Universe.addMinefield(m)
	report := *m
	p.MinefieldIntels = []*Minefield{&report}
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	u.Run((*turnGenerator).planMinesweeperAutomation)
	require.Equal(t, "2:1", f.GetTag(minesweeperTargetTag))
	require.Empty(t, spare.GetTag(minesweeperTargetTag))
	require.Len(t, spare.Waypoints, 1)
	require.Contains(t, spare.GetTag(fuelStatusTag), "reserve")
	spare.Position = Vector{99, 0}
	u.Run((*turnGenerator).planMinesweeperAutomation)
	require.Equal(t, "2:1", f.GetTag(minesweeperTargetTag), "closer newcomer must not steal an ongoing job")
	require.Empty(t, spare.GetTag(minesweeperTargetTag))
}
