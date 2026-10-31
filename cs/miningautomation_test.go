//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAutomaticMiningPrefersNeededMineralAndKeepsAssignment(t *testing.T) {
	u := newTestUniverse(t, remoteMiningScenario("", 0, DesignPotatoBug, WaypointTaskNone, JoaT))
	p := u.Player(1)
	fleet := u.Game.Fleets[0]
	p.Automation.FleetRoles = map[int]string{fleet.Num: "miner"}
	mine := u.Planet("Mine")
	mine.Position = fleet.Position
	mine.MineralConcentration = Mineral{Ironium: 80, Boranium: 20, Germanium: 10}
	mine.ReportAge = 0
	p.PlanetIntels = []*Planet{mine}
	addExpeditionTestDock(p, fleet.Position)
	u.Run((*turnGenerator).planMiningAutomation)
	require.Equal(t, WaypointTaskRemoteMining, fleet.Waypoints[0].Task)
	require.Equal(t, mine.Num, readMiningJob(fleet).PlanetNum)
	other := &Planet{MapObject: MapObject{Num: 99, Position: Vector{5, 0}, ReportAge: 0}, MineralConcentration: Mineral{Ironium: 100, Boranium: 100, Germanium: 100}}
	p.PlanetIntels = append(p.PlanetIntels, other)
	u.Game.Year += 5
	u.Run((*turnGenerator).planMiningAutomation)
	require.Equal(t, mine.Num, readMiningJob(fleet).PlanetNum)
	u.Game.Year += 10
	u.Run((*turnGenerator).planMiningAutomation)
	require.Equal(t, 99, readMiningJob(fleet).PlanetNum)
}

func TestMiningWeightsAndEligibility(t *testing.T) {
	require.Greater(t, miningValue(&Planet{MineralConcentration: Mineral{Germanium: 80}}, [3]float64{1, 1, 10}), miningValue(&Planet{MineralConcentration: Mineral{Ironium: 100}}, [3]float64{1, 1, 10}))
	u := logisticsUniverse(t)
	p, fleet := u.Player(1), u.Fleet("Transport")
	require.Error(t, p.ValidateLogisticsAutomation(LogisticsAutomation{FleetRoles: map[int]string{fleet.Num: "miner"}}, u.Game.Fleets))
	yard := u.Planet("Home")
	item := NewProductionQueueItemShip(10, p.Designs[0])
	item.design = p.Designs[0]
	yard.ProductionQueue = []ProductionQueueItem{*item}
	cost := u.turn.productionMinerals(p, yard, false)
	yard.Cargo.Ironium = cost.Ironium
	yard.Cargo.Boranium = cost.Boranium
	yard.Cargo.Germanium = 0
	yard.Spec.MiningOutput = Mineral{}
	weights := u.turn.miningWeights(p)
	require.Greater(t, weights[2], weights[0])
	require.Greater(t, weights[2], weights[1])
}

func TestAutomaticMiningRunsNormalExtraction(t *testing.T) {
	u := newTestUniverse(t, remoteMiningScenario("Mine", 0, DesignPotatoBug, WaypointTaskNone, JoaT))
	p, fleet, mine := u.Player(1), u.Game.Fleets[0], u.Planet("Mine")
	p.Automation.FleetRoles = map[int]string{fleet.Num: "miner"}
	mine.ReportAge = 0
	p.PlanetIntels = []*Planet{mine}
	before := mine.Cargo.ToMineral().Total()
	u.Run((*turnGenerator).planMiningAutomation, (*turnGenerator).fleetRemoteMine)
	require.Greater(t, mine.Cargo.ToMineral().Total(), before)
	require.Equal(t, 0, fleet.Cargo.ToMineral().Total())
}

func TestMiningRelocationMustRecoverLostProductionDuringTravel(t *testing.T) {
	u := newTestUniverse(t, remoteMiningScenario("Mine", 0, DesignPotatoBug, WaypointTaskNone, JoaT))
	p, fleet, mine := u.Player(1), u.Game.Fleets[0], u.Planet("Mine")
	p.Automation.FleetRoles = map[int]string{fleet.Num: "miner"}
	mine.ReportAge = 0
	mine.MineralConcentration = Mineral{Ironium: 30, Boranium: 30, Germanium: 30}
	p.PlanetIntels = []*Planet{mine}
	addExpeditionTestDock(p, fleet.Position)
	for i := range fleet.Tokens {
		fleet.Tokens[i].design.Spec.Engine.FuelUsage = make([]int, 11)
	}
	u.Run((*turnGenerator).planMiningAutomation)
	u.Game.Year += 11
	far := &Planet{MapObject: MapObject{Num: 99, Position: Vector{fleet.Position.X + 10000, fleet.Position.Y}}, MineralConcentration: Mineral{Ironium: 80, Boranium: 80, Germanium: 80}}
	p.PlanetIntels = append(p.PlanetIntels, far)
	u.Run((*turnGenerator).planMiningAutomation)
	require.Equal(t, mine.Num, readMiningJob(fleet).PlanetNum, "higher yield must compensate for the long production outage")
	far.Position = Vector{fleet.Position.X + 1, fleet.Position.Y}
	u.Run((*turnGenerator).planMiningAutomation)
	require.Equal(t, 99, readMiningJob(fleet).PlanetNum, "same higher yield becomes worthwhile when travel is short")
}

func TestAutomaticMiningRejectsUnknownAndForeignPlanets(t *testing.T) {
	u := newTestUniverse(t, remoteMiningScenario("", 0, DesignPotatoBug, WaypointTaskNone, JoaT))
	p, fleet := u.Player(1), u.Game.Fleets[0]
	p.Automation.FleetRoles = map[int]string{fleet.Num: "miner"}
	p.PlanetIntels = []*Planet{
		{MapObject: MapObject{Num: 1, ReportAge: ReportAgeUnexplored}, MineralConcentration: Mineral{Ironium: 100}},
		{MapObject: MapObject{Num: 2, PlayerNum: 2, ReportAge: 0}, MineralConcentration: Mineral{Ironium: 100}},
	}
	u.Run((*turnGenerator).planMiningAutomation)
	require.Zero(t, readMiningJob(fleet).PlanetNum)
	require.Len(t, fleet.Waypoints, 1)
}
