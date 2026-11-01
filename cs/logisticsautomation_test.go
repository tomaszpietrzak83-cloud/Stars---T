//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func logisticsUniverse(t *testing.T) *testUniverse {
	u := newTestUniverse(t, TestScenario{
		Players: []ScenarioPlayer{{Designs: Designs(DesignTeamster, DesignDestroyerDelta, DesignSpaceStation),
			Fleets: []ScenarioFleet{{Name: "Transport", Design: DesignTeamster.Name, At: "Home"}, {Name: "Escort", Design: DesignDestroyerDelta.Name, At: "Home"},
				{Name: "Combat", Position: Vector{50, 0}, Tokens: []ScenarioShipToken{{Design: DesignDestroyerDelta.Name, Quantity: 1}, {Design: DesignTeamster.Name, Quantity: 1}}}},
			Salvages: []Salvage{{MapObject: MapObject{Position: Vector{50, 0}}, Cargo: Cargo{Ironium: 100}}}}},
		Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500}}},
	})
	u.Player(1).SalvageIntels = []*Salvage{u.Game.Salvages[0]}
	u.Player(1).Automation.FleetRoles = map[int]string{u.Fleet("Transport").Num: "freighter"}
	return u
}

func TestLogisticsCollectsAndDeliversKnownSalvage(t *testing.T) {
	u := logisticsUniverse(t)
	transport := u.Fleet("Transport")
	for range 2 {
		u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).fleetMove, (*turnGenerator).resolveLogisticsAutomation)
	}
	require.Equal(t, 100, transport.Cargo.Ironium)
	require.True(t, u.Game.Salvages[0].Delete)
	for range 2 {
		u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).fleetMove, (*turnGenerator).resolveLogisticsAutomation)
	}
	require.Zero(t, transport.Cargo.Ironium)
	require.Equal(t, 100, u.Planet("Home").Cargo.Ironium)
}

func TestLogisticsWaitsForEscortAndMatchesSpeed(t *testing.T) {
	u := logisticsUniverse(t)
	p, transport, escort := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	p.FleetIntels = []*Fleet{{MapObject: MapObject{PlayerNum: 2, Position: Vector{50, 0}}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true, PowerRating: 1}}}}
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.Len(t, transport.Waypoints, 1)
	require.Contains(t, logisticsWork(transport).Status, "Waiting")
	p.Automation.FleetRoles[escort.Num] = "escort"
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.Len(t, transport.Waypoints, 2)
	require.Len(t, escort.Waypoints, 2)
	require.Equal(t, transport.Waypoints[1].WarpSpeed, escort.Waypoints[1].WarpSpeed)
	u.Run((*turnGenerator).fleetMove)
	require.Equal(t, transport.Position, escort.Position)
}

func TestLogisticsDoesNotUseUnknownSalvage(t *testing.T) {
	u := logisticsUniverse(t)
	u.Player(1).SalvageIntels = nil
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.Nil(t, logisticsWork(u.Fleet("Transport")))
}

func TestAutomaticTransportTakesControlOfItsRoute(t *testing.T) {
	u := logisticsUniverse(t)
	fleet := u.Fleet("Transport")
	fleet.Waypoints = append(fleet.Waypoints, NewPositionWaypoint(Vector{200, 100}, 5))
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.NotNil(t, logisticsWork(fleet))
	require.Equal(t, Vector{50, 0}, fleet.Waypoints[1].Position)
}

func TestLogisticsBattlePickupAndCombatOffload(t *testing.T) {
	u := logisticsUniverse(t)
	p, combat, transport := u.Player(1), u.Fleet("Combat"), u.Fleet("Transport")
	p.Automation.BattleSalvage = true
	p.Automation.OffloadCombatCargo = true
	p.BattleRecords = []BattleRecord{{Position: combat.Position, Stats: BattleRecordStats{ShipsDestroyedByPlayer: map[int]int{2: 1}}}}
	u.Run((*turnGenerator).collectBattleSalvage)
	require.Equal(t, 100, combat.Cargo.Ironium)
	p.SalvageIntels = nil
	for range 2 {
		u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).fleetMove, (*turnGenerator).resolveLogisticsAutomation)
	}
	require.Zero(t, combat.Cargo.Ironium)
	require.Equal(t, 100, transport.Cargo.Ironium)
}

func TestLogisticsCapacityAndMineSafety(t *testing.T) {
	u := logisticsUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Transport")
	p.MinefieldIntels = []*Minefield{{MapObject: MapObject{PlayerNum: 2, Position: Vector{25, 0}}, MinefieldType: MinefieldTypeStandard, NumMines: 100}}
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.Len(t, fleet.Waypoints, 2)
	require.LessOrEqual(t, fleet.Waypoints[1].WarpSpeed, u.Game.Rules.MinefieldStatsByType[MinefieldTypeStandard].MaxSpeed)
	p.MinefieldIntels[0].Detonate = true
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.Len(t, fleet.Waypoints, 1)
	require.Contains(t, logisticsWork(fleet).Status, "safe route")
	cargo := Cargo{Ironium: 1000, Boranium: 1000, Germanium: 1000}
	loaded := takeMinerals(fleet, &cargo)
	require.Equal(t, fleet.Spec.CargoCapacity, loaded)
	require.Equal(t, 3000-loaded, cargo.ToMineral().Total())
}

