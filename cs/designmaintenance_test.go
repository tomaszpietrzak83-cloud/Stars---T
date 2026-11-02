//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func maintenanceScenario() TestScenario {
	designs := Designs(DesignLongRangeScout, DesignLongRangeScout, DesignSpaceStation)
	designs[0].Name, designs[1].Name = "Sword1", "Sword2"
	designs[1].Slots[2].HullComponent = ManeuveringJet.Name
	return TestScenario{Players: []ScenarioPlayer{{Designs: designs, Fleets: []ScenarioFleet{{Name: "Fleet", Design: "Sword1", Quantity: 2, At: "Yard"}}}}, Planets: []ScenarioPlanet{{Name: "Yard", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500, Ironium: 1000, Boranium: 1000, Germanium: 1000}}}}
}
func maintenanceUniverse(t *testing.T) *testUniverse {
	t.Helper()
	return newTestUniverse(t, maintenanceScenario())
}
func requestRebuild(u *testUniverse) {
	p, fleet := u.Player(1), u.Fleet("Fleet")
	p.DesignMaintenance = DesignMaintenance{p.Designs[0].Num: p.Designs[1].Num}
	p.Automation.RebuildFleets = map[int]bool{fleet.Num: true}
}
func replacementCount(u *testUniverse) int {
	n := 0
	for _, fleet := range u.Game.Fleets {
		if !fleet.Delete {
			if token := fleet.getTokenByDesign(u.Player(1).Designs[1].Num); token != nil {
				n += token.Quantity
			}
		}
	}
	return n
}
func TestDesignMaintenanceValidation(t *testing.T) {
	u := maintenanceUniverse(t)
	p := u.Player(1)
	a, b := p.Designs[0].Num, p.Designs[1].Num
	require.NoError(t, p.ValidateDesignMaintenance(DesignMaintenance{a: b}))
	require.NoError(t, p.ValidateDesignMaintenance(DesignMaintenance{a: RetireDesign}))
	for _, policy := range []DesignMaintenance{{a: a}, {a: 9999}, {a: p.Designs[2].Num}, {a: b, b: a}, {a: b, b: RetireDesign}} {
		require.Error(t, p.ValidateDesignMaintenance(policy))
	}
	p.Designs[1].Hull = Cruiser.Name
	require.Error(t, p.ValidateDesignMaintenance(DesignMaintenance{a: b}))
}

