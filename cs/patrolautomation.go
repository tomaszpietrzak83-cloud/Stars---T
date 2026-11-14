package cs

import (
	"encoding/json"
	"maps"
	"math"
	"slices"
	"sort"
)

const patrolTag = "automatic-patrol"
const patrolSupportTag = "patrol-support-role"

type patrolJob struct {
	Minelayer      bool        `json:"minelayer,omitempty"`
	ReliefStaging  bool        `json:"reliefStaging,omitempty"`
	ReliefYear     int         `json:"reliefYear,omitempty"`
	Provisional    bool        `json:"provisional,omitempty"`
	Rally          Vector      `json:"rally,omitzero"`
	RequiredShips  int         `json:"requiredShips,omitempty"`
	RecruitPower   int         `json:"recruitPower,omitempty"`
	GatheringSince int         `json:"gatheringSince,omitempty"`
	SupportDesigns map[int]int `json:"supportDesigns,omitempty"`
	Leader         int         `json:"leader,omitempty"`
	LeaderID       int64       `json:"leaderId,omitempty"`
	RendezvousBy   int         `json:"rendezvousBy,omitempty"`
	CleanupEnemy   int         `json:"cleanupEnemy,omitempty"`
	CleanupTarget  int         `json:"cleanupTarget,omitempty"`
	CleanupUsed    bool        `json:"cleanupUsed,omitempty"`
	Enemy          int         `json:"enemy,omitempty"`
	Target         int         `json:"target,omitempty"`
	Grouped        bool        `json:"grouped,omitempty"`
	Cycle          int         `json:"cycle,omitempty"`
	Status         string      `json:"status"`
}

type cachedPatrolJob struct {
	raw string
	job patrolJob
}

func readPatrolJob(f *Fleet) patrolJob {
	raw := f.GetTag(patrolTag)
	if raw == "" {
		return patrolJob{}
	}
	if f.patrolJobCache == nil || f.patrolJobCache.raw != raw {
		var job patrolJob
		_ = json.Unmarshal([]byte(raw), &job)
		f.patrolJobCache = &cachedPatrolJob{raw: raw, job: job}
	}
	job := f.patrolJobCache.job
	job.SupportDesigns = maps.Clone(job.SupportDesigns)
	return job
}
func savePatrolJob(f *Fleet, job patrolJob) {
	if job.Leader == f.Num {
		job.LeaderID = f.ID
	}
	raw, _ := json.Marshal(job)
	f.SetTag(patrolTag, string(raw))
}

// Native total rating includes planetary bombs. Those cannot fire on ships.
func shipCombatPower(spec ShipDesignSpec) int {
	if spec.Bomber && !spec.HasWeapons {
		return 0
	}
	power := spec.PowerRating
	for _, bombs := range [][]Bomb{spec.Bombs, spec.SmartBombs, spec.RetroBombs} {
		for _, bomb := range bombs {
			power -= int(bomb.KillRate*10+bomb.StructureDestroyRate) * bomb.Quantity * 2
		}
	}
	return max(0, power)
}
func patrolPower(f *Fleet) int {
	return shipCombatPower(f.Spec.ShipDesignSpec) * (100 - fleetDamagePercent(f)) / 100
}
func patrolEnemy(player *Player, job patrolJob) *Fleet {
	for _, enemy := range player.FleetIntels {
		if enemy != nil && !enemy.Delete && !enemy.Starbase && enemy.ReportAge == 0 && !player.IsFriend(enemy.PlayerNum) && enemy.PlayerNum == job.Enemy && enemy.Num == job.Target {
			return enemy
		}
	}
	return nil
}

// Fleet reports contain token counts, but their private design pointers may
// refer to the real enemy designs. Consult the player's design reports only.
func patrolReportedPower(player *Player, enemy *Fleet) int {
	if len(enemy.Tokens) == 0 {
		if enemy.Spec.HasWeapons && enemy.Spec.PowerRating <= 0 {
			return max(1, reportedWeaponUnit(player)*4)
		}
		return max(1, shipCombatPower(enemy.Spec.ShipDesignSpec))
	}
	power := 0
	for _, token := range enemy.Tokens {
		design := player.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum)
		if design == nil {
			power += max(1, token.Quantity) * reportedWeaponUnit(player) * 4
			continue
		}
		if len(design.Slots) > 0 {
			power += max(1, shipCombatPower(design.Spec)) * token.Quantity
		} else {
			if design.Hull == Scout.Name {
				// A basic scout has one general-purpose slot, not the weapons
				// capacity of a warship of comparable mass. This remains an
				// estimate until its components are scanned.
				power += max(2, design.Spec.Mass/8) * token.Quantity
				continue
			}
			// Unknown equipment is never assumed unarmed. Use a conservative
			// mass estimate until a component scan gives an actual rating.
			slots := 0
			if hull := StaticTechStore.GetHull(design.Hull); hull != nil {
				for _, slot := range hull.Slots {
					if slot.Type&HullSlotTypeWeapon != 0 {
						slots += slot.Capacity
					}
				}
			}
			power += max(1, slots*reportedWeaponUnit(player)) * token.Quantity
		}
	}
	return max(1, power)
}

