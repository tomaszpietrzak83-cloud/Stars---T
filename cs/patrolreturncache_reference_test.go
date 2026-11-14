package cs

import "slices"

func (t *turnGenerator) referenceBalanceDispersedPatrolFuel(p *Player, ships []*Fleet) {
	if len(ships) < 2 || !patrolFuelAreaClear(p, ships[0], ships[0].Position) {
		return
	}
	ships = slices.Clone(ships)
	reserves := map[*Fleet]int{}
	for _, f := range ships {
		if f.Delete || f.Spec.TotalShips != 1 || f.Position != ships[0].Position || p.Automation.FleetRoles[f.Num] != "patrol" {
			continue
		}
		reserve := automaticReturnFuel(&t.game.Rules, p, f, f.Position, expeditionStations(p, f), false)
		if f.Fuel >= reserve {
			reserves[f] = reserve
		}
	}
	ships = slices.DeleteFunc(ships, func(f *Fleet) bool { _, ok := reserves[f]; return !ok })
	slices.SortStableFunc(ships, func(a, b *Fleet) int { return (a.Spec.FuelCapacity - a.Fuel) - (b.Spec.FuelCapacity - b.Fuel) })
	for i, receiver := range ships {
		need := receiver.Spec.FuelCapacity - receiver.Fuel
		if need <= 0 {
			continue
		}
		surplus := 0
		for _, f := range ships[i+1:] {
			surplus += f.Fuel - reserves[f]
		}
		if surplus < need {
			break
		}
		for j := len(ships) - 1; j > i && need > 0; j-- {
			f := ships[j]
			amount := min(need, f.Fuel-reserves[f])
			receiver.Fuel += amount
			f.Fuel -= amount
			need -= amount
			f.ComputeFuelUsage(p)
		}
		receiver.ComputeFuelUsage(p)
	}
}
