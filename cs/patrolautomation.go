package cs

import (
	"encoding/json"
	"math"
	"slices"
	"sort"
)

const patrolTag = "automatic-patrol"
const patrolSupportTag = "patrol-support-role"

type patrolJob struct {
	SupportDesigns map[int]int `json:"supportDesigns,omitempty"`
	Leader         int         `json:"leader,omitempty"`
	Enemy          int         `json:"enemy,omitempty"`
	Target         int         `json:"target,omitempty"`
	Grouped        bool        `json:"grouped,omitempty"`
	Cycle          int         `json:"cycle,omitempty"`
	Status         string      `json:"status"`
}

func readPatrolJob(f *Fleet) patrolJob {
	var job patrolJob
	_ = json.Unmarshal([]byte(f.GetTag(patrolTag)), &job)
	return job
}
func savePatrolJob(f *Fleet, job patrolJob) {
	raw, _ := json.Marshal(job)
	f.SetTag(patrolTag, string(raw))
}
func patrolPower(f *Fleet) int {
	return max(0, f.Spec.PowerRating) * (100 - fleetDamagePercent(f)) / 100
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
			return 1_000_000_000
		}
		return max(1, enemy.Spec.PowerRating)
	}
	power := 0
	for _, token := range enemy.Tokens {
		design := player.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum)
		if design == nil {
			return 1_000_000_000
		}
		if len(design.Slots) > 0 {
			power += max(1, design.Spec.PowerRating) * token.Quantity
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
			power += max(50, design.Spec.Mass*4) * token.Quantity
		}
	}
	return max(1, power)
}

// Include nearby visible reinforcements, rather than treating each enemy as isolated.
func patrolThreat(player *Player, target *Fleet) int {
	power := 0
	for _, enemy := range player.FleetIntels {
		if enemy == nil || enemy.Delete || enemy.ReportAge != 0 || player.IsFriend(enemy.PlayerNum) || enemy.Position.DistanceTo(target.Position) > 30 {
			continue
		}
		if enemy != target && !patrolReportedCombatant(player, enemy) {
			continue
		}
		power = min(1_000_000_000, power+patrolReportedPower(player, enemy))
	}
	for _, planet := range player.PlanetIntels {
		if planet != nil && planet.Owned() && !player.IsFriend(planet.PlayerNum) && planet.PlayerNum != player.Num && planet.Spec.HasStarbase && planet.Position.DistanceTo(target.Position) <= 30 {
			return 1_000_000_000
		}
	}
	return max(1, power)
}

