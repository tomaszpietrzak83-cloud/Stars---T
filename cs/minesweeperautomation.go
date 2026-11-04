package cs

import "encoding/json"

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
	return spec.DefaultAutoRole
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

func (t *turnGenerator) planMinesweeperAutomation() {
	for _, p := range t.automaticPlayers() {
		for _, f := range t.game.getFleets(p.Num) {
			if p.Automation.FleetRoles[f.Num] != "minesweeper" || f.Delete || f.Starbase || f.InDesignMaintenance() || f.InAutomaticSafety() || f.GetTag("maintenance-pending") != "" || p.FleetInActiveAssembly(f) || fuelNetworkManaged(f) || f.GetTag(tankerAwaitTag) != "" {
				continue
			}
			setMinesweeperBattlePlan(p, f)
			if t.patrolSweepObstacle(p, f) {
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
				f.SetTag(fuelStatusTag, "Minesweeper returning to a safe fuel dock; no reachable hostile field")
			} else {
				holdAutomaticFleet(p, f, "Minesweeper: no confirmed safe field route; awaiting fuel assistance")
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
