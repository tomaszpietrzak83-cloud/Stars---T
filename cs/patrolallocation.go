package cs

import "slices"

// A pack assembled for an earlier threat need not chase a small weak contact
// with every ship. Keep the smallest funded adequate group; release the rest.
func (t *turnGenerator) releaseExcessPatrolShips(p *Player, f, enemy *Fleet, job *patrolJob) {
	if t.ordersOnly || reportedFleetShips(enemy) > 7 || patrolReportedCombatant(p, enemy) || patrolNeedsRecon(p, enemy) || f.Spec.TotalShips <= 1 || f.CargoMass() > 0 || len(job.SupportDesigns) > 0 || f.GetTag(patrolSupportTag) != "" || f.GetTag(escortTag) != "" || fuelNetworkReserved(f) {
		return
	}
	encounterShips := 0
	for _, other := range patrolEncounterReports(p, enemy, patrolInterceptPosition(enemy), 1) {
		encounterShips += reportedFleetShips(other)
	}
	if encounterShips > 7 {
		return
	}
	for _, tok := range f.Tokens {
		if tok.QuantityDamaged > 0 || tok.design == nil || !tok.design.Spec.HasWeapons {
			return
		}
	}
	needed := max(patrolThreat(p, enemy)*3/2+1, job.RecruitPower)
	wanted := patrolDesiredShips(p, enemy)
	keep, spare := slices.Clone(f.Tokens), slices.Clone(f.Tokens)
	for i := range keep {
		keep[i].Quantity = 0
	}
	// Prefer the most useful available weapon design; retain deterministic ties.
	indices := make([]int, len(keep))
	for i := range indices {
		indices[i] = i
	}
	slices.SortStableFunc(indices, func(a, b int) int { return shipCombatPower(keep[b].design.Spec) - shipCombatPower(keep[a].design.Spec) })
	count := 0
	for _, i := range indices {
		for keep[i].Quantity < f.Tokens[i].Quantity {
			keep[i].Quantity++
			spare[i].Quantity--
			count++
			if count >= f.Spec.TotalShips {
				return
			}
			trial := *f
			trial.Tokens = nil
			for _, tok := range keep {
				if tok.Quantity > 0 {
					trial.Tokens = append(trial.Tokens, tok)
				}
			}
			trial.Spec = ComputeFleetSpec(&t.game.Rules, p, &trial)
			if count < wanted || patrolPower(&trial) < needed {
				continue
			}
			if p.patrolEvaluator != nil && !p.patrolEvaluator.assess(p, []*Fleet{&trial}, enemy, patrolInterceptPosition(enemy), 1).Ready {
				continue
			}
			trial.Fuel = min(trial.Spec.FuelCapacity, f.Fuel*trial.Spec.FuelCapacity/max(1, f.Spec.FuelCapacity))
			position := patrolInterceptPosition(enemy)
			if route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{&trial}, position, MapObjectTarget{}, automaticRoutePolicy{Fast: true, AllowThreat: true, MaxTurns: 20, MinWarp: automaticCruiseWarp(p, &trial)}); route == nil || route.WaitForFuel {
				continue
			}
			reserve := *f
			reserve.Tokens = nil
			for _, tok := range spare {
				if tok.Quantity > 0 {
					reserve.Tokens = append(reserve.Tokens, tok)
				}
			}
			reserve.Spec = ComputeFleetSpec(&t.game.Rules, p, &reserve)
			reserve.Fuel = f.Fuel - trial.Fuel
			if reserve.Fuel > reserve.Spec.FuelCapacity || reserve.Fuel < automaticReturnFuel(&t.game.Rules, p, &reserve, reserve.Position, expeditionStations(p, &reserve), true) {
				continue
			}
			_, released, err := NewOrderer().SplitFleet(&t.game.Rules, p, t.game.getFleets(p.Num), SplitFleetRequest{Source: f, SourceTokens: keep, DestTokens: spare, TransferAmount: CargoTransferRequest{Fuel: -reserve.Fuel}})
			if err != nil {
				return
			}
			t.game.Fleets = append(t.game.Fleets, released)
			_ = t.game.Universe.addFleet(released)
			p.Automation.FleetRoles[released.Num] = "patrol"
			holdAutomaticFleet(p, released, "Released from an oversized patrol; available for other targets")
			job.RequiredShips = min(wanted, count)
			return
		}
	}
}
