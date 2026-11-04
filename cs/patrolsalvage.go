package cs

import (
	"encoding/json"
	"math"
	"slices"
	"strconv"
)

const patrolSalvageTag = "patrol-salvage"
const anticipatedSalvageTag = "anticipated-battle-salvage"

type anticipatedSalvage struct {
	Staging  bool `json:",omitempty"`
	Position Vector
	Until    int
	Hunter   int `json:",omitempty"`
	Enemy    int `json:",omitempty"`
	Target   int `json:",omitempty"`
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
				if p.patrolEvaluator == nil {
					job.RecruitPower = min(1_000_000_000, max(job.RecruitPower+patrolPower(f), patrolPower(f)*2))
				} else {
					job.RecruitPower = 0
				}
				job.Status = "Enemy survivors: retain pack, refresh reports and reassess before continuing"
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
		t.assignLocalDebrisPickups(p)
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
			hunter := t.game.getFleet(p.Num, pending.Hunter)
			valid := hunter != nil && !hunter.Delete && len(hunter.Waypoints) > 1 && readPatrolJob(hunter).Enemy == pending.Enemy && readPatrolJob(hunter).Target == pending.Target && (pending.Staging || patrolCanAttack(p, hunter))
			if valid && !pending.Staging {
				position := hunter.Waypoints[len(hunter.Waypoints)-1].Position
				route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{hunter}, position, MapObjectTarget{}, automaticRoutePolicy{MaxTurns: 1, Fast: true, AllowThreat: true, MinWarp: automaticCruiseWarp(p, hunter)})
				valid = route != nil && !route.WaitForFuel
			}
			if available || !valid || t.game.Year > pending.Until {
				delete(f.Tags, anticipatedSalvageTag)
				if !f.Delete && !f.InAutomaticSafety() && !f.InDesignMaintenance() && logisticsWork(f) == nil {
					holdAutomaticFleet(p, f, "Battle collection reservation ended; selecting a transport task")
				}
			}
		}
		claimed := map[int]bool{}
		priorityContact := false
		for _, enemy := range p.FleetIntels {
			priorityContact = priorityContact || enemy != nil && !enemy.Delete && enemy.ReportAge == 0 && priorityFleet(p, enemy)
		}
		for _, f := range t.game.getFleets(p.Num) {
			if job := logisticsWork(f); job != nil && job.Kind == "salvage" {
				if current := t.currentLogisticsWork(p, f); current != nil {
					claimed[current.TargetNum] = true
				}
			}
		}
		collectors := func(position Vector, amount, salvageNum int) (*Fleet, *expeditionRoute) {
			var best *Fleet
			var route *expeditionRoute
			score := math.Inf(1)
			for _, f := range t.game.getFleets(p.Num) {
				role := p.Automation.FleetRoles[f.Num]
				work := logisticsWork(f)
				protectedWork := work != nil && !(salvageNum != 0 && work.Kind == "return" && f.CargoMass() == 0)
				if salvageNum == 0 && f.GetTag(containmentTag) != "" {
					job := readContainment(f)
					if job == nil || job.Kind != "collector" {
						continue
					}
				}
				if f.Delete || f.Starbase || f.InDesignMaintenance() || f.InAutomaticSafety() || f.GetTag("maintenance-pending") != "" || p.FleetInActiveAssembly(f) || fuelNetworkManaged(f) || readTankerJob(f) != nil || f.GetTag(tankerAwaitTag) != "" || salvageNum == 0 && f.GetTag(anticipatedSalvageTag) != "" || salvageNum != 0 && f.GetTag("empty-salvage") == strconv.Itoa(salvageNum) || f.CargoMass() > 0 || f.Spec.CargoCapacity <= 0 || protectedWork {
					continue
				}
				if role != "freighter" && (role != "patrol" || !patrolAvailable(p, f) || readPatrolJob(f).Leader != 0) {
					continue
				}
				if role == "patrol" && priorityContact {
					continue // Reserve combat ships for the player's explicit destruction order.
				}
				policy := automaticRoutePolicy{MaxTurns: 5, Fast: true}
				if salvageNum == 0 {
					policy = automaticRoutePolicy{MaxTurns: 1, Fast: true, AllowThreat: true, MinWarp: automaticCruiseWarp(p, f)}
				}
				candidate := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, MapObjectTarget{}, policy)
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
			f, route := collectors(salvage.Position, salvage.Cargo.ToMineral().Total(), salvage.Num)
			if f == nil {
				continue
			}
			// Actual debris takes precedence over speculative battle reservations.
			// Existing cargo deliveries and pickups remain protected above.
			delete(f.Tags, anticipatedSalvageTag)
			delete(f.Tags, containmentTag)
			setCollectionBattlePlan(p, f)
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
			if hunter.Delete || job.Leader != hunter.Num || enemy == nil || len(hunter.Waypoints) < 2 {
				continue
			}
			position := hunter.Waypoints[len(hunter.Waypoints)-1].Position
			orbital := false
			for _, planet := range p.PlanetIntels {
				orbital = orbital || planet != nil && planet.OwnedBy(p.Num) && planet.Position == enemy.Position
			}
			if orbital {
				continue
			}
			if !patrolCanAttack(p, hunter) {
				t.stagePatrolCollector(p, hunter, enemy, position)
				continue
			}
			policy := automaticRoutePolicy{MaxTurns: 1, Fast: true, AllowThreat: true, MinWarp: automaticCruiseWarp(p, hunter)}
			arrival := planAutomaticRoute(&t.game.Rules, p, []*Fleet{hunter}, position, MapObjectTarget{}, policy)
			if arrival == nil || arrival.WaitForFuel {
				t.stagePatrolCollector(p, hunter, enemy, position)
				continue
			}
			reserved := false
			for _, f := range t.game.getFleets(p.Num) {
				var pending anticipatedSalvage
				if json.Unmarshal([]byte(f.GetTag(anticipatedSalvageTag)), &pending) == nil && (pending.Hunter == hunter.Num || pending.Position == position) {
					if f.Delete || f.InAutomaticSafety() || f.InDesignMaintenance() || f.CargoMass() > 0 || logisticsWork(f) != nil {
						delete(f.Tags, anticipatedSalvageTag)
						continue
					}
					policy.MinWarp = automaticCruiseWarp(p, f)
					route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, position, MapObjectTarget{}, policy)
					if route == nil || route.WaitForFuel {
						delete(f.Tags, anticipatedSalvageTag)
						continue
					}
					pending.Position, pending.Until = position, t.game.Year+1
					pending.Staging = false
					pending.Hunter, pending.Enemy, pending.Target = hunter.Num, job.Enemy, job.Target
					data, _ := json.Marshal(pending)
					f.SetTag(anticipatedSalvageTag, string(data))
					delete(f.Tags, containmentTag)
					setCollectionBattlePlan(p, f)
					applyExpeditionRoute(p, f, route)
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
			f, route := collectors(position, max(1, estimate), 0)
			if f == nil || f == hunter {
				continue
			}
			data, _ := json.Marshal(anticipatedSalvage{Position: position, Until: t.game.Year + 1, Hunter: hunter.Num, Enemy: job.Enemy, Target: job.Target})
			f.SetTag(anticipatedSalvageTag, string(data))
			delete(f.Tags, containmentTag)
			setCollectionBattlePlan(p, f)
			applyExpeditionRoute(p, f, route)
			f.SetTag(fuelStatusTag, "Approaching a superior patrol interception to collect debris if the battle produces it")
		}
	}
}