// Count public participants and courses converging during the interception.
func patrolThreat(player *Player, target *Fleet) int {
	power := 0
	for _, enemy := range patrolEncounterReports(player, target, patrolInterceptPosition(target), 1) {
		power = min(1_000_000_000, power+patrolReportedPower(player, enemy))
	}
	for _, planet := range player.PlanetIntels {
		if planet != nil && planet.Owned() && !player.IsFriend(planet.PlayerNum) && planet.PlayerNum != player.Num && planet.Spec.HasStarbase && (planet.Position.DistanceTo(target.Position) <= 3 || planet.Position.DistanceTo(patrolInterceptPosition(target)) <= 3) {
			return 1_000_000_000
		}
	}
	return max(1, power)
}

func patrolReportedCombatant(player *Player, enemy *Fleet) bool {
	if reportedScoutsOnly(player, enemy) {
		return false
	}
	if len(enemy.Tokens) == 0 {
		return enemy.Spec.HasWeapons || enemy.Spec.PowerRating > 0 && !enemy.Spec.Bomber
	}
	for _, token := range enemy.Tokens {
		d := player.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum)
		if d == nil {
			return true
		}
		if len(d.Slots) > 0 {
			if d.Spec.HasWeapons || d.Spec.PowerRating > 0 && !d.Spec.Bomber {
				return true
			}
			continue
		}
		h := StaticTechStore.GetHull(d.Hull)
		if h == nil {
			return true
		}
		for _, slot := range h.Slots {
			if slot.Type&HullSlotTypeWeapon != 0 {
				return true
			}
		}
	}
	return false
}
func patrolCanAttack(player *Player, fleet *Fleet) bool {
	if fleetHasDedicatedMinesweeper(player, fleet) {
		return false
	}
	job := readPatrolJob(fleet)
	enemy := patrolTacticalEnemy(player, fleet)
	if job.CleanupTarget != 0 && player.patrolEvaluator != nil && enemy != nil {
		return player.patrolEvaluator.assess(player, []*Fleet{fleet}, enemy, patrolInterceptPosition(enemy), 1).Ready
	}
	if player.patrolEvaluator != nil && job.Leader == fleet.Num && enemy != nil && !patrolNeedsRecon(player, enemy) && fleet.Spec.TotalShips >= job.RequiredShips {
		result := patrolFleetEvaluator(player, fleet).assess(player, []*Fleet{fleet}, enemy, patrolInterceptPosition(enemy), 1)
		if result.Ready {
			applyPatrolBattlePlan(player, fleet, result.Tactic)
		}
		return result.Ready
	}
	return job.Leader == fleet.Num && enemy != nil && !patrolNeedsRecon(player, enemy) && patrolPower(fleet) >= patrolThreat(player, enemy)*3/2+1 && fleet.Spec.TotalShips >= job.RequiredShips
}
func patrolAvailable(player *Player, f *Fleet) bool {
	return f.GetTag(containmentTag) == "" && patrolOperational(player, f)
}
func patrolOperational(player *Player, f *Fleet) bool {
	defencePlanning := player.patrolEvaluator != nil && player.patrolEvaluator.PlanningUrgentDefence
	if landingManaged(f) || f.GetTag(battleshipRallyTag) != "" && !defencePlanning {
		return false
	}
	if f.GetTag(recaptureAssaultTag) != "" && (!defencePlanning || !pureAssaultBattleships(player, f)) {
		return false
	}
	if fleetHasDedicatedMinesweeper(player, f) {
		return false
	}
	return f.GetTag(patrolSalvageTag) == "" && f.GetTag(anticipatedSalvageTag) == "" && f.GetTag("maintenance-pending") == "" && !fuelNetworkManaged(f) && !f.Delete && !f.Starbase && (player.Automation.FleetRoles[f.Num] == "patrol" || player.Automation.FleetRoles[f.Num] == "escort" && f.GetTag(escortTag) == "" && f.GetTag(bomberBorrowTag) == "") && f.Spec.HasWeapons && patrolPower(f) > 0 && !f.InDesignMaintenance() && !f.InAutomaticSafety() && !player.FleetInActiveAssembly(f) && f.GetTag(tankerAwaitTag) == "" && readTankerJob(f) == nil && fleetDamagePercent(f) < player.Automation.repairThreshold()
}
func (t *turnGenerator) releasePatrol(player *Player, leader *Fleet) {
	job := readPatrolJob(leader)
	var stacks []*Fleet
	for _, member := range slices.Clone(t.game.getFleets(player.Num)) {
		if readPatrolJob(member).Leader != leader.Num {
			continue
		}
		if job.Grouped && !member.Delete && !member.InDesignMaintenance() && member.Spec.TotalShips > 1 && patrolFuelAreaClear(player, member, member.Position) {
			stacks = append(stacks, member)
		}
		delete(member.Tags, patrolTag)
		restorePatrolBattlePlan(player, member)
		if member.GetTag(patrolSupportTag) == "escort" {
			player.Automation.FleetRoles[member.Num] = "escort"
			delete(member.Tags, patrolSupportTag)
		}
		if patrolOperational(player, member) {
			holdAutomaticFleet(player, member, "Wolf pack mission finished")
		}
	}
	for _, stack := range stacks {
		dispersing := stack
		if len(job.SupportDesigns) == 0 {
			if spare := t.retainPatrolSectorReserve(player, stack); spare != nil {
				dispersing = spare
			}
		}
		children, err := NewOrderer().SplitAll(&t.game.Rules, player, t.game.getFleets(player.Num), dispersing)
		if err == nil {
			for _, child := range children {
				t.game.Fleets = append(t.game.Fleets, child)
				_ = t.game.Universe.addFleet(child)
			}
			for _, child := range append([]*Fleet{dispersing}, children...) {
				delete(child.Tags, patrolTag)
				delete(child.Tags, patrolSupportTag)
				player.Automation.FleetRoles[child.Num] = "patrol"
				for _, token := range child.Tokens {
					if job.SupportDesigns[token.DesignNum] > 0 {
						player.Automation.FleetRoles[child.Num] = "escort"
						job.SupportDesigns[token.DesignNum] -= token.Quantity
					}
				}
			}
			t.balanceDispersedPatrolFuel(player, append([]*Fleet{dispersing}, children...))
		}
	}
}

