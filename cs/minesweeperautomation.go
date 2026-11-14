package cs

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

// This doctrine follows equipment, never the player's project name. Multiple
// Gatling Guns are allowed, but any other weapon/bomb makes a different design.
func dedicatedMinesweeperSpec(spec ShipDesignSpec) bool {
	if spec.Starbase || spec.MineSweep <= 0 || len(spec.WeaponSlots) == 0 || len(spec.Bombs)+len(spec.SmartBombs)+len(spec.RetroBombs) > 0 {
		return false
	}
	for _, slot := range spec.WeaponSlots {
		if slot.Quantity <= 0 || slot.HullComponent != GatlingGun.Name {
			return false
		}
	}
	return true
}

func fleetHasDedicatedMinesweeper(p *Player, f *Fleet) bool {
	if p.AIControlled {
		return false
	}
	for _, token := range f.Tokens {
		if d := p.GetDesign(token.DesignNum); token.Quantity > 0 && d != nil && dedicatedMinesweeperSpec(d.Spec) {
			return true
		}
	}
	return dedicatedMinesweeperSpec(f.Spec.ShipDesignSpec)
}

func allDedicatedMinesweepers(p *Player, f *Fleet) bool {
	if len(f.Tokens) == 0 {
		return false
	}
	for _, token := range f.Tokens {
		d := p.GetDesign(token.DesignNum)
		if token.Quantity > 0 && (d == nil || !dedicatedMinesweeperSpec(d.Spec)) {
			return false
		}
	}
	return true
}

func effectiveDesignAutoRole(spec ShipDesignSpec) string {
	if spec.DefaultAutoRole == "manual" {
		return ""
	}
	if dedicatedMinesweeperSpec(spec) {
		return "minesweeper"
	}
	return NormalizeAutomaticRole(spec.DefaultAutoRole)
}

// Retire stale combat assignments without touching manual fleets or interrupting
// refit/safety orders. New construction receives this role immediately.
func (t *turnGenerator) prepareDedicatedMinesweepers() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			restoreMinesweeperBattlePlan(p, f)
			if p.AIControlled {
				continue
			}
			role := p.Automation.FleetRoles[f.Num]
			if f.Delete || f.Starbase || role == "" || role == "minesweeper" || !allDedicatedMinesweepers(p, f) || p.FleetInActiveAssembly(f) {
				continue
			}
			if plan := readFuelNetworkPlan(f); plan != nil {
				t.closeFuelNetwork(p, f, plan, true)
			}
			if tanker := readTankerJob(f); tanker != nil {
				t.releaseTankerCustomers(p, f, tanker)
				delete(f.Tags, tankerJobTag)
			}
			restorePatrolBattlePlan(p, f)
			for _, tag := range []string{patrolTag, patrolSupportTag, escortTag, bomberBorrowTag, containmentTag, frontierPatrolTag, anticipatedSalvageTag, logisticsTag, scoutReconTag, scoutMissionTag, scoutTag} {
				delete(f.Tags, tag)
			}
			p.Automation.FleetRoles[f.Num] = "minesweeper"
			if f.InAutomaticSafety() {
				var safety automaticSafetyJob
				if json.Unmarshal([]byte(f.GetTag(safetyTag)), &safety) == nil {
					safety.Orders = FleetOrders{BattlePlanNum: f.BattlePlanNum, Waypoints: []Waypoint{{Position: f.Position}}}
					raw, _ := json.Marshal(safety)
					f.SetTag(safetyTag, string(raw))
				}
			}
			if work := readMaintenance(f); work != nil {
				work.OriginalRole = "minesweeper"
				writeMaintenance(f, work)
			}
			if !f.InDesignMaintenance() && !f.InAutomaticSafety() && f.GetTag("maintenance-pending") == "" {
				holdAutomaticFleet(p, f, "Gatling-only minesweeper: combat assignment released")
			}
		}
	}
}

const minesweeperTargetTag = "minesweeper-target"

func minefieldAssignmentKey(field *Minefield) string {
	return fmt.Sprintf("%d:%d", field.PlayerNum, field.Num)
}

// Only known colonies anchor the front; enemy territory never attracts a sweep.
func minesweeperTerritoryDistance(p *Player, field *Minefield) float64 {
	distance := math.Inf(1)
	for _, planet := range p.PlanetIntels {
		if planet != nil && !planet.Delete && (planet.PlayerNum == p.Num || (planet.PlayerNum > 0 && p.IsFriend(planet.PlayerNum))) {
			distance = min(distance, max(0, planet.Position.DistanceTo(field.Position)-field.Radius()))
		}
	}
	if math.IsInf(distance, 1) {
		return 0
	}
	return distance
}

func reportedHostileMinefield(p *Player, m *Minefield) bool {
	return m != nil && !m.Delete && m.NumMines > 0 && !m.Detonate && m.PlayerNum != p.Num && !p.IsFriend(m.PlayerNum)
}

