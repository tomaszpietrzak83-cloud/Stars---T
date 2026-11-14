package cs

import (
	"encoding/json"
	"slices"
)

// Completed armies return useful individual holds to civilian logistics rather
// than waiting for one colony to fill a huge merged fleet again.
func (t *turnGenerator) disperseEmptyLandingTransports(p *Player, leader *Fleet) {
	t.disperseLandingTransports(p, leader, false)
}

func (t *turnGenerator) disperseLandingTransports(p *Player, leader *Fleet, loaded bool) {
	colonyGroup := leader.GetTag("landing-colonizer-roles") != "" || colonizationMode(p.Automation.FleetRoles[leader.Num]) != ""
	if leader.Delete || leader.CargoMass() != 0 && !colonyGroup && !loaded || leader.Spec.HasWeapons || !landingTransport(p, leader) {
		return
	}
	roles := map[int]string{}
	_ = json.Unmarshal([]byte(leader.GetTag("landing-colonizer-roles")), &roles)
	restore := func(f *Fleet) {
		delete(f.Tags, "landing-colonizer-roles")
		if !p.CanAutomateColonizer(f) {
			p.Automation.FleetRoles[f.Num] = "freighter"
			delete(f.Tags, colonizationTag)
			return
		}
		role := roles[f.Tokens[0].DesignNum]
		if role == "" {
			role = p.Automation.FleetRoles[f.Num]
		}
		if colonizationMode(role) == "" {
			role = "colonizer:best"
		}
		p.Automation.FleetRoles[f.Num] = role
		if f.Cargo.Colonists > 0 {
			writeColonizationJob(f, &colonizationJob{Mode: colonizationMode(role), Kind: "passengers", Status: "Landing cancelled; retaining passengers for colonist transport or a new colony"})
			return
		}
		t.completedColonizerTask(p, f, &colonizationJob{Mode: colonizationMode(role)})
	}
	if leader.Spec.TotalShips < 2 {
		restore(leader)
		return
	}
	children, err := NewOrderer().SplitAll(&t.game.Rules, p, t.game.getFleets(p.Num), leader)
	if err != nil {
		return
	}
	for _, child := range children {
		t.game.Fleets = append(t.game.Fleets, child)
		_ = t.game.Universe.addFleet(child)
		p.Automation.FleetRoles[child.Num] = "freighter"
		holdAutomaticFleet(p, child, "Landing completed; transport available for colony support")
		restore(child)
	}
	restore(leader)
}

// A failed occupation can still be useful, but never send tiny or mostly
// annihilated drops. This estimate uses reported ground data, not hidden state.
func usefulAttritionLanding(rules *Rules, p *Player, target *Planet, troops int) bool {
	policy := p.Automation.transportPolicy()
	if policy.DisableAttrition || troops < policy.AttritionMinimumPeople/100 || target == nil || target.Cargo.Colonists <= 0 {
		return false
	}
	survival := max(0, 1-target.Spec.DefenseCoverage*rules.InvasionDefenseCoverageFactor)
	kills := float64(troops) * p.Race.Spec.InvasionAttackBonus * survival / reportedInvasionDefense(rules)
	return kills >= float64(max(500, target.Cargo.Colonists/10)) && kills >= float64(troops)/5
}

// A loaded army need not remain tied to an unavailable orbital assault. Only
// choose a nearby reported base-free colony it can provision for occupation;
// the new operation still assembles, scouts and passes the orbital checks.
func (t *turnGenerator) retargetBlockedLanding(p *Player, leader *Fleet, op *landingOperation) bool {
	if p.Automation.transportPolicy().DisableRetarget || leader.Position != op.Rally || leader.Cargo.Colonists == 0 {
		return false
	}
	candidates := []*Planet{}
	for _, target := range p.PlanetIntels {
		if !t.landingPrepared(p, target) || target.Num == op.Target || target.Spec.HasStarbase || !invasionCandidate(p, target) || t.landingTargetClaimed(p, target.Num) || t.landingRequirement(p, target) > leader.Cargo.Colonists || leader.Position.DistanceTo(target.Position) > 2*leader.StrategicSpeed(automaticMaxWarp(p, leader)) {
			continue
		}
		candidates = append(candidates, target)
	}
	slices.SortFunc(candidates, func(a, b *Planet) int {
		if rank := expansionPriority(p, b) - expansionPriority(p, a); rank != 0 {
			return rank
		}
		da, db := leader.Position.DistanceTo(a.Position), leader.Position.DistanceTo(b.Position)
		if da < db {
			return -1
		}
		if da > db {
			return 1
		}
		return a.Num - b.Num
	})
	for i, target := range candidates {
		if i >= 8 {
			break
		}
		direction := leader.Position.Subtract(target.Position).Normalized()
		if direction.LengthSquared() == 0 {
			continue
		}
		radius := max(1.0, min(35.0, leader.StrategicSpeed(automaticCruiseWarp(p, leader))-2))
		rally := target.Position.ToFloat64().Add(direction.Scale(radius)).ToInt(true)
		if rally == target.Position || hostileOrbitalBase(p, rally) {
			continue
		}
		members := t.landingMembers(p, leader)
		ready := true
		for _, f := range members {
			route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, rally, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 4, Fast: true})
			if route == nil || route.WaitForFuel {
				ready = false
				break
			}
		}
		if !ready {
			continue
		}
		t.releaseLandingBaseRecon(p, op)
		op.Target, op.Owner, op.Need = target.Num, target.PlayerNum, t.landingArmyNeed(p, target)
		op.Started = t.game.Year
		op.BaseRallyBy = 0
		op.BaseRallyRetryYear = 0
		op.Rally, op.Phase, op.Base, op.ScanYear = rally, "gather", false, 0
		for _, f := range members {
			if !landingManaged(f) {
				continue
			}
			if job := logisticsWork(f); job != nil {
				job.DeliveryNum = target.Num
				if job.Kind == "landing-rally" {
					job.Position = rally
				}
				job.Status = "Landing redirected from unavailable base assault to nearby colony"
				saveLogisticsWork(f, job)
			}
			if job := logisticsWork(f); job == nil || job.Kind != "landing-load" {
				t.routeLanding(p, f, rally, MapObjectTarget{}, false, 4)
			}
		}
		saveLandingOperation(leader, op)
		return true
	}
	return false
}