func patrolReportedCombatant(player *Player, enemy *Fleet) bool {
	if enemy.Spec.HasWeapons || enemy.Spec.PowerRating > 0 {
		return true
	}
	for _, token := range enemy.Tokens {
		design := player.GetShipDesignIntel(enemy.PlayerNum, token.DesignNum)
		if design == nil || design.Spec.PowerRating > 0 {
			return true
		}
		if len(design.Slots) == 0 && design.Hull != Scout.Name && design.Spec.ShipClass != ShipClassFreighter && design.Spec.ShipClass != ShipClassMiner {
			return true
		}
	}
	return false
}
func patrolCanAttack(player *Player, fleet *Fleet) bool {
	job := readPatrolJob(fleet)
	enemy := patrolEnemy(player, job)
	return job.Leader == fleet.Num && enemy != nil && patrolPower(fleet) >= patrolThreat(player, enemy)*3/2+1
}
func patrolAvailable(player *Player, f *Fleet) bool {
	return !fuelNetworkManaged(f) && !f.Delete && !f.Starbase && player.Automation.FleetRoles[f.Num] == "patrol" && f.Spec.HasWeapons && patrolPower(f) > 0 && !f.InDesignMaintenance() && !f.InAutomaticSafety() && !player.FleetInActiveAssembly(f) && f.GetTag(tankerAwaitTag) == "" && readTankerJob(f) == nil && fleetDamagePercent(f) < player.Automation.repairThreshold()
}
func (t *turnGenerator) releasePatrol(player *Player, leader *Fleet) {
	job := readPatrolJob(leader)
	for _, member := range t.game.getFleets(player.Num) {
		if readPatrolJob(member).Leader != leader.Num {
			continue
		}
		delete(member.Tags, patrolTag)
		if member.GetTag(patrolSupportTag) == "escort" {
			player.Automation.FleetRoles[member.Num] = "escort"
			delete(member.Tags, patrolSupportTag)
		}
		if !member.InDesignMaintenance() && !member.InAutomaticSafety() {
			holdAutomaticFleet(player, member, "Wolf pack mission finished")
		}
	}
	if job.Grouped && !leader.Delete && !leader.InDesignMaintenance() && leader.Spec.TotalShips > 1 {
		children, err := NewOrderer().SplitAll(&t.game.Rules, player, t.game.getFleets(player.Num), leader)
		if err == nil {
			for _, child := range children {
				t.game.Fleets = append(t.game.Fleets, child)
				_ = t.game.Universe.addFleet(child)
			}
			for _, child := range append([]*Fleet{leader}, children...) {
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
		}
	}
}

func (t *turnGenerator) patrolRoute(player *Player, fleet *Fleet, position Vector, attack bool) bool {
	route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, position, MapObjectTarget{}, automaticRoutePolicy{AllowThreat: attack, MaxTurns: 8, MinWarp: automaticCruiseWarp(player, fleet)})
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
		route := planAutomaticRoute(&t.game.Rules, player, []*Fleet{fleet}, dock.Position, dock.MapObject.ToTarget(), automaticRoutePolicy{MaxTurns: 8, MinWarp: automaticCruiseWarp(player, fleet)})
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
			if !patrolAvailable(player, fleet) || !patrolCanAttack(player, fleet) || len(fleet.Waypoints) != 2 {
				continue
			}
			enemy := patrolEnemy(player, readPatrolJob(fleet))
			wp := &fleet.Waypoints[1]
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
	_, _, _, enough := automaticLegFuel(player, fleet, wp.WarpSpeed, distance, fleet.Fuel, reserve)
	return enough
}
func (t *turnGenerator) advancePatrol(player *Player, leader *Fleet) {
	job := readPatrolJob(leader)
	enemy := patrolEnemy(player, job)
	if enemy == nil || !patrolAvailable(player, leader) {
		t.releasePatrol(player, leader)
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
	if !allHere {
		job.Status = "Gathering patrol ships before interception"
		holdAutomaticFleet(player, leader, "")
		savePatrolJob(leader, job)
		return
	}
	if patrolPower(leader) < patrolThreat(player, enemy)*3/2+1 {
		t.releasePatrol(player, leader)
		return
	}
	// Recompute a bounded interception from fresh reports each turn. A fixed waypoint
	// prevents native pursuit from silently extending the fuel budget mid-movement.
	position := enemy.Position
	if enemy.WarpSpeed > 0 && enemy.WarpSpeed <= 10 {
		delta := enemy.Heading.Scale(enemy.observedStrategicSpeed())
		position.X += int(math.Round(delta.X))
		position.Y += int(math.Round(delta.Y))
	}
	job.Status = "Intercepting " + enemy.Name
	savePatrolJob(leader, job)
	if !t.patrolRoute(player, leader, position, true) {
		t.releasePatrol(player, leader)
		t.returnPatrolToFuel(player, leader)
	}
}
func (t *turnGenerator) planPatrolAutomation() {
	for _, player := range t.game.Players {
		fleets := []*Fleet{}
		for _, fleet := range t.game.getFleets(player.Num) {
			if player.Automation.FleetRoles[fleet.Num] == "patrol" || fleet.GetTag(patrolTag) != "" {
				fleets = append(fleets, fleet)
			}
		}
		if len(fleets) == 0 {
			continue
		}
		nearest := func(f *Fleet) float64 {
			distance := math.Inf(1)
			for _, enemy := range player.FleetIntels {
				if enemy != nil && !enemy.Delete && !enemy.Starbase && enemy.ReportAge == 0 && enemy.PlayerNum != player.Num && !player.IsFriend(enemy.PlayerNum) {
					distance = min(distance, f.Position.DistanceTo(enemy.Position))
				}
			}
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
			job := readPatrolJob(fleet)
			if fleet.GetTag("maintenance-pending") != "" || fuelNetworkManaged(fleet) || readTankerJob(fleet) != nil {
				continue
			}
			if !fleet.Delete && player.Automation.FleetRoles[fleet.Num] == "patrol" && fleet.GetTag(tankerAwaitTag) == "" && !fleet.InAutomaticSafety() && !fleet.InDesignMaintenance() {
				reserve := automaticReturnFuel(&t.game.Rules, player, fleet, fleet.Position, expeditionStations(player, fleet), false)
				if fleet.Fuel < fleet.Spec.FuelCapacity/4 || reserve > fleet.Fuel {
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
				t.advancePatrol(player, fleet)
			}
			if job.Leader != 0 && job.Leader != fleet.Num {
				leader := t.game.getFleet(player.Num, job.Leader)
				if leader == nil || leader.Delete || player.Automation.FleetRoles[leader.Num] != "patrol" {
					delete(fleet.Tags, patrolTag)
				}
			}
		}
		for _, fleet := range fleets {
			if !patrolAvailable(player, fleet) || readPatrolJob(fleet).Leader != 0 || fleet.Fuel < fleet.Spec.FuelCapacity/4 || readAutomaticFuelRequest(fleet) != nil && readAutomaticFuelRequest(fleet).Urgent {
				continue
			}
			enemies := slices.Clone(player.FleetIntels)
			sort.SliceStable(enemies, func(i, j int) bool {
				if enemies[i] == nil {
					return false
				}
				if enemies[j] == nil {
					return true
				}
				ib, jb := patrolReportedBomber(player, enemies[i]), patrolReportedBomber(player, enemies[j])
				if ib != jb {
					return ib
				}
				ii, ji := enemyApproachingColony(player, enemies[i]), enemyApproachingColony(player, enemies[j])
				if ii != ji {
					return ii
				}
				return fleet.Position.DistanceTo(enemies[i].Position) < fleet.Position.DistanceTo(enemies[j].Position)
			})
			assigned := false
			for _, enemy := range enemies {
				if enemy == nil || enemy.Delete || enemy.Starbase || enemy.ReportAge != 0 || player.IsFriend(enemy.PlayerNum) || fleet.Position.DistanceTo(enemy.Position) > 150 {
					continue
				}
				claimed := false
				for _, other := range fleets {
					j := readPatrolJob(other)
					if !other.Delete && j.Leader != 0 && j.Target == enemy.Num && j.Enemy == enemy.PlayerNum {
						claimed = true
						break
					}
				}
				if declinedPatrolTarget(fleet, enemy, t.game.Year) || claimed {
					continue
				}
				delete(fleet.Tags, frontierPatrolTag)
				members := []*Fleet{fleet}
				power := patrolPower(fleet)
				needed := patrolThreat(player, enemy)*3/2 + 1
				for _, member := range fleets {
					if power >= needed {
						break
					}
					if member != fleet && patrolAvailable(player, member) && readPatrolJob(member).Leader == 0 && member.Position.DistanceTo(fleet.Position) <= 100 {
						members = append(members, member)
						power += patrolPower(member)
					}
				}
				borrowed := []*Fleet{}
				support := map[int]int{}
				if power < needed && (patrolReportedBomber(player, enemy) || enemyApproachingColony(player, enemy)) {
					for _, e := range t.game.getFleets(player.Num) {
						if power >= needed {
							break
						}
						if e.Delete || e.InDesignMaintenance() || e.InAutomaticSafety() || fuelNetworkManaged(e) || readTankerJob(e) != nil || player.FleetInActiveAssembly(e) || player.Automation.FleetRoles[e.Num] != "escort" || e.GetTag(escortTag) != "" || !e.Idle() || !e.Spec.HasWeapons || e.Position.DistanceTo(fleet.Position) > 100 || e.Fuel < e.Spec.FuelCapacity/4 {
							continue
						}
						members = append(members, e)
						borrowed = append(borrowed, e)
						power += patrolPower(e)
						for _, token := range e.Tokens {
							support[token.DesignNum] += token.Quantity
						}
					}
				}
				if power < needed {
					continue
				}
				for _, e := range borrowed {
					e.SetTag(patrolSupportTag, "escort")
					player.Automation.FleetRoles[e.Num] = "patrol"
				}
				for _, member := range members {
					job := patrolJob{Leader: fleet.Num, Enemy: enemy.PlayerNum, Target: enemy.Num, Status: "Wolf pack gathering before coordinated interception"}
					if member == fleet {
						job.SupportDesigns = support
					}
					savePatrolJob(member, job)
				}
				t.advancePatrol(player, fleet)
				assigned = true
				break
			}
			if assigned {
				continue
			}
			job := readPatrolJob(fleet)
			if t.patrolFrontier(player, fleet) {
				job.Status = "Patrolling frontier sectors with fuel reserved for return"
				savePatrolJob(fleet, job)
				continue
			}
			planets := t.game.getPlanets(player.Num)
			if len(planets) > 0 {
				if fleet.Idle() {
					job.Cycle++
				}
				dest := planets[(fleet.Num+job.Cycle)%len(planets)]
				position := dest.Position
				angle := float64((fleet.Num*7+job.Cycle)%12) * math.Pi / 6
				position.X += int(math.Round(30 * math.Cos(angle)))
				position.Y += int(math.Round(30 * math.Sin(angle)))
				t.patrolRoute(player, fleet, position, false)
			}
			job.Status = "Patrolling owned territory"
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
