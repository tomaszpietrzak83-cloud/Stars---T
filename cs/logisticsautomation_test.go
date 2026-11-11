//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
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
	target := *u.Planet("Home")
	target.Num, target.PlayerNum, target.Position = 2, 2, Vector{50, 0}
	target.Spec.HasStarbase = false
	p.PlanetIntels = []*Planet{u.Planet("Home"), &target}
	job := &logisticsJob{Kind: "invade", Position: target.Position}
	prepare := func(tg *turnGenerator) { tg.prepareConvoy(p, transport, job, target.ToTarget()) }
	u.Run(prepare)
	require.Len(t, transport.Waypoints, 1)
	require.Contains(t, job.Status, "Waiting for enough escort strength")
	p.Automation.FleetRoles[escort.Num] = "escort"
	u.Run(prepare)
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
	p.BattleRecords = []BattleRecord{{Position: combat.Position, fleets: []*Fleet{combat}, Stats: BattleRecordStats{ShipsDestroyedByPlayer: map[int]int{1: 1}}}}
	orders := slices.Clone(combat.Waypoints)
	u.Run((*turnGenerator).collectBattleSalvage)
	require.Equal(t, 100, combat.Cargo.Ironium)
	require.Equal(t, orders, combat.Waypoints, "pickup must not order the warship to return")
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
	require.Greater(t, len(fleet.Waypoints), 2)
	field := p.MinefieldIntels[0]
	field.Detonate = true
	u.Run((*turnGenerator).planLogisticsAutomation)
	for i, wp := range fleet.Waypoints[1:] {
		require.Greater(t, segmentDistance(field.Position, fleet.Waypoints[i].Position, wp.Position), automaticMineClearance(field))
	}
	require.Equal(t, Vector{50, 0}, fleet.Waypoints[len(fleet.Waypoints)-1].Position)
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

func TestArmedCargoHullUsesOnlyFreeSpaceAfterBattle(t *testing.T) {
	galleon, privateer := DesignGalleon, DesignPrivateer
	galleon.Slots = append(slices.Clone(galleon.Slots), ShipDesignSlot{HullComponent: Laser.Name, HullSlotIndex: 4, Quantity: 1})
	privateer.Slots = slices.Clone(privateer.Slots)
	privateer.Slots[3].HullComponent = Laser.Name
	for _, design := range []ShipDesign{galleon, privateer} {
		t.Run(design.Hull, func(t *testing.T) {
			position := Vector{50, 50}
			u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
				{Designs: Designs(design), Fleets: []ScenarioFleet{{Name: "Collector", Position: position, Design: design.Name}},
					Salvages: []Salvage{{MapObject: MapObject{Position: position}, Cargo: Cargo{Ironium: 100}}}},
				{Designs: Designs(DesignDestroyerDelta), Fleets: []ScenarioFleet{{Name: "Enemy", Position: position, Design: DesignDestroyerDelta.Name}}},
			}})
			p, f := u.Player(1), u.Fleet("Collector")
			p.Automation.BattleSalvage = true
			p.Relations[1].Relation = PlayerRelationEnemy
			require.True(t, f.Spec.HasWeapons)
			f.Cargo.Boranium = f.Spec.CargoCapacity - 15
			f.Waypoints = append(f.Waypoints, NewPositionWaypoint(Vector{140, 50}, 6))
			p.BattleRecords = []BattleRecord{{Position: position, fleets: []*Fleet{f}}}
			u.Run((*turnGenerator).collectBattleSalvage)
			require.Zero(t, f.Cargo.Ironium, "a surviving armed enemy prevents pickup")
			u.Game.deleteFleet(u.Fleet("Enemy"))
			u.Run((*turnGenerator).collectBattleSalvage)
			require.Equal(t, 15, f.Cargo.Ironium)
			require.Equal(t, 85, u.Game.Salvages[0].Cargo.Ironium)
			require.Equal(t, Vector{140, 50}, f.Waypoints[1].Position)
		})
	}
}

func TestMineralPickupAbandonsOccupiedOwnOrbitWithoutBorrowingPatrol(t *testing.T) {
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
			require.Zero(t, f.CargoMass())
			require.Nil(t, logisticsWork(f))
			require.Empty(t, e.GetTag(escortTag))
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
	require.Zero(t, knownRouteThreat(p, Vector{0, 0}, Vector{40, 0}), "speed without an approaching course does not imply a meeting")
	enemy.Position, enemy.Heading = Vector{76, 0}, VectorFloat64{X: -1}
	require.Positive(t, knownRouteThreat(p, Vector{0, 0}, Vector{40, 0}), "a predicted meeting remains dangerous")
}

func TestLegacyEscortSelectionBecomesPatrol(t *testing.T) {
	u := logisticsUniverse(t)
	player := u.Player(1)
	fleet := u.Fleet("Escort")
	transport := u.Fleet("Transport")
	player.Automation.FleetRoles = map[int]string{fleet.Num: "escort"}
	fleet.SetTag(escortStagingTag, "legacy staging")

	player.NormalizeFleetEscortRoles(&player.Automation, u.Game.getFleets(player.Num))

	require.Equal(t, "patrol", player.Automation.FleetRoles[fleet.Num])
	require.True(t, patrolOperational(player, fleet))

	borrowBomberProtection(player, fleet, transport)
	player.NormalizeFleetEscortRoles(&player.Automation, u.Game.getFleets(player.Num))
	require.Equal(t, "escort", player.Automation.FleetRoles[fleet.Num])
	u.turn.releaseEscorts(player, transport)
	require.Equal(t, "patrol", player.Automation.FleetRoles[fleet.Num])

	require.Equal(t, "patrol", effectiveDesignAutoRole(ShipDesignSpec{DefaultAutoRole: "escort"}))
}
