package cs

import "math"

type movingFuelMeeting struct {
	point           Vector
	receiver, donor *expeditionRoute
}

func freeRendezvousWarp(p *Player, f *Fleet) int {
	best := 0
	for warp := 1; warp <= automaticMaxWarp(p, f); warp++ {
		distance := f.StrategicSpeed(warp)
		_, left, _, ok := automaticLegFuel(p, f, warp, distance, f.Fuel, 0)
		if ok && left >= f.Fuel {
			best = warp
		}
	}
	return best
}

// The two vectors split distance in proportion to the fleets' free movement.
// Both budgets are checked, including the donor's delivery and funded return.
func (t *turnGenerator) movingFuelMeeting(p *Player, donor *Fleet, members []*Fleet, amount int, allowThreat bool) *movingFuelMeeting {
	if len(members) == 0 || donor.Position == members[0].Position {
		return nil
	}
	rw := 10
	rc := 10
	for _, f := range members {
		rw = min(rw, freeRendezvousWarp(p, f))
		rc = min(rc, economicalPatrolMaxWarp(p, f))
	}
	dw := freeRendezvousWarp(p, donor)
	dc := economicalPatrolMaxWarp(p, donor)
	from := members[0].Position
	var best *movingFuelMeeting
	score := math.Inf(1)
	for _, pair := range [][2]int{{rw, dw}, {rw, dc}, {rc, dw}, {rc, dc}} {
		if pair[0] <= 0 || pair[1] <= 0 {
			continue
		}
		rspeed := members[0].StrategicSpeed(pair[0])
		dspeed := donor.StrategicSpeed(pair[1])
		point := from.ToFloat64().Add(donor.Position.Subtract(from).Normalized().Scale(from.DistanceTo(donor.Position) * rspeed / (rspeed + dspeed))).ToInt(true)
		if point == from || point == donor.Position {
			continue
		}
		receiver := planAutomaticRoute(&t.game.Rules, p, members, point, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: allowThreat, RefuelRendezvous: true, EconomicalPatrol: true, MaxWarp: pair[0], MinWarp: 1, MaxTurns: 20})
		delivery := planAutomaticRoute(&t.game.Rules, p, []*Fleet{donor}, point, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: combatAutonomy(p, donor), DeliveryFuel: []int{amount}, EconomicalPatrol: true, MaxWarp: pair[1], MinWarp: 1, MaxTurns: 20})
		if receiver == nil || delivery == nil || receiver.WaitForFuel || delivery.WaitForFuel {
			continue
		}
		if max(receiver.Turns, delivery.Turns) > maxTankerMeetingTurns {
			continue
		}
		capacity := donor.Spec.FuelCapacity
		for _, f := range members {
			capacity += f.Spec.FuelCapacity
		}
		value := float64(max(receiver.Turns, delivery.Turns)) + 20*float64(receiver.Fuel+delivery.Fuel)/float64(max(1, capacity))
		// Prefer a fully free meeting whenever it fits the bounded rendezvous horizon.
		if pair == [2]int{rw, dw} {
			return &movingFuelMeeting{point, receiver, delivery}
		}
		if value < score {
			score = value
			best = &movingFuelMeeting{point, receiver, delivery}
		}
	}
	return best
}
