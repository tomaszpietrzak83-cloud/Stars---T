//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func shipProductionUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignLongRangeScout, DesignSpaceStation)}}, Planets: []ScenarioPlanet{
		{Name: "Alpha", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500, Ironium: 10000, Boranium: 10000, Germanium: 10000}},
		{Name: "Beta", Owner: 1, Starbase: DesignSpaceStation.Name, Position: Vector{100, 0}, Cargo: Cargo{Colonists: 1500, Ironium: 10000, Boranium: 10000, Germanium: 10000}},
		{Name: "Colony", Owner: 1, Position: Vector{200, 0}, Cargo: Cargo{Colonists: 2500, Ironium: 10000, Boranium: 10000, Germanium: 10000}},
	}})
	return u
}

func TestEmpireProductionFillsAllBasesBeforeTurningBlue(t *testing.T) {
	u := shipProductionUniverse(t)
	p := u.Player(1)
	d := p.Designs[0]
	before, _ := json.Marshal(u.Game)
	fill, err := PlanEmpireShipProduction(&u.Game.Rules, p, u.Game.Planets, d.Num, MaxShipProductionOrder, "tanker", true)
	require.NoError(t, err)
	require.Len(t, fill.Yards, 2)
	require.Positive(t, fill.Assigned)
	require.Equal(t, fill.Assigned, fill.ThisYear)
	for _, yard := range fill.Yards {
		require.Positive(t, yard.ThisYear)
		require.Equal(t, 1, yard.Years)
	}
	after, _ := json.Marshal(u.Game)
	require.JSONEq(t, string(before), string(after), "preview must not mutate source orders, cargo, specs or plans")
	plan, err := PlanEmpireShipProduction(&u.Game.Rules, p, u.Game.Planets, d.Num, fill.Assigned+1, "tanker", false)
	require.NoError(t, err)
	require.Zero(t, plan.Unassigned)
	require.Equal(t, fill.Assigned, plan.ThisYear)
	later := 0
	for _, yard := range plan.Yards {
		later += yard.Later
		if yard.Later > 0 {
			require.Greater(t, yard.Years, 1)
		}
		for _, item := range yard.Planet.ProductionQueue {
			if item.GetTag(shipProductionWindowTag) == "current" {
				require.Equal(t, 1, item.YearsToBuildAll)
			}
			if item.GetTag(shipProductionWindowTag) == "later" {
				require.Greater(t, item.YearsToBuildAll, 1)
			}
		}
	}
	require.Equal(t, 1, later)
	after, _ = json.Marshal(u.Game)
	require.JSONEq(t, string(before), string(after), "future capacity curves must also leave inputs unchanged")
}

func TestEmpireProductionHonorsMineralResearchAndDockLimits(t *testing.T) {
	u := shipProductionUniverse(t)
	p := u.Player(1)
	d := p.Designs[0]
	for _, planet := range u.Game.Planets {
		planet.Cargo.Ironium = 0
		planet.Cargo.Boranium = 0
		planet.Cargo.Germanium = 0
		planet.Mines = 0
	}
	plan, err := PlanEmpireShipProduction(&u.Game.Rules, p, u.Game.Planets, d.Num, 100, "manual", true)
	require.NoError(t, err)
	require.Zero(t, plan.ThisYear)
	for _, planet := range u.Game.Planets {
		planet.Cargo.Ironium = 10000
		planet.Cargo.Boranium = 10000
		planet.Cargo.Germanium = 10000
	}
	p.ResearchAmount = 100
	plan, err = PlanEmpireShipProduction(&u.Game.Rules, p, u.Game.Planets, d.Num, 100, "manual", true)
	require.NoError(t, err)
	require.Zero(t, plan.ThisYear)
	p.ResearchAmount = 0
	// Changing the owned station to a fort removes its construction dock.
	base := p.Designs[1]
	base.Spec.SpaceDock = 0
	for i := range base.Slots {
		if base.Slots[i].HullComponent == SpaceDock.Name {
			base.Slots[i].Quantity = 0
		}
	}
	// The native hull supplies station capacity, so exercise an actual fort design.
	fort := NewShipDesign(p.Num, base.Num).WithHull(OrbitalFort.Name).WithSpec(&u.Game.Rules, p)
	p.Designs[1] = fort
	plan, err = PlanEmpireShipProduction(&u.Game.Rules, p, u.Game.Planets, d.Num, 1, "manual", false)
	require.NoError(t, err)
	require.Zero(t, plan.Assigned)
}

func TestEmpireProductionBuildsWithChosenRoleAndPreservesPaidOrders(t *testing.T) {
	u := shipProductionUniverse(t)
	p, planet := u.Player(1), u.Planet("Alpha")
	d := p.Designs[0]
	d.Spec.DefaultAutoRole = "scout"
	old := *NewProductionQueueItemShip(1, d)
	old.design = d
	old.Allocated = Cost{Resources: 1}
	old.Note = "Existing paid order"
	planet.ProductionQueue = []ProductionQueueItem{old}
	plan, err := PlanEmpireShipProduction(&u.Game.Rules, p, u.Game.Planets, d.Num, 2, "tanker", false)
	require.NoError(t, err)
	for _, yard := range plan.Yards {
		if yard.Planet.Num == planet.Num {
			found := false
			for _, item := range yard.Planet.ProductionQueue {
				if item.Note == old.Note {
					require.Equal(t, old.Allocated, item.Allocated)
					found = true
				}
			}
			require.True(t, found, "existing paid work must be retained")
		}
		if yard.ThisYear+yard.Later > 0 {
			u.Game.getPlanet(yard.Planet.Num).ProductionQueue = yard.Planet.ProductionQueue
		}
	}
	require.NoError(t, u.turn.generateTurn())
	newTankers, oldScouts := 0, 0
	for _, f := range u.Game.getFleets(p.Num) {
		if f.Delete {
			continue
		}
		switch p.Automation.FleetRoles[f.Num] {
		case "tanker":
			newTankers += f.Spec.TotalShips
		case "scout":
			oldScouts += f.Spec.TotalShips
		}
		require.Empty(t, f.GetTag(ProductionAutoRoleTag))
	}
	require.Equal(t, 2, newTankers)
	require.Equal(t, 1, oldScouts)
	// A partial manual batch must keep its override for later years.
	tags := Tags{ProductionAutoRoleTag: "manual"}
	_, err = u.turn.buildFleet(p, planet, ShipToken{DesignNum: d.Num, Quantity: 1, design: d}, tags, nil)
	require.NoError(t, err)
	require.Equal(t, "manual", tags[ProductionAutoRoleTag])
}

func TestEmpireProductionValidatesRoleAndAvailableDesign(t *testing.T) {
	u := shipProductionUniverse(t)
	p := u.Player(1)
	d := p.Designs[0]
	for _, quantity := range []int{0, -1, MaxShipProductionOrder + 1} {
		_, err := PlanEmpireShipProduction(&u.Game.Rules, p, u.Game.Planets, d.Num, quantity, "", false)
		require.Error(t, err)
	}
	_, err := PlanEmpireShipProduction(&u.Game.Rules, p, u.Game.Planets, d.Num, 1, "unknown", false)
	require.Error(t, err)
	p.DesignMaintenance = DesignMaintenance{d.Num: RetireDesign}
	_, err = PlanEmpireShipProduction(&u.Game.Rules, p, u.Game.Planets, d.Num, 1, "", false)
	require.Error(t, err)
	require.Error(t, validateProductionAutoRoles(p, []ProductionQueueItem{{Type: QueueItemTypeMine, Tags: Tags{ProductionAutoRoleTag: "scout"}}}))
}
