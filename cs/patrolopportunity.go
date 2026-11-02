package cs

import (
	"math"
	"sort"
)

func patrolInterceptPosition(enemy *Fleet) Vector {
	position := enemy.Position
	if enemy.WarpSpeed > 0 && enemy.WarpSpeed <= 10 {
		delta := enemy.Heading.Scale(enemy.observedStrategicSpeed())
		position.X += int(math.Round(delta.X))
		position.Y += int(math.Round(delta.Y))
	}
	return position
}

// Local opportunities inside the colony network beat long outward chases.
// The prediction uses public heading/speed, never the enemy's private orders.
func patrolOpportunityScore(p *Player, f, enemy *Fleet) float64 {
	position := patrolInterceptPosition(enemy)
	distance := f.Position.DistanceTo(position)
	inside := math.Inf(1)
	for _, colony := range p.PlanetIntels {
		if colony != nil && colony.OwnedBy(p.Num) {
			inside = min(inside, colony.Position.DistanceTo(position))
		}
	}
	if math.IsInf(inside, 1) {
		inside = 0
	}
	score := (1 + distance) * (1 + min(300, inside)/100)
	if patrolReportedTransport(p, enemy) {
		if enemyApproachingColony(p, enemy) {
			return score * .001
		}
		return score * .25
	}
	if patrolTargetPriority(p, enemy, false) == 3 {
		return score * .5
	}
	return score
}

// Opportunistic hunters reconsider a long chase each year. Fresh one-turn
// opportunities beat distant, receding targets, but not a colony's bomber threat.
func (t *turnGenerator) retargetPatrolOpportunity(p *Player, f *Fleet) {
	job := readPatrolJob(f)
	if job.Leader != f.Num || f.Delete || !patrolAvailable(p, f) {
		return
	}
	current := patrolEnemy(p, job)
	if current == nil || enemyApproachingColony(p, current) && patrolReportedBomber(p, current) && !nearbyLandingThreat(p, f) {
		return
	}
	var best *Fleet
	bestScore := patrolOpportunityScore(p, f, current)
	candidates := []*Fleet{}
	scores := map[*Fleet]float64{}
	gateReach := false
	for _, base := range expeditionStations(p, f) {
		gateReach = gateReach || automaticGateCompatible(p, []*Fleet{f}, base)
	}
	localReach := max(float64(150), 2*f.StrategicSpeed(automaticMaxWarp(p, f)))
	for _, candidate := range p.FleetIntels {
		if candidate == nil || candidate == current || candidate.Delete || candidate.Starbase || candidate.ReportAge != 0 || candidate.PlayerNum == p.Num || p.IsFriend(candidate.PlayerNum) {
			continue
		}
		if !gateReach && f.Position.DistanceTo(patrolInterceptPosition(candidate)) > localReach {
			continue
		}
		score := patrolOpportunityScore(p, f, candidate)
		if declinedPatrolTarget(f, candidate, t.game.Year) || score >= bestScore*0.9 {
			continue
		}
		scores[candidate] = score
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := scores[candidates[i]], scores[candidates[j]]
		if a != b {
			return a < b
		}
		if candidates[i].PlayerNum != candidates[j].PlayerNum {
			return candidates[i].PlayerNum < candidates[j].PlayerNum
		}
		return candidates[i].Num < candidates[j].Num
	})
	for _, candidate := range candidates[:min(8, len(candidates))] {
		if patrolPower(f) < patrolThreat(p, candidate)*3/2+1 {
			continue
		}
		d := f.Position.DistanceTo(patrolInterceptPosition(candidate))
		assigned := false
		for _, other := range t.game.getFleets(p.Num) {
			otherJob := readPatrolJob(other)
			if other != f && !other.Delete && otherJob.Leader == other.Num && otherJob.Enemy == candidate.PlayerNum && otherJob.Target == candidate.Num && other.Position.DistanceTo(candidate.Position) <= d && patrolPower(other) >= patrolThreat(p, candidate)*3/2+1 {
				if patrolDesiredShips(p, candidate) > patrolGroupShips(t.game.getFleets(p.Num), other) {
					t.joinPatrolOpportunity(p, f, other, candidate)
					return
				}
				assigned = true
				break
			}
		}
		if assigned {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, patrolInterceptPosition(candidate), MapObjectTarget{}, automaticRoutePolicy{AllowThreat: true, Fast: true, MaxTurns: 2, MinWarp: automaticCruiseWarp(p, f)})
		if route == nil || route.WaitForFuel {
			continue
		}
		best = candidate
		break
	}
	if best == nil {
		return
	}
	availableShips := patrolGroupShips(t.game.getFleets(p.Num), f)
	for _, other := range t.game.getFleets(p.Num) {
		if other != f && patrolAvailable(p, other) && readPatrolJob(other).Leader == 0 && other.Position.DistanceTo(f.Position) <= 100 {
			availableShips += other.Spec.TotalShips
		}
	}
	for _, member := range t.game.getFleets(p.Num) {
		memberJob := readPatrolJob(member)
		if memberJob.Leader == f.Num {
			memberJob.Enemy, memberJob.Target = best.PlayerNum, best.Num
			memberJob.RecruitPower = 0
			memberJob.RequiredShips = min(availableShips, patrolDesiredShips(p, best))
			memberJob.GatheringSince = t.game.Year
			memberJob.Status = "Switching from distant pursuit to nearby interception: " + best.Name
			savePatrolJob(member, memberJob)
		}
	}
}