func (t *turnGenerator) patrolRoute(player *Player, fleet *Fleet, position Vector, attack bool) bool {
	policy := automaticRoutePolicy{AllowThreat: true, Fast: attack, MaxTurns: 20, MinWarp: automaticCruiseWarp(player, fleet)}
	if !attack {
		policy.EconomicalPatrol = true
		policy.MaxWarp = economicalPatrolMaxWarp(player, fleet)
		policy.MinWarp = 1
	}
	route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, position, MapObjectTarget{}, policy)
	if route == nil {
		if t.returnPatrolToFuel(player, fleet) {
			return false
		}
		holdAutomaticFleet(player, fleet, "Patrol: no safe route with return fuel")
		saveAutomaticFuelRequest(fleet, automaticFuelRequest{Destination: position, Members: []int{fleet.Num}, Urgent: true})
		return false
	}
	applyExpeditionRoute(player, fleet, route)
	return true
}

func (t *turnGenerator) returnPatrolToFuel(player *Player, fleet *Fleet) bool {
	var best *expeditionRoute
	for _, dock := range expeditionStations(player, fleet) {
		if dock.Position == fleet.Position && fleet.Fuel >= fleet.Spec.FuelCapacity {
			continue
		}
		route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, dock.Position, dock.MapObject.ToTarget(), automaticRoutePolicy{AllowThreat: true, MaxTurns: 20, MinWarp: automaticCruiseWarp(player, fleet)})
		if route != nil && (best == nil || route.Turns < best.Turns) {
			best = route
		}
	}
	if best == nil {
		return t.routeEmergencyReturn(player, fleet)
	}
	applyExpeditionRoute(player, fleet, best)
	fleet.SetTag(fuelStatusTag, "Patrol returning to refuel; combat reserve protected")
	return true
}

// Use the game's actual fleet interception for the final one-turn leg. An
// extrapolated heading alone misses scouts which stop at a planet this turn.
func (t *turnGenerator) preparePatrolPursuits() {
	for _, player := range t.game.Players {
		for _, fleet := range t.game.getFleets(player.Num) {
			if !patrolAvailable(player, fleet) || readPatrolJob(fleet).reliefActive(t.game.Year) || !patrolCanAttack(player, fleet) || len(fleet.Waypoints) != 2 {
				continue
			}
			enemy := patrolTacticalEnemy(player, fleet)
			if fleet.Position.DistanceTo(enemy.Position) > 150 {
				continue
			}
			wp := &fleet.Waypoints[1]
			if wp.WarpSpeed == StargateWarpSpeed {
				continue
			}
			for speed := automaticMaxWarp(player, fleet); speed >= automaticCruiseWarp(player, fleet); speed-- {
				if fleet.Position.DistanceTo(wp.Position) > fleet.StrategicSpeed(speed) {
					continue
				}
				reserve := automaticReturnFuel(&t.game.Rules, player, fleet, wp.Position, expeditionStations(player, fleet), true)
				_, _, _, enough := automaticLegFuel(player, fleet, speed, fleet.Position.DistanceTo(wp.Position), fleet.Fuel, reserve)
				if !enough || !automaticLegSafe(&t.game.Rules, player, fleet.Position, wp.Position, speed, true, fleet.StrategicSpeed(speed)) {
					continue
				}
				wp.MapObjectTarget = enemy.MapObject.ToTarget()
				wp.WarpSpeed = speed
				break
			}
		}
	}
}

