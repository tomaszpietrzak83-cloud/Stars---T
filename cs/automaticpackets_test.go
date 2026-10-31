//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAutomaticPacketsRespectReceiversAndDoNotDuplicate(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignSpaceStation, DesignTeamster)}}, Planets: []ScenarioPlanet{{Name: "Source", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Ironium: 3000, Boranium: 3000, Germanium: 3000}}, {Name: "Yard", Owner: 1, Starbase: DesignSpaceStation.Name, Position: Vector{100, 0}, ProductionQueue: []ScenarioProductionQueueItem{{Type: QueueItemTypeShipToken, Design: DesignTeamster.Name, Quantity: 100}}}}})
	p, source, yard := u.Player(1), u.Planet("Source"), u.Planet("Yard")
	p.Automation.AutomaticPackets = true
	source.Spec.HasMassDriver = true
	source.Spec.SafePacketSpeed = 10
	yard.Spec.HasMassDriver = true
	yard.Spec.SafePacketSpeed = 5
	u.Run((*turnGenerator).planAutomaticPackets)
	require.NotEmpty(t, source.ProductionQueue)
	require.Equal(t, yard.Num, source.PacketTargetNum)
	require.Equal(t, 5, source.PacketSpeed)
	count := len(source.ProductionQueue)
	u.Run((*turnGenerator).planAutomaticPackets)
	require.Len(t, source.ProductionQueue, count)
	yard.Spec.HasMassDriver = false
	u.Run((*turnGenerator).planAutomaticPackets)
	require.Empty(t, source.ProductionQueue)
	source.ProductionQueue = []ProductionQueueItem{{Type: QueueItemTypeIroniumMineralPacket, Quantity: 1}}
	yard.Spec.HasMassDriver = true
	source.PacketTargetNum = 77
	u.Run((*turnGenerator).planAutomaticPackets)
	require.Equal(t, 77, source.PacketTargetNum)
	require.Len(t, source.ProductionQueue, 1)
}