func TestReplacementInheritsAutomaticRoleBeforeRegrouping(t *testing.T) {
	for _, role := range []string{"scout", "freighter", "escort", "patrol", "miner", "tanker", "terraformer", "colonizer:best"} {
		t.Run(role, func(t *testing.T) {
			u := maintenanceUniverse(t)
			p, planet := u.Player(1), u.Planet("Yard")
			design := p.Designs[1]
			work := &fleetMaintenance{OriginalRole: role, Group: "inherit-test", BaseName: "Replacement", RallyPlanetNum: planet.Num}
			fleet, err := u.turn.addReplacementShips(p, planet, ShipToken{DesignNum: design.Num, Quantity: 1, design: design}, work)
			require.NoError(t, err)
			require.Equal(t, role, p.Automation.FleetRoles[fleet.Num])
			require.True(t, fleet.InDesignMaintenance())
			// Later production batches joining this replacement retain the same role.
			again, err := u.turn.addReplacementShips(p, planet, ShipToken{DesignNum: design.Num, Quantity: 1, design: design}, work)
			require.NoError(t, err)
			require.Same(t, fleet, again)
			require.Equal(t, role, p.Automation.FleetRoles[again.Num])
		})
	}
}
func TestDesignMaintenanceRequiresFleetOrder(t *testing.T) {
	u := maintenanceUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Fleet")
	p.DesignMaintenance = DesignMaintenance{p.Designs[0].Num: p.Designs[1].Num}
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.False(t, fleet.InDesignMaintenance())
	require.Len(t, u.Game.Fleets, 1)
}
func TestDesignMaintenanceScrapBuildAndRegroup(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	p, old := u.Player(1), u.Fleet("Fleet")
	old.Tokens[0].QuantityDamaged = 1
	old.Tokens[0].Damage = float64(p.Designs[0].Spec.Armor) * 0.4
	// Idle fleet: a separate regression covers finishing an assigned voyage first.
	require.NoError(t, u.turn.generateTurn())
	require.Equal(t, 2, replacementCount(u))
	for _, fleet := range u.Game.Fleets {
		if !fleet.Delete {
			require.Nil(t, fleet.getTokenByDesign(p.Designs[0].Num))
			for _, token := range fleet.Tokens {
				require.Zero(t, token.Damage)
				require.Zero(t, token.QuantityDamaged)
			}
		}
	}
	require.NoError(t, u.turn.generateTurn())
	live := 0
	for _, fleet := range u.Game.Fleets {
		if !fleet.Delete {
			live++
			require.False(t, fleet.InDesignMaintenance())
			require.Equal(t, u.Planet("Yard").Position, fleet.Position)
		}
	}
	require.Equal(t, 1, live)
}
func TestDesignMaintenanceUsesNormalMultiYearProduction(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	p, planet := u.Player(1), u.Planet("Yard")
	planet.Spec.ResourcesPerYearAvailable = 1
	u.Run((*turnGenerator).fleetDesignMaintenance)
	produce := func() productionResult {
		producer := newProducer(testLogger, &u.Game.Rules, planet, p)
		producer.maintainShips = func(available *Cost) { u.turn.maintainShipsAtPlanet(p, planet, available) }
		result, err := producer.produce()
		require.NoError(t, err)
		return result
	}
	first := produce()
	require.Empty(t, first.tokens)
	require.NotEmpty(t, planet.ProductionQueue)
	require.Equal(t, 1, planet.ProductionQueue[0].Allocated.Resources)
	for _, fleet := range u.Game.Fleets {
		require.True(t, fleet.Delete, "old ships must already be scrapped while replacements are incomplete")
	}
	raw, err := json.Marshal(planet.ProductionQueue)
	require.NoError(t, err)
	var restored []ProductionQueueItem
	require.NoError(t, json.Unmarshal(raw, &restored))
	planet.ProductionQueue = restored
	require.NoError(t, planet.PopulateProductionQueueDesigns(p))
	built := 0
	for range 200 {
		result := produce()
		for _, token := range result.tokens {
			built += token.Quantity
		}
		if len(planet.ProductionQueue) == 0 {
			break
		}
	}
	require.Empty(t, planet.ProductionQueue)
	require.Equal(t, 2, built)
}
func TestDesignMaintenanceRetiresOnlySelectedDesign(t *testing.T) {
	u := maintenanceUniverse(t)
	p, fleet, planet := u.Player(1), u.Fleet("Fleet"), u.Planet("Yard")
	old, keep := p.Designs[0], p.Designs[1]
	fleet.Tokens = append(fleet.Tokens, ShipToken{DesignNum: keep.Num, Quantity: 1, design: keep})
	p.DesignMaintenance = DesignMaintenance{old.Num: RetireDesign}
	u.Run((*turnGenerator).fleetDesignMaintenance)
	available := Cost{}
	u.turn.maintainShipsAtPlanet(p, planet, &available)
	require.False(t, fleet.Delete)
	require.Len(t, fleet.Tokens, 1)
	require.Equal(t, keep.Num, fleet.Tokens[0].DesignNum)
	require.Equal(t, MultiplyCost(MultiplyCost(old.Spec.Cost, 2), 0.8).ToMineral(), available.ToMineral())
	require.Empty(t, planet.ProductionQueue)
}
func TestDesignMaintenanceNoYardAndCancellation(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	p, fleet, planet := u.Player(1), u.Fleet("Fleet"), u.Planet("Yard")
	planet.Spec.HasStarbase = false
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.True(t, fleet.InDesignMaintenance())
	require.Len(t, fleet.Waypoints, 1)
	require.False(t, fleet.Delete)
	p.DesignMaintenance = nil
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.False(t, fleet.InDesignMaintenance())
}
func TestDesignMaintenanceLocksSplitting(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	u.Run((*turnGenerator).fleetDesignMaintenance)
	_, err := NewOrderer().SplitAll(&u.Game.Rules, u.Player(1), u.Game.Fleets, u.Fleet("Fleet"))
	require.Error(t, err)
}
func TestDesignMaintenanceRoutesWholeFleet(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	fleet := u.Fleet("Fleet")
	fleet.Position = Vector{10, 0}
	fleet.OrbitingPlanetNum = None
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.Len(t, fleet.Waypoints, 2)
	require.Equal(t, u.Planet("Yard").Num, fleet.Waypoints[1].TargetNum)
	require.Len(t, u.Game.Fleets, 2, "split inside the owned base sector without staging at its exact position")
	u.Run((*turnGenerator).fleetMove)
	require.Equal(t, u.Planet("Yard").Position, fleet.Position)
}
func TestDesignMaintenanceDistributesAcrossShipyards(t *testing.T) {
	scenario := maintenanceScenario()
	scenario.Players[0].Fleets[0].Quantity = 4
	second := scenario.Planets[0]
	second.Name = "Other yard"
	second.Position = Vector{1, 0}
	scenario.Planets = append(scenario.Planets, second)
	u := newTestUniverse(t, scenario)
	// Travel now costs whole turns. Distribution is useful when building at
	// one yard takes long enough to outweigh the extra journey.
	for _, planet := range u.Game.Planets {
		planet.Spec.ResourcesPerYearAvailable = 10
	}
	requestRebuild(u)
	u.Run((*turnGenerator).fleetDesignMaintenance)
	assigned := map[int]bool{}
	for _, fleet := range u.Game.Fleets {
		require.Equal(t, 1, fleet.Tokens[0].Quantity)
		assigned[readMaintenance(fleet).PlanetNum] = true
	}
	require.Len(t, assigned, 2)
	for range 6 {
		require.NoError(t, u.turn.generateTurn())
	}
	require.Equal(t, 4, replacementCount(u))
	live := 0
	for _, fleet := range u.Game.Fleets {
		if !fleet.Delete {
			live++
			require.False(t, fleet.InDesignMaintenance())
		}
	}
	require.Equal(t, 1, live, "rebuilds from different yards must regroup")
}