func (t *turnGenerator) planMinesweeperAutomation() {
	for _, p := range t.automaticPlayers() {
		available := []*Fleet{}
		for _, f := range t.game.getFleets(p.Num) {
			if !automaticMinesweeper(p, f) || f.Delete || f.Starbase || f.InDesignMaintenance() || f.InAutomaticSafety() || f.GetTag("maintenance-pending") != "" || p.FleetInActiveAssembly(f) || fuelNetworkManaged(f) || f.GetTag(tankerAwaitTag) != "" {
				continue
			}
			delete(f.Tags, patrolMineSweepTag)
			setMinesweeperBattlePlan(p, f)
			t.surveyMinesweeperField(p, f)
			available = append(available, f)
		}
		fields := slices.DeleteFunc(slices.Clone(p.MinefieldIntels), func(m *Minefield) bool {
			return !reportedHostileMinefield(p, m)
		})
		slices.SortFunc(fields, func(a, b *Minefield) int {
			da, db := minesweeperTerritoryDistance(p, a), minesweeperTerritoryDistance(p, b)
			if da < db {
				return -1
			}
			if da > db {
				return 1
			}
			if a.PlayerNum != b.PlayerNum {
				return a.PlayerNum - b.PlayerNum
			}
			return a.Num - b.Num
		})
		assigned := map[int]bool{}
		reserved := map[string][]sweepArrival{}
		// Cover fields in territory order before spending spare fleets on reinforcement.
		for _, field := range fields {
			key := minefieldAssignmentKey(field)
			candidates := slices.Clone(available)
			slices.SortStableFunc(candidates, func(a, b *Fleet) int {
				keepsA := a.GetTag(minesweeperTargetTag) == key
				keepsB := b.GetTag(minesweeperTargetTag) == key
				if keepsA != keepsB {
					if keepsA {
						return -1
					}
					return 1
				}
				return cmp.Compare(a.Position.DistanceTo(field.Position), b.Position.DistanceTo(field.Position))
			})
			for _, f := range candidates {
				if assigned[f.Num] {
					continue
				}
				if route := t.minesweeperFieldRoute(p, f, field); route != nil {
					t.applyMinesweeperRoute(p, f, field, route)
					assigned[f.Num] = true
					reserved[key] = []sweepArrival{newSweepArrival(&t.game.Rules, p, field, f, route)}
					break
				}
			}
		}
		for _, field := range fields {
			group := reserved[minefieldAssignmentKey(field)]
			if len(group) != 0 && field.NumMines > group[0].capacity {
				t.reinforceMinefield(p, field, group, available, assigned)
			}
		}
		for _, f := range available {
			if assigned[f.Num] {
				continue
			}
			delete(f.Tags, minesweeperTargetTag)
			if len(assigned) > 0 {
				holdAutomaticFleet(p, f, "Minesweeper reserve: no safe reinforcement would shorten clearing")
				continue
			}
			var best *expeditionRoute
			for _, dock := range expeditionStations(p, f) {
				route := planAutomaticRoute(&t.game.Rules, p, []*Fleet{f}, dock.Position, dock.ToTarget(), automaticRoutePolicy{EconomicalPatrol: true, MaxTurns: 20})
				if route != nil && !route.WaitForFuel && (best == nil || route.Turns < best.Turns) {
					best = route
				}
			}
			if best != nil {
				applyExpeditionRoute(p, f, best)
				status := "No reported hostile minefields; returning to a dock, not sweeping"
				if len(fields) > 0 {
					status = "Reported fields are unreachable with current fuel or route; returning to a dock"
				}
				f.SetTag(fuelStatusTag, status)
			} else {
				holdAutomaticFleet(p, f, "Minesweeper idle: no feasible route to a reported hostile field or dock")
				requestAutomaticRescue(p, f, true)
			}
		}
	}
}

const minesweeperBattlePlanTag = "minesweeper-battle-plan"

func setMinesweeperBattlePlan(p *Player, f *Fleet) {
	var saved collectionBattlePlan
	if json.Unmarshal([]byte(f.GetTag(minesweeperBattlePlanTag)), &saved) == nil && saved.Assigned == f.BattlePlanNum {
		return
	}
	plan := BattlePlan{Num: p.GetNextBattlePlanNum(), Name: "Minesweeper retreat", Tactic: BattleTacticDisengage, PrimaryTarget: BattleTargetNone, SecondaryTarget: BattleTargetNone, AttackWho: BattleAttackWhoEnemiesAndNeutrals}
	if plan.Num == 0 {
		plan.Num = 1
	}
	found := false
	for _, existing := range p.BattlePlans {
		if existing.Num > 0 && existing.Name == plan.Name && existing.Tactic == plan.Tactic && existing.PrimaryTarget == BattleTargetNone && existing.SecondaryTarget == BattleTargetNone && existing.AttackWho == plan.AttackWho {
			plan = existing
			found = true
			break
		}
	}
	if !found {
		p.BattlePlans = append(p.BattlePlans, plan)
	}
	raw, _ := json.Marshal(collectionBattlePlan{Previous: f.BattlePlanNum, Assigned: plan.Num})
	f.SetTag(minesweeperBattlePlanTag, string(raw))
	f.BattlePlanNum = plan.Num
	f.battlePlan = &plan
}

func restoreMinesweeperBattlePlan(p *Player, f *Fleet) {
	if p.Automation.FleetRoles[f.Num] == "minesweeper" {
		return
	}
	var saved collectionBattlePlan
	if json.Unmarshal([]byte(f.GetTag(minesweeperBattlePlanTag)), &saved) != nil {
		return
	}
	if f.BattlePlanNum == saved.Assigned {
		f.BattlePlanNum = saved.Previous
		plan := patrolPlanForFleet(p, f)
		f.battlePlan = &plan
	}
	delete(f.Tags, minesweeperBattlePlanTag)
}