func (t *turnGenerator) patrolPursuitSafe(fleet *Fleet, position Vector, remaining float64) bool {
	player := t.game.getPlayer(fleet.PlayerNum)
	if player.Automation.FleetRoles[fleet.Num] == "patrol" && player.patrolEvaluator != nil {
		if enemy := patrolTacticalEnemy(player, fleet); enemy != nil {
			result := patrolFleetEvaluator(player, fleet).assess(player, []*Fleet{fleet}, enemy, position, 1)
			if !result.Ready {
				return false
			}
			applyPatrolBattlePlan(player, fleet, result.Tactic)
		}
	}
	if player.Automation.FleetRoles[fleet.Num] != "patrol" && !scoutReconActive(player, fleet) {
		return true
	}
	wp := fleet.Waypoints[1]
	distance := fleet.Position.DistanceTo(position)
	if scoutReconActive(player, fleet) && distance > remaining {
		position = fleet.Position.ToFloat64().Add(position.Subtract(fleet.Position).Normalized().Scale(max(0, remaining))).ToInt(true)
		distance = fleet.Position.DistanceTo(position)
	}
	if distance > remaining+1.5 || !automaticLegSafe(&t.game.Rules, player, fleet.Position, position, wp.WarpSpeed, true, fleet.StrategicSpeed(wp.WarpSpeed)) {
		return false
	}
	reserve := automaticReturnFuel(&t.game.Rules, player, fleet, position, expeditionStations(player, fleet), true)
	if job := readScoutRecon(fleet); scoutReconActive(player, fleet) && job != nil && !job.Watching {
		reserve = 0
	}
	_, _, _, enough := automaticLegFuel(player, fleet, wp.WarpSpeed, distance, fleet.Fuel, reserve)
	return enough
}
func (t *turnGenerator) advancePatrol(player *Player, leader *Fleet) {
	if leader.InDesignMaintenance() {
		if !readPatrolJob(leader).Grouped {
			delete(leader.Tags, patrolTag)
		}
		return
	}
	if leader.InAutomaticSafety() {
		return
	} // Keep the combat stack together throughout retreat/repair.
	job := readPatrolJob(leader)
	enemy := patrolEnemy(player, job)
	if enemy == nil || !patrolAvailable(player, leader) {
		t.releasePatrol(player, leader)
		return
	}
	if declinedPatrolTarget(leader, enemy, t.game.Year) {
		t.releasePatrol(player, leader)
		return
	}
	if player.patrolEvaluator != nil {
		t.advanceAssessedPatrol(player, leader, enemy, &job)
		return
	}
	needsRecon := patrolNeedsRecon(player, enemy)
	if !needsRecon {
		job.Provisional = false
	}
	if patrolShouldAdvanceInstead(player, leader, enemy) {
		t.declinePatrolTarget(player, leader)
		t.advancePatrolFront(player, leader)
		return
	}
	// A pack can recruit fresh nearby support when the reported enemy grows.
	current := patrolPower(leader)
	currentShips := leader.Spec.TotalShips
	for _, m := range t.game.getFleets(player.Num) {
		if m != leader && !m.Delete && readPatrolJob(m).Leader == leader.Num {
			current += patrolPower(m)
			currentShips += m.Spec.TotalShips
		}
	}
	needed := patrolThreat(player, enemy)*3/2 + 1
	for _, m := range t.game.getFleets(player.Num) {
		if current >= max(needed, job.RecruitPower) && currentShips >= job.RequiredShips {
			break
		}
		if m == leader || !patrolAvailable(player, m) || readPatrolJob(m).Leader != 0 || m.Position.DistanceTo(leader.Position) > 100 || m.Fuel < m.Spec.FuelCapacity/4 {
			continue
		}
		if player.Automation.FleetRoles[m.Num] == "escort" {
			m.SetTag(patrolSupportTag, "escort")
			if job.SupportDesigns == nil {
				job.SupportDesigns = map[int]int{}
			}
			for _, token := range m.Tokens {
				job.SupportDesigns[token.DesignNum] += token.Quantity
			}
			player.Automation.FleetRoles[m.Num] = "patrol"
		}
		savePatrolJob(m, patrolJob{Leader: leader.Num, Enemy: job.Enemy, Target: job.Target, Status: "Joining nearby combat group"})
		current += patrolPower(m)
		currentShips += m.Spec.TotalShips
	}
	if needsRecon {
		t.provisionalPatrolGather(player, leader, enemy, &job)
		return
	}
	allHere := true
	for _, member := range slices.Clone(t.game.getFleets(player.Num)) {
		if member == leader || member.Delete || readPatrolJob(member).Leader != leader.Num {
			continue
		}
		if !patrolAvailable(player, member) {
			t.releasePatrol(player, leader)
			return
		}
		if member.Position != leader.Position {
			if !t.patrolRoute(player, member, leader.Position, false) {
				t.releasePatrol(player, leader)
				t.returnPatrolToFuel(player, member)
				return
			}
			allHere = false
			continue
		}
		if _, err := NewOrderer().Merge(&t.game.Rules, player, []*Fleet{leader, member}); err == nil {
			t.game.deleteFleet(member)
			job.Grouped = true
		}
	}
	if job.GatheringSince == 0 {
		job.GatheringSince = t.game.Year - 2
	}
	if !allHere && patrolPower(leader) >= needed && leader.Spec.TotalShips >= job.RequiredShips {
		allHere = true
	}
	if (!allHere || patrolPower(leader) < needed) && t.game.Year-job.GatheringSince >= 2 && leader.Position.DistanceTo(enemy.Position) > leader.StrategicSpeed(automaticMaxWarp(player, leader)) {
		t.declinePatrolTarget(player, leader)
		return
	}
	if !allHere && t.game.Year-job.GatheringSince >= 2 && patrolPower(leader) >= needed {
		job.RequiredShips = leader.Spec.TotalShips
		allHere = true
	}
	if !allHere {
		if t.movingPatrolRally(player, leader, &job) {
			return
		}
		job.Status = "Gathering patrol ships before interception"
		holdAutomaticFleet(player, leader, "")
		savePatrolJob(leader, job)
		return
	}
	if colony := patrolDefendedColony(player, enemy); colony != nil && !priorityFleet(player, enemy) && leader.Position.DistanceTo(enemy.Position) > 150 {
		job.Status = "Mobilizing one defensive fleet at " + colony.Name + " against incoming " + enemy.Name
		savePatrolJob(leader, job)
		if leader.Position != colony.Position {
			t.patrolRoute(player, leader, colony.Position, false)
		} else {
			holdAutomaticFleet(player, leader, job.Status)
		}
		return
	}
	if patrolPower(leader) < patrolThreat(player, enemy)*3/2+1 {
		job.Status = "Regrouped as one fleet; withdrawing toward fuel while the enemy is stronger"
		savePatrolJob(leader, job)
		t.returnPatrolToFuel(player, leader)
		return
	}
	t.releaseExcessPatrolShips(player, leader, enemy, &job)
	if player.patrolEvaluator != nil {
		result := player.patrolEvaluator.assess(player, []*Fleet{leader}, enemy, patrolInterceptPosition(enemy), 1)
		if !result.Ready {
			job.RecruitPower = max(job.RecruitPower, patrolPower(leader)+max(1, patrolPower(leader)/max(1, leader.Spec.TotalShips)))
			job.Status = result.Reason
			t.provisionalPatrolGather(player, leader, enemy, &job)
			return
		}
		applyPatrolBattlePlan(player, leader, result.Tactic)
	}
	// Recompute a bounded interception from fresh reports each turn. A fixed waypoint
	// prevents native pursuit from silently extending the fuel budget mid-movement.
	position := patrolInterceptPosition(enemy)
	job.Status = "Intercepting " + enemy.Name
	savePatrolJob(leader, job)
	if !t.patrolRoute(player, leader, position, true) {
		t.releasePatrol(player, leader)
		t.returnPatrolToFuel(player, leader)
	}
}
func (t *turnGenerator) planPatrolAutomation() {
	for _, player := range t.game.Players {
		t.reconcilePatrolJobs(player)
		t.planUrgentPatrolDefence(player)
		t.planMinelayerInterceptions(player)
		fleets := []*Fleet{}
		for _, fleet := range t.game.getFleets(player.Num) {
			delete(fleet.Tags, patrolMineSweepTag)
			if player.Automation.FleetRoles[fleet.Num] == "patrol" || patrolAvailable(player, fleet) || fleet.GetTag(patrolTag) != "" {
				fleets = append(fleets, fleet)
			}
		}
		if len(fleets) == 0 {
			continue
		}
		nearestDistances := make(map[*Fleet]float64, len(fleets))
		nearest := func(f *Fleet) float64 {
			if distance, ok := nearestDistances[f]; ok {
				return distance
			}
			distance := math.Inf(1)
			for _, enemy := range player.FleetIntels {
				if enemy != nil && !enemy.Delete && !enemy.Starbase && enemy.ReportAge == 0 && enemy.PlayerNum != player.Num && !player.IsFriend(enemy.PlayerNum) {
					distance = min(distance, f.Position.DistanceTo(enemy.Position))
				}
			}
			nearestDistances[f] = distance
			return distance
		}
		sort.Slice(fleets, func(i, j int) bool {
			a, b := nearest(fleets[i]), nearest(fleets[j])
			if a == b {
				return fleets[i].Num < fleets[j].Num
			}
			return a < b
		})
		for _, fleet := range fleets {
			if landingManaged(fleet) || fleet.GetTag(battleshipRallyTag) != "" || fleet.GetTag(battleshipNextAssaultTag) != "" {
				continue
			}
			job := readPatrolJob(fleet)
			if job.Minelayer || defencePatrolCommitted(player, fleet) {
				continue
			}
			if fleet.GetTag(recaptureAssaultTag) != "" {
				continue
			}
			if fleet.GetTag("maintenance-pending") != "" || fuelNetworkManaged(fleet) || readTankerJob(fleet) != nil {
				continue
			}
			if !fleet.Delete && player.Automation.FleetRoles[fleet.Num] == "patrol" && fleet.GetTag(tankerAwaitTag) == "" && !fleet.InAutomaticSafety() && !fleet.InDesignMaintenance() {
				reserve := automaticReturnFuel(&t.game.Rules, player, fleet, fleet.Position, expeditionStations(player, fleet), false)
				if reserve > fleet.Fuel || fleet.Fuel < fleet.Spec.FuelCapacity/4 && !nearbyPatrolContact(player, fleet, 100) && !usefulFreePatrolMovement(player, fleet) {
					if job.Leader == fleet.Num {
						t.releasePatrol(player, fleet)
					}
					if !t.returnPatrolToFuel(player, fleet) {
						holdAutomaticFleet(player, fleet, "Patrol needs fuel assistance for a timely return")
						saveAutomaticFuelRequest(fleet, automaticFuelRequest{Destination: fleet.Position, Members: []int{fleet.Num}, Urgent: true})
					}
					continue
				}
			}
			if job.Leader == fleet.Num && !fleet.Delete {
				t.retargetPatrolOpportunity(player, fleet)
				job = readPatrolJob(fleet)
				if !fleet.Delete && job.Leader == fleet.Num {
					t.advancePatrol(player, fleet)
				}
			}
			if job.Leader != 0 && job.Leader != fleet.Num {
				leader := t.game.getFleet(player.Num, job.Leader)
				if leader == nil || leader.Delete || player.Automation.FleetRoles[leader.Num] != "patrol" {
					delete(fleet.Tags, patrolTag)
					if fleet.GetTag(patrolSupportTag) == "escort" {
						player.Automation.FleetRoles[fleet.Num] = "escort"
						delete(fleet.Tags, patrolSupportTag)
					}
				}
			}
		}
		// Releasing a pack creates new single-ship fleets. Give them work in
		// this same planning pass, rather than leaving them idle for a year.
		fleets = slices.Clone(t.game.getFleets(player.Num))
		clear(nearestDistances)
		sort.Slice(fleets, func(i, j int) bool {
			a, b := nearest(fleets[i]), nearest(fleets[j])
			if a == b {
				return fleets[i].Num < fleets[j].Num
			}
			return a < b
		})
		for _, fleet := range fleets {
			if !patrolAvailable(player, fleet) || readPatrolJob(fleet).Leader != 0 || fleet.Fuel < fleet.Spec.FuelCapacity/4 && !nearbyPatrolContact(player, fleet, 100) && !usefulFreePatrolMovement(player, fleet) || readAutomaticFuelRequest(fleet) != nil && readAutomaticFuelRequest(fleet).Urgent {
				continue
			}
			if player.patrolEvaluator != nil && fleet.GetTag(patrolReturnHuntTag) != "" && t.patrolReturnHunt(player, fleet) {
				continue
			}
			enemies := sortedPatrolTargets(player, fleet)
			assigned := false
			for _, enemy := range enemies {
				if player.patrolEvaluator != nil && enemy != nil && player.patrolEvaluator.Deferred[priorityFleetKey(enemy.PlayerNum, enemy.Num)] {
					continue
				}
				if enemy == nil || enemy.Delete || enemy.Starbase || enemy.ReportAge != 0 || enemy.PlayerNum == player.Num || player.IsFriend(enemy.PlayerNum) || (!priorityFleet(player, enemy) && slowPreySupport(player, enemy) == nil && !(player.patrolEvaluator != nil && slowPatrolPrey(player, enemy) && fleet.Position.DistanceTo(enemy.Position) <= 300) && fleet.Position.DistanceTo(enemy.Position) > 150 && !patrolDefenceReachable(player, fleet, enemy) && !patrolGateReachable(player, fleet, enemy)) {
					continue
				}
				if patrolShouldAdvanceInstead(player, fleet, enemy) {
					continue
				}
				claimed := false
				for _, other := range fleets {
					j := readPatrolJob(other)
					if !other.Delete && j.Leader != 0 && j.Target == enemy.Num && j.Enemy == enemy.PlayerNum {
						if j.Leader == other.Num && patrolDesiredShips(player, enemy) > patrolGroupShips(fleets, other) && other.Position.DistanceTo(fleet.Position) <= 100 {
							t.joinPatrolOpportunity(player, fleet, other, enemy)
							assigned = true
						}
						claimed = true
						break
					}
				}
				if assigned {
					break
				}
				if !priorityFleet(player, enemy) && declinedPatrolTarget(fleet, enemy, t.game.Year) || claimed {
					continue
				}
				delete(fleet.Tags, frontierPatrolTag)
				members := []*Fleet{fleet}
				power := patrolPower(fleet)
				ships := fleet.Spec.TotalShips
				wanted := patrolDesiredShips(player, enemy)
				needed := patrolThreat(player, enemy)*3/2 + 1
				for _, member := range fleets {
					if power >= needed && ships >= wanted {
						break
					}
					if member != fleet && patrolAvailable(player, member) && readPatrolJob(member).Leader == 0 && member.Position.DistanceTo(fleet.Position) <= 100 {
						members = append(members, member)
						power += patrolPower(member)
						ships += member.Spec.TotalShips
					}
				}
				borrowed := []*Fleet{}
				support := map[int]int{}
				if power < needed || ships < wanted {
					for _, e := range t.game.getFleets(player.Num) {
						if power >= needed && ships >= wanted {
							break
						}
						if slices.Contains(members, e) || e.Delete || e.InDesignMaintenance() || e.InAutomaticSafety() || fuelNetworkManaged(e) || readTankerJob(e) != nil || player.FleetInActiveAssembly(e) || player.Automation.FleetRoles[e.Num] != "escort" || e.GetTag(escortTag) != "" || !e.Spec.HasWeapons || e.Position.DistanceTo(fleet.Position) > 100 || e.Fuel < e.Spec.FuelCapacity/4 {
							continue
						}
						members = append(members, e)
						borrowed = append(borrowed, e)
						power += patrolPower(e)
						ships += e.Spec.TotalShips

					}
				}
				if power < needed && len(members) == 1 && !patrolDefenceReachable(player, fleet, enemy) && !patrolNeedsRecon(player, enemy) {
					continue
				}
				for _, m := range members {
					if player.Automation.FleetRoles[m.Num] == "escort" && m.GetTag(patrolSupportTag) == "" {
						m.SetTag(patrolSupportTag, "escort")
						for _, token := range m.Tokens {
							support[token.DesignNum] += token.Quantity
						}
						player.Automation.FleetRoles[m.Num] = "patrol"
					}
				}
				for _, e := range borrowed {
					e.SetTag(patrolSupportTag, "escort")
					player.Automation.FleetRoles[e.Num] = "patrol"
				}
				for _, member := range members {
					job := patrolJob{Grouped: readPatrolJob(member).Grouped, RequiredShips: min(wanted, ships), GatheringSince: t.game.Year, Leader: fleet.Num, Enemy: enemy.PlayerNum, Target: enemy.Num, Status: "Wolf pack gathering before coordinated interception"}
					if member == fleet {
						job.SupportDesigns = support
					}
					savePatrolJob(member, job)
				}
				t.advancePatrol(player, fleet)
				if readPatrolJob(fleet).Leader == 0 {
					continue
				}
				assigned = true
				break
			}
			if assigned {
				continue
			}
			if localPatrolReserve(fleet, t.game.Year) {
				holdAutomaticFleet(player, fleet, "Holding a local patrol reserve after orbital relief")
				continue
			}
			if player.Automation.FleetRoles[fleet.Num] == "escort" {
				continue
			}
			job := readPatrolJob(fleet)
			if t.patrolFrontier(player, fleet) {
				job.Status = fleet.GetTag(frontierPatrolTag)
				savePatrolJob(fleet, job)
				continue
			}
			// A lack of a funded objective is not a reason to manufacture home
			// loops. Fuel recovery and new contact reports can enable the next leg.
			delete(fleet.Tags, frontierPatrolTag)
			job.Status = "Offensive advance awaits a funded objective or fresh reconnaissance"
			savePatrolJob(fleet, job)
		}
	}
}