func TestDesignMaintenanceQueueEditsCannotDuplicatePaidWork(t *testing.T) {
	item := ProductionQueueItem{Type: QueueItemTypeShipToken, DesignNum: 2, Quantity: 2, Allocated: Cost{Resources: 10}, Tags: Tags{replacementOrderTag: "server metadata"}}
	require.NoError(t, validateMaintenanceQueueEdits([]ProductionQueueItem{item}, []ProductionQueueItem{item}))
	require.NoError(t, validateMaintenanceQueueEdits([]ProductionQueueItem{item}, nil))
	require.Error(t, validateMaintenanceQueueEdits(nil, []ProductionQueueItem{item}))
	require.Error(t, validateMaintenanceQueueEdits([]ProductionQueueItem{item}, []ProductionQueueItem{item, item}))
}

func TestRebuildFinishesCurrentNavigationBeforeWithdrawing(t *testing.T) {
	u := maintenanceUniverse(t)
	requestRebuild(u)
	f := u.Fleet("Fleet")
	goal := Vector{25, 0}
	f.Waypoints = append(f.Waypoints, NewPositionWaypoint(goal, 5))
	u.Run((*turnGenerator).fleetDesignMaintenance)
	require.False(t, f.InDesignMaintenance())
	require.Equal(t, goal, f.Waypoints[1].Position)
	require.NotEmpty(t, f.GetTag("maintenance-pending"))
	u.Run((*turnGenerator).fleetMove, (*turnGenerator).fleetDesignMaintenance)
	require.Equal(t, goal, f.Position)
	require.True(t, f.InDesignMaintenance())
	require.Equal(t, u.Planet("Yard").Position, f.Waypoints[len(f.Waypoints)-1].Position)
}

func TestRebuildCompletesTransportDeliveryAndReturnFirst(t *testing.T) {
	scenario := maintenanceScenario()
	scenario.Players[0].Designs = Designs(DesignTeamster, DesignTeamster, DesignSpaceStation)
	scenario.Players[0].Designs[0].Name = "Old Transport"
	scenario.Players[0].Designs[1].Name = "New Transport"
	scenario.Players[0].Fleets = []ScenarioFleet{{Name: "Fleet", Design: "Old Transport", At: "Yard", Cargo: Cargo{Colonists: 10}}}
	scenario.Planets = append(scenario.Planets, ScenarioPlanet{Name: "Destination", Owner: 1, Position: Vector{20, 0}, Cargo: Cargo{Colonists: 1}})
	u := newTestUniverse(t, scenario)
	requestRebuild(u)
	p, f := u.Player(1), u.Fleet("Fleet")
	p.Automation.FleetRoles = map[int]string{f.Num: "freighter"}
	home, dest := u.Planet("Yard"), u.Planet("Destination")
	f.Waypoints = append(f.Waypoints, Waypoint{Position: dest.Position, MapObjectTarget: dest.ToTarget(), WarpSpeed: 7}, Waypoint{Position: home.Position, MapObjectTarget: home.ToTarget(), WarpSpeed: 7})
	saveLogisticsWork(f, &logisticsJob{Kind: "settle", TargetNum: dest.Num, DeliveryNum: dest.Num, Position: dest.Position, Passengers: 10})
	require.NoError(t, u.turn.generateTurn())
	require.False(t, f.Delete, "deliver the original passengers before scrapping")
	require.False(t, f.InDesignMaintenance())
	require.Equal(t, dest.Position, f.Position)
	require.GreaterOrEqual(t, dest.Cargo.Colonists, 11)
	require.Zero(t, f.Cargo.Colonists)
	require.NoError(t, u.turn.generateTurn())
	require.False(t, f.Delete, "finish the funded return before rebuilding")
	require.Equal(t, home.Position, f.Position)
	require.NoError(t, u.turn.generateTurn())
	require.True(t, f.Delete, "the completed transport may now be scrapped")
	require.Equal(t, 1, replacementCount(u))
}
