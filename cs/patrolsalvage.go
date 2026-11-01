package cs

import (
	"encoding/json"
	"math"
	"slices"
)

const patrolSalvageTag = "patrol-salvage"
const anticipatedSalvageTag = "anticipated-battle-salvage"

type anticipatedSalvage struct {
	Position Vector
	Until    int
}

// Only ships that actually met in battle influence the reinforcement request.
func (t *turnGenerator) markSurvivingPatrolTarget(record *BattleRecord) {
	for _, f := range record.fleets {
		p := t.game.getPlayer(f.PlayerNum)
		if p == nil {
			continue
		}
		job := readPatrolJob(f)
		if f.Delete || p.Automation.FleetRoles[f.Num] != "patrol" || job.Leader != f.Num {
			continue
		}
		for _, enemy := range record.fleets {
			if enemy.PlayerNum != job.Enemy || enemy.Num != job.Target || enemy.Delete {
				continue
			}
			survivors := 0
			for _, token := range enemy.Tokens {
				survivors += token.Quantity
			}
			if survivors > 0 {
				job.RecruitPower = min(1_000_000_000, max(job.RecruitPower+patrolPower(f), patrolPower(f)*2))
				job.Status = "Enemy survived interception; nearby patrols requested as reinforcements"
				savePatrolJob(f, job)
			}
		}
	}
}

func (t *turnGenerator) planPatrolSalvage() {
	for _, p := range t.automaticPlayers() {
		if p.Automation.DisableSalvage {
			continue
		}
		// Release speculative reservations promptly, even if a battle produces no debris.
		for _, f := range t.game.getFleets(p.Num) {
			var pending anticipatedSalvage
			if json.Unmarshal([]byte(f.GetTag(anticipatedSalvageTag)), &pending) != nil {
				continue
			}
			available := false
			for _, salvage := range p.SalvageIntels {
				if salvage != nil && !salvage.Delete && salvage.Position == pending.Position && salvage.Cargo.ToMineral().Total() > 0 {
					available = true
					break
				}
			}
			if available || t.game.Year > pending.Until {
				delete(f.Tags, anticipatedSalvageTag)
			}
		}
		claimed := map[int]bool{}
		for _, f := range t.game.getFleets(p.Num) {
			if job := logisticsWork(f); job != nil && job.Kind == "salvage" {
				claimed[job.TargetNum] = true
			}
		}
		collectors := func(position Vector, amount int) (*Fleet, *expeditionRoute) {
			var best *Fleet
			var route *expeditionRoute
			score := math.Inf(1)
			for _, f := range t.game.getFleets(p.Num) {
				role := p.Automation.FleetRoles[f.Num]
				if f.Delete || f.Starbase || f.InDesignMaintenance() || f.InAutomaticSafety() || f.GetTag("maintenance-pending") != "" || p.FleetInActiveAssembly(f) || fuelNetworkManaged(f) || readTankerJob(f) != nil || f.GetTag(tankerAwaitTag) != "" || f.GetTag(anticipatedSalvageTag) != "" || f.CargoMass() > 0 || f.Spec.CargoCapacity <= 0 || logisticsWork(f) != nil {
					continue
				}
				if role != "freighter" && (role != "patrol" || !patrolAvailable(p, f) || readPatrolJob(f).Leader != 0) {
					continue
				}
				candidate := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 5})
				if candidate == nil || candidate.WaitForFuel {
					continue
				}
				// Favor the smallest sufficient hold; oversized and undersized collectors
				// are both penalized. Nearby small patrol holds can beat large freighters.
				value := float64(candidate.Turns)*10 + math.Abs(math.Log2(float64(f.Spec.CargoCapacity)/float64(max(1, amount))))*20
				if f.Spec.CargoCapacity < amount {
					value += 30
				}
				if value < score || value == score && (best == nil || f.Num < best.Num) {
					best, route, score = f, candidate, value
				}
			}
			return best, route
		}
		salvages := slices.Clone(p.SalvageIntels)
		slices.SortStableFunc(salvages, func(a, b *Salvage) int {
			if a == nil {
				return 1
			}
			if b == nil {
				return -1
			}
			return b.Cargo.ToMineral().Total() - a.Cargo.ToMineral().Total()
		})
		for _, salvage := range salvages {
			if salvage == nil || salvage.Delete || claimed[salvage.Num] || salvage.Cargo.ToMineral().Total() == 0 {
				continue
			}
			f, route := collectors(salvage.Position, salvage.Cargo.ToMineral().Total())
			if f == nil {
				continue
			}
			if p.Automation.FleetRoles[f.Num] == "patrol" {
				f.SetTag(patrolSalvageTag, "1")
			}
			saveLogisticsWork(f, &logisticsJob{Kind: "salvage", TargetNum: salvage.Num, Position: salvage.Position, Status: "Collecting nearby debris with a size-matched cargo hold"})
			applyExpeditionRoute(p, f, route)
			claimed[salvage.Num] = true
		}
		for _, hunter := range t.game.getFleets(p.Num) {
			job := readPatrolJob(hunter)
			enemy := patrolEnemy(p, job)
			if hunter.Delete || job.Leader != hunter.Num || !patrolCanAttack(p, hunter) || enemy == nil || patrolReportedCombatant(p, enemy) || len(hunter.Waypoints) < 2 {
				continue
			}
			position := hunter.Waypoints[len(hunter.Waypoints)-1].Position
			reserved := false
			for _, f := range t.game.getFleets(p.Num) {
				var pending anticipatedSalvage
				if json.Unmarshal([]byte(f.GetTag(anticipatedSalvageTag)), &pending) == nil && pending.Position == position {
					reserved = true
					break
				}
			}
			if reserved {
				continue
			}
			estimate := 0
			for _, token := range enemy.Tokens {
				if d := p.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum); d != nil {
					estimate += max(1, d.Spec.Cost.ToMineral().Total()) * token.Quantity / 3
				}
			}
			f, route := collectors(position, max(1, estimate))
			if f == nil || f == hunter {
				continue
			}
			data, _ := json.Marshal(anticipatedSalvage{position, t.game.Year + 2})
			f.SetTag(anticipatedSalvageTag, string(data))
			applyExpeditionRoute(p, f, route)
			f.SetTag(fuelStatusTag, "Approaching a superior patrol interception to collect debris if the battle produces it")
		}
	}
}