func patrolReportedBomber(p *Player, enemy *Fleet) bool {
	if enemy == nil {
		return false
	}
	if enemy.Spec.Bomber {
		return true
	}
	for _, token := range enemy.Tokens {
		design := p.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum)
		if design != nil {
			hull := StaticTechStore.GetHull(design.Hull)
			if hull != nil && hull.Class == ShipClassBomber {
				return true
			}
		}
	}
	return false
}

// Estimate unscanned weapons from visible hull capacity and the player's known
// weapon scale, never from hidden design pointers or an "infinite" threat.
func reportedWeaponUnit(p *Player) int {
	unit := 4
	for _, d := range p.Designs {
		weapons := 0
		for _, slot := range d.Slots {
			c := StaticTechStore.GetHullComponent(slot.HullComponent)
			if c != nil && c.HullSlotType&HullSlotTypeWeapon != 0 {
				weapons += slot.Quantity
			}
		}
		if d.Spec.HasWeapons && weapons > 0 {
			unit = max(unit, (d.Spec.PowerRating+weapons-1)/weapons)
		}
	}
	return unit
}
func reportedScoutsOnly(p *Player, enemy *Fleet) bool {
	if enemy.Spec.HasWeapons {
		return false
	}
	if len(enemy.Tokens) == 0 {
		return enemy.Spec.ShipClass == ShipClassScout && !enemy.Spec.HasWeapons && enemy.Spec.PowerRating == 0
	}
	for _, token := range enemy.Tokens {
		d := p.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum)
		if d == nil || d.Spec.HasWeapons {
			return false
		}
		// The native broad Scout class also includes Frigate/Destroyer.
		// Identify actual exploration hulls by Type, never that broad class.
		h := StaticTechStore.GetHull(d.Hull)
		if h != nil {
			if h.Type != TechHullTypeScout {
				return false
			}
		} else if d.Hull != Scout.Name {
			return false
		}
	}
	return true
}

