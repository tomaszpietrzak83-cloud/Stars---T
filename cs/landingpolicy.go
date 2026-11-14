package cs

import (
	"fmt"
	"slices"
	"strconv"
)

// Cargo population units are hundreds of people. Civilian logistics remains
// the primary job of transports; occupation uses a small, replaceable army.
const landingArmyLimit = 4000
const landingPreparationTag = "landing-preparation"
const landingRetryTag = "landing-retry-year"

func (t *turnGenerator) landingPrepared(p *Player, target *Planet) bool {
	return target != nil && !target.Spec.HasStarbase &&
		(!reportedGroundDefenses(target) && target.Cargo.Colonists <= 1000 || t.percentageBombingLanding(p, target))
}

func (t *turnGenerator) landingArmyNeed(p *Player, target *Planet) int {
	return min(landingArmyLimit, max(2000, t.landingRequirement(p, target)))
}

func (t *turnGenerator) landingReservedTroops(p *Player, leader *Fleet) int {
	total := 0
	for _, f := range t.landingMembers(p, leader) {
		total += f.Cargo.Colonists
		if job := logisticsWork(f); job != nil && job.Kind == "landing-load" {
			total += job.Passengers
		}
	}
	return total
}

// Run before civilian assignments, so released holds get real deliveries in
// the same planning pass. A planet request does not reserve any transport.
func (t *turnGenerator) releaseBlockedLandings() {
	if t.ordersOnly {
		return
	}
	for _, p := range t.automaticPlayers() {
		for _, leader := range slices.Clone(t.game.getFleets(p.Num)) {
			op := readLandingOperation(leader)
			if op == nil || leader.Delete || op.Phase == "landed" || op.Phase == "cancelled" {
				continue
			}
			target := p.GetPlanetIntel(op.Target)
			stamp := fmt.Sprintf("%v/%d/%s", leader.Position, leader.Cargo.Colonists, op.Phase)
			if op.Progress != stamp {
				if op.Progress == "" && leader.Position == op.Rally && t.game.Year-op.Started >= 4 {
					op.ProgressYear = op.Started
				} else {
					op.ProgressYear = t.game.Year
				}
				op.Progress = stamp
			}
			blocked := !t.landingPrepared(p, target)
			oversized := t.landingReservedTroops(p, leader) > landingArmyLimit
			stalled := t.game.Year-op.ProgressYear >= 4
			if !blocked && !oversized && !stalled {
				saveLandingOperation(leader, op)
				continue
			}
			if target != nil {
				target.SetTag(landingPreparationTag, "true")
				if stalled {
					target.SetTag(landingRetryTag, strconv.Itoa(t.game.Year+5))
				}
			}
			members := t.landingMembers(p, leader)
			op.Phase = "cancelled"
			t.finishLandingOperation(p, leader, op)
			for _, f := range members {
				if !f.Delete && landingTransport(p, f) {
					t.disperseLandingTransports(p, f, true)
				}
			}
		}
	}
}