func TestLogisticsValidatesOwnershipAndRoles(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	require.NoError(t, p.ValidateLogisticsAutomation(p.Automation, u.Game.Fleets))
	require.Error(t, p.ValidateLogisticsAutomation(LogisticsAutomation{FleetRoles: map[int]string{99999: "freighter"}}, u.Game.Fleets))
	require.Error(t, p.ValidateLogisticsAutomation(LogisticsAutomation{FleetRoles: map[int]string{u.Fleet("Transport").Num: "escort"}}, u.Game.Fleets))
}

func TestLogisticsSuppliesShipyardWithoutStockThresholds(t *testing.T) {
	u := logisticsUniverse(t)
	p, yard := u.Player(1), u.Planet("Home")
	p.SalvageIntels = nil
	item := NewProductionQueueItemShip(5, p.Designs[0])
	item.design = p.Designs[0]
	yard.ProductionQueue = []ProductionQueueItem{*item}
	source := &Planet{MapObject: MapObject{Type: MapObjectTypePlanet, Num: 2, PlayerNum: 1, Name: "Mining world", Position: Vector{50, 0}}, Cargo: Cargo{Ironium: 1000, Boranium: 1000, Germanium: 1000}}
	u.Game.Planets = append(u.Game.Planets, source)
	for range 4 {
		u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).fleetMove, (*turnGenerator).resolveLogisticsAutomation)
	}
	require.Positive(t, yard.Cargo.ToMineral().Total())
	require.Equal(t, 3000-source.Cargo.ToMineral().Total(), yard.Cargo.ToMineral().Total())
}

func TestLogisticsCollectsAfterRealBattle(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignBattleBeam, DesignTeamster), Fleets: []ScenarioFleet{{Name: "Winner", Position: Vector{50, 50}, Tokens: []ScenarioShipToken{{Design: DesignBattleBeam.Name, Quantity: 4}, {Design: DesignTeamster.Name, Quantity: 1}}}}},
		{Designs: Designs(DesignLongRangeScout), Fleets: []ScenarioFleet{{Name: "Target", Position: Vector{50, 50}, Design: DesignLongRangeScout.Name}}},
	}})
	u.Player(1).Relations[1].Relation = PlayerRelationEnemy
	u.Player(2).Relations[0].Relation = PlayerRelationEnemy
	u.Player(1).Automation.BattleSalvage = true
	u.Run((*turnGenerator).fleetBattle)
	require.True(t, u.Fleet("Target").Delete)
	require.NotEmpty(t, u.Player(1).BattleRecords)
	u.Run((*turnGenerator).collectBattleSalvage)
	require.Positive(t, u.Fleet("Winner").Cargo.ToMineral().Total())
}

func TestMineralPickupRequestsDeliveryEscortBeforeLoading(t *testing.T) {
	for _, auto := range []bool{false, true} {
		t.Run(fmt.Sprint(auto), func(t *testing.T) {
			u := passengerUniverse(t)
			p, f, e := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
			source, dest := u.Planet("Home"), u.Planet("Young colony")
			p.Automation.DisablePassengerTransport = true
			source.Cargo.Ironium = 10000
			p.PlanetIntels = []*Planet{source, dest}
			p.FleetIntels = []*Fleet{{MapObject: MapObject{PlayerNum: 2, Num: 91, Position: dest.Position}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true, PowerRating: 1}}}}
			if auto {
				p.Automation.FleetRoles[e.Num] = "patrol"
			}
			saveLogisticsWork(f, &logisticsJob{Kind: "planet", TargetNum: source.Num, Position: source.Position, DeliveryNum: dest.Num, Load: Mineral{Ironium: f.Spec.CargoCapacity}})
			u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).resolveLogisticsAutomation, (*turnGenerator).dispatchReadyLogistics)
			if auto {
				require.Positive(t, f.Cargo.Ironium)
				require.Greater(t, len(f.Waypoints), 1)
				require.Equal(t, "escort", p.Automation.FleetRoles[e.Num])
			} else {
				require.Zero(t, f.CargoMass())
				require.Contains(t, logisticsWork(f).Status, "escort")
				require.Empty(t, p.Automation.FleetRoles[e.Num])
			}
		})
	}
}

func TestBombardmentEscortDoesNotBlockPassingCivilianRoute(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	planet := u.Planet("Home")
	planet.Position = Vector{20, 10}
	p.PlanetIntels = []*Planet{planet}
	enemy := &Fleet{MapObject: MapObject{PlayerNum: 2, Num: 90, Position: planet.Position}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true, Bomber: true, PowerRating: 100}}}
	p.FleetIntels = []*Fleet{enemy}
	require.Zero(t, knownRouteThreat(p, Vector{0, 0}, Vector{40, 0}))
	require.Positive(t, knownRouteThreat(p, Vector{0, 0}, planet.Position), "entering the occupied orbit can still cause native combat")
	enemy.WarpSpeed = 6
	require.Positive(t, knownRouteThreat(p, Vector{0, 0}, Vector{40, 0}), "a moving formation must be treated as mobile again")
}