// Merge co-located members even at an intermediate rendezvous. They must enter
// battle as one token stack rather than several independent single-ship fleets.
func (t *turnGenerator) mergePatrolRendezvous() {
	for _, p := range t.automaticPlayers() {
		if p.patrolEvaluator != nil {
			t.mergeArrivedPatrolRelief(p)
			// Only the launch planner may merge a combat-ready local formation.
			continue
		}
		fleets := slices.Clone(t.game.getFleets(p.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		for _, a := range fleets {
			j := readPatrolJob(a)
			if !patrolAvailable(p, a) || j.Leader == 0 {
				continue
			}
			for _, b := range fleets {
				k := readPatrolJob(b)
				if b == a || b.Delete || !patrolAvailable(p, b) || a.Position != b.Position || k.Leader != j.Leader || k.Target != j.Target || k.Enemy != j.Enemy {
					continue
				}
				leader, member := a, b
				if b.Num == j.Leader {
					leader, member = b, a
				}
				if _, err := NewOrderer().Merge(&t.game.Rules, p, []*Fleet{leader, member}); err != nil {
					continue
				}
				t.game.deleteFleet(member)
				saved := readPatrolJob(leader)
				saved.Grouped = true
				savePatrolJob(leader, saved)
				if overall := t.game.getFleet(p.Num, j.Leader); overall != nil && !overall.Delete {
					w := readPatrolJob(overall)
					w.Grouped = true
					savePatrolJob(overall, w)
				}
				if member == a {
					break
				}
			}
		}
	}
}

// Respond to a reported course toward our colonies before the attacker enters
// the local 150-ly interception radius. No hidden enemy orders are consulted.
func patrolDefendedColony(p *Player, enemy *Fleet) *Planet {
	return patrolDefendedColonyAmong(p, enemy, p.PlanetIntels)
}

func patrolDefendedColonyAmong(p *Player, enemy *Fleet, colonies []*Planet) *Planet {
	if enemy == nil || enemy.Delete || enemy.PlayerNum == p.Num || p.IsFriend(enemy.PlayerNum) || enemy.ReportAge < 0 || enemy.ReportAge > 2 || enemy.Heading.LengthSquared() == 0 {
		return nil
	}
	heading := enemy.Heading.Normalized()
	speed := max(1, enemy.observedStrategicSpeed())
	var best *Planet
	nearest := math.Inf(1)
	for _, colony := range colonies {
		if colony == nil || !colony.OwnedBy(p.Num) {
			continue
		}
		delta := colony.Position.Subtract(enemy.Position).ToFloat64()
		along := delta.X*heading.X + delta.Y*heading.Y
		cross := math.Abs(delta.X*heading.Y - delta.Y*heading.X)
		if along >= 0 && along <= speed*10 && cross <= 20 && along < nearest {
			best = colony
			nearest = along
		}
	}
	return best
}
func patrolDefenceReachable(p *Player, f, enemy *Fleet) bool {
	colony := patrolDefendedColony(p, enemy)
	return colony != nil && f.Position.DistanceTo(colony.Position) <= 150
}