// Multiple confirmed unarmed hulls are worth a short local rendezvous: enough
// independent weapons to finish the kill, without waiting for a distant armada.
func patrolDesiredShips(p *Player, enemy *Fleet) int {
	if patrolReportedCombatant(p, enemy) {
		return 1
	}
	count := enemy.Spec.TotalShips
	if count == 0 {
		for _, token := range enemy.Tokens {
			count += token.Quantity
		}
	}
	return max(1, min(3, count))
}

func patrolGroupShips(fleets []*Fleet, leader *Fleet) int {
	count := 0
	for _, f := range fleets {
		if !f.Delete && (f == leader || readPatrolJob(f).Leader == leader.Num) {
			count += f.Spec.TotalShips
		}
	}
	return count
}
func (t *turnGenerator) joinPatrolOpportunity(p *Player, f, leader, enemy *Fleet) {
	own := readPatrolJob(f)
	group := []*Fleet{f}
	if own.Leader == f.Num {
		group = nil
		for _, member := range t.game.getFleets(p.Num) {
			if !member.Delete && readPatrolJob(member).Leader == f.Num {
				group = append(group, member)
			}
		}
	}
	job := readPatrolJob(leader)
	for _, member := range group {
		if p.Automation.FleetRoles[member.Num] == "escort" {
			member.SetTag(patrolSupportTag, "escort")
			p.Automation.FleetRoles[member.Num] = "patrol"
		}
		if job.SupportDesigns == nil {
			job.SupportDesigns = map[int]int{}
		}
		if member.GetTag(patrolSupportTag) == "escort" {
			for _, token := range member.Tokens {
				job.SupportDesigns[token.DesignNum] += token.Quantity
			}
		}
		next := readPatrolJob(member)
		next.Leader = leader.Num
		next.Enemy = enemy.PlayerNum
		next.Target = enemy.Num
		next.Status = "Joining nearby patrol for a decisive interception"
		savePatrolJob(member, next)
	}
	job.RequiredShips = min(patrolDesiredShips(p, enemy), patrolGroupShips(t.game.getFleets(p.Num), leader))
	savePatrolJob(leader, job)
	t.advancePatrol(p, leader)
}

// A gate can put a hunter beside a local target even when the fleet is far
// away geometrically. Use native mass/cargo/range rules, not a gate icon.
func patrolGateReachable(p *Player, f, enemy *Fleet) bool {
	source := p.GetPlanetIntel(f.OrbitingPlanetNum)
	if !f.Spec.CanJump && (source == nil || !f.CanJump(p, 0, source, source)) {
		return false
	}
	position := patrolInterceptPosition(enemy)
	for _, base := range p.PlanetIntels {
		if base != nil && base.OwnedBy(p.Num) && base.Spec.HasStargate && base.Position.DistanceTo(position) <= f.StrategicSpeed(automaticMaxWarp(p, f)) && f.CanJump(p, f.Position.DistanceTo(base.Position), source, base) {
			return true
		}
	}
	return false
}
