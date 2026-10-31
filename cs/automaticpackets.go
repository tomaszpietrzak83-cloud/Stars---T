package cs

import (
	"fmt"
	"math"
)

const automaticPacketTag = "automatic-packet"

// Packets supplement transport only between owned, functioning mass drivers.
// The receiver's safe speed is a hard limit; an ordinary colony is never a target.
func (t *turnGenerator) planAutomaticPackets() {
	for _, player := range t.game.Players {
		// Cancel unpaid/part-paid automatic exports if their receiver is lost or
		// loses its mass driver. Never launch a formerly safe packet blindly.
		for _, source := range t.game.getPlanets(player.Num) {
			destination := t.game.getPlanet(source.PacketTargetNum)
			valid := source.Spec.HasMassDriver && destination != nil && destination.OwnedBy(player.Num) && destination.Spec.HasMassDriver && source.PacketSpeed > 0 && source.PacketSpeed <= destination.Spec.SafePacketSpeed
			if !valid {
				queue := source.ProductionQueue[:0]
				for _, item := range source.ProductionQueue {
					if item.Tags[automaticPacketTag] != "" {
						source.Cargo = source.Cargo.AddMineral(item.Allocated.ToMineral())
					} else {
						queue = append(queue, item)
					}
				}
				source.ProductionQueue = queue
			}
		}
		if !player.Automation.AutomaticPackets {
			continue
		}
		incoming := map[int]Mineral{}
		for _, packet := range t.game.MineralPackets {
			if !packet.Delete && packet.PlayerNum == player.Num {
				incoming[packet.TargetPlanetNum] = incoming[packet.TargetPlanetNum].Add(packet.Cargo.ToMineral())
			}
		}
		for _, f := range t.game.getFleets(player.Num) {
			if job := logisticsWork(f); job != nil && job.DeliveryNum > 0 {
				amount := f.Cargo.ToMineral()
				if job.Kind == "planet" {
					amount = amount.Add(job.Load)
				}
				incoming[job.DeliveryNum] = incoming[job.DeliveryNum].Add(amount)
			}
		}
		for _, source := range t.game.Planets {
			if !source.OwnedBy(player.Num) || !source.Spec.HasMassDriver {
				continue
			}
			pending, manual := false, false
			for _, item := range source.ProductionQueue {
				if item.Type.IsPacket() {
					if item.Tags[automaticPacketTag] != "" {
						pending = true
					} else {
						manual = true
					}
				}
			}
			if pending || manual {
				continue
			}
			spare := source.Cargo.ToMineral().ToCost().Subtract(t.productionMinerals(player, source, false).ToCost()).ToMineral()
			var destination *Planet
			best := math.Inf(1)
			var load Mineral
			unit := max(1, player.Race.Spec.MineralsPerSingleMineralPacket)
			costPer := max(1, int(float64(unit)*player.Race.Spec.PacketMineralCostFactor))
			for _, yard := range t.game.Planets {
				if yard.Num == source.Num || !yard.OwnedBy(player.Num) || !yard.Spec.HasMassDriver || yard.Spec.DockCapacity == 0 || yard.Spec.SafePacketSpeed <= 0 {
					continue
				}
				need := t.productionMinerals(player, yard, true).ToCost().Subtract(yard.Cargo.ToMineral().ToCost()).Subtract(yard.Spec.MiningOutput.ToCost()).Subtract(incoming[yard.Num].ToCost()).ToMineral()
				fullNeed := committedManufacturingMinerals(&t.game.Rules, player, yard).ToCost().Subtract(yard.Cargo.ToMineral().ToCost()).Subtract(yard.Spec.MiningOutput.ToCost()).Subtract(incoming[yard.Num].ToCost()).MinZero().ToMineral()
				for _, kind := range MineralTypes {
					if need.GetAmount(kind) > 0 && fullNeed.GetAmount(kind) >= unit {
						need.Set(kind, max(unit, need.GetAmount(kind)))
					}
				}
				candidate := Mineral{}
				for _, kind := range MineralTypes {
					count := min(10, max(0, need.GetAmount(kind))/unit, max(0, spare.GetAmount(kind))/costPer)
					candidate.Set(kind, count*unit)
				}
				if candidate.Total() == 0 {
					continue
				}
				distance := source.Position.DistanceTo(yard.Position)
				if distance < best {
					best, destination, load = distance, yard, candidate
				}
			}
			if destination == nil {
				continue
			}
			source.PacketTargetNum = destination.Num
			source.PacketSpeed = min(source.Spec.SafePacketSpeed, destination.Spec.SafePacketSpeed)
			if source.PacketSpeed < 1 {
				continue
			}
			types := []QueueItemType{QueueItemTypeIroniumMineralPacket, QueueItemTypeBoraniumMineralPacket, QueueItemTypeGermaniumMineralPacket}
			for i, kind := range MineralTypes {
				quantity := load.GetAmount(kind) / unit
				if quantity > 0 {
					source.ProductionQueue = append(source.ProductionQueue, ProductionQueueItem{Type: types[i], Quantity: quantity, Tags: Tags{automaticPacketTag: fmt.Sprint(destination.Num)}, Note: "Automatic mineral supply"})
				}
			}
			incoming[destination.Num] = incoming[destination.Num].Add(load)
		}
	}
}
