package cs

import (
	"fmt"
	"strconv"
)

// Older operations stored both escort and transport designs on the leader.
// Preserve its identity and passengers, and divide only actual fuel.
func (t *turnGenerator) separateLegacyLandingEscort(p *Player, leader *Fleet, op *landingOperation) bool {
	if len(op.EscortDesigns) == 0 && !leader.Spec.Radiating {
		return true
	}
	transports, guards := []ShipToken{}, []ShipToken{}
	for _, token := range leader.Tokens {
		design := p.GetDesign(token.DesignNum)
		if design == nil {
			holdAutomaticFleet(p, leader, "Landing held: missing design for legacy escort separation")
			return false
		}
		if op.EscortDesigns[token.DesignNum] || design.Spec.Radiating {
			guards = append(guards, token)
		} else {
			transports = append(transports, token)
		}
	}
	if len(guards) == 0 && !leader.Spec.Radiating {
		op.EscortDesigns = nil
		saveLandingOperation(leader, op)
		return true
	}
	if len(transports) == 0 || len(guards) == 0 {
		holdAutomaticFleet(p, leader, "Landing held: no separate non-radiating transport for the passengers")
		return false
	}
	probe := *leader
	probe.Tokens = guards
	capacity := ComputeFleetSpec(&t.game.Rules, p, &probe).FuelCapacity
	fuel := min(capacity, leader.Fuel*capacity/max(1, leader.Spec.FuelCapacity))
	_, escort, err := NewOrderer().SplitFleet(&t.game.Rules, p, t.game.getFleets(p.Num), SplitFleetRequest{
		Source: leader, SourceTokens: transports, DestTokens: guards, DestBaseName: "Landing patrol", TransferAmount: CargoTransferRequest{Fuel: -fuel},
	})
	if err != nil {
		holdAutomaticFleet(p, leader, fmt.Sprintf("Landing held: cannot separate legacy escort: %v", err))
		return false
	}
	t.game.Fleets = append(t.game.Fleets, escort)
	_ = t.game.Universe.addFleet(escort)
	delete(escort.Tags, landingOperationTag)
	delete(escort.Tags, logisticsTag)
	delete(escort.Tags, landingMemberTag)
	if escort.Spec.HasWeapons {
		if p.Automation.FleetRoles == nil {
			p.Automation.FleetRoles = map[int]string{}
		}
		p.Automation.FleetRoles[escort.Num] = "patrol"
		escort.SetTag(landingMemberTag, strconv.Itoa(leader.Num))
	}
	holdAutomaticFleet(p, escort, "Legacy landing escort separated")
	holdAutomaticFleet(p, leader, "Legacy landing transport separated; reviewing orbital protection")
	op.EscortDesigns = nil
	if op.Phase == "assault" {
		op.Phase = "scan"
	}
	saveLandingOperation(leader, op)
	return true
}

func (t *turnGenerator) separateLegacyLandingEscorts() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if op := readLandingOperation(f); op != nil && !f.Delete && op.Phase != "landed" && op.Phase != "cancelled" {
				t.separateLegacyLandingEscort(p, f, op)
			}
		}
	}
}

func (t *turnGenerator) landingEscortDepartureConfirmed(p *Player, f *Fleet) bool {
	if f.GetTag("automatic-landing-screen") == "" {
		return false
	}
	num, _ := strconv.Atoi(f.GetTag(landingMemberTag))
	leader := t.game.getFleet(p.Num, num)
	op := readLandingOperation(leader)
	if op == nil {
		return false
	}
	target := p.GetPlanetIntel(op.Target)
	if target == nil || len(f.Waypoints) != 2 || f.Waypoints[1].Position != target.Position {
		return false
	}
	cover := t.landingArrivingCover(p, leader, target)
	found := false
	for _, guard := range cover {
		found = found || guard == f
	}
	if !found {
		return false
	}
	reports, known := t.landingEncounterReports(p, op, target)
	return known && (len(reports) == 0 || t.defensiveEvaluator(p).assessFormations(p, cover, reports, target.Position, 1, false, true).Ready)
}

func (t *turnGenerator) landingHostileAt(p *Player, point Vector) bool {
	for _, obj := range t.game.getMapObjectsAtPosition(point) {
		if f, ok := obj.(*Fleet); ok && !f.Delete && f.PlayerNum != p.Num && !p.IsFriend(f.PlayerNum) && (f.Spec.HasWeapons || f.Spec.Bomber) {
			return true
		}
	}
	return false
}
