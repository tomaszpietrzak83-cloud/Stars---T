package cs

import "slices"

// Group automatic combat ships where they physically meet and have the same
// remaining orders. Keeping each design in one token lets it share battle damage.
func (t *turnGenerator) mergeCompatibleCombatFleets() {
	type groupKey struct {
		position Vector
		role     string
		design   int
		plan     int
	}
	for _, player := range t.automaticPlayers() {
		if len(player.Automation.FleetRoles) == 0 {
			continue
		}
		fleets := t.game.getFleets(player.Num)
		referenced := combatMergeReferences(player, fleets)
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		groups := make(map[groupKey][]*Fleet)
		for _, fleet := range fleets {
			role := player.Automation.FleetRoles[fleet.Num]
			if referenced[fleet.Num] || !canGroupCombatFleet(fleet, role) {
				continue
			}
			key := groupKey{fleet.Position, role, fleet.Tokens[0].DesignNum, fleet.BattlePlanNum}
			merged := false
			for _, leader := range groups[key] {
				if !sameCombatMovement(leader, fleet) {
					continue
				}
				if _, err := NewOrderer().Merge(&t.game.Rules, player, []*Fleet{leader, fleet}); err != nil {
					continue
				}
				t.game.deleteFleet(fleet)
				delete(player.Automation.FleetRoles, fleet.Num)
				merged = true
				break
			}
			if !merged {
				groups[key] = append(groups[key], fleet)
			}
		}
	}
}

func canGroupCombatFleet(fleet *Fleet, role string) bool {
	if role != "escort" || fleet.Delete || fleet.Starbase || !fleet.Spec.HasWeapons || len(fleet.Tokens) != 1 || len(fleet.Waypoints) == 0 || fleet.RepeatOrders || fleet.struckMinefield {
		return false
	}
	// These tags can safely be shared when their values match. Other tags
	// may track a particular fleet, cargo reservation or saved recovery job.
	for tag := range fleet.Tags {
		switch tag {
		case fuelStatusTag, frontierPatrolTag, patrolSectorTag, escortStagingTag, escortTag, bomberBorrowTag:
		default:
			return false
		}
	}
	for _, wp := range fleet.Waypoints {
		if wp.Task != WaypointTaskNone || wp.WarpSpeed == StargateWarpSpeed {
			return false
		}
	}
	return true
}

func sameCombatMovement(a, b *Fleet) bool {
	if a.Purpose != b.Purpose || a.OrbitingPlanetNum != b.OrbitingPlanetNum {
		return false
	}
	for key, value := range a.Tags {
		if key != fuelStatusTag && b.Tags[key] != value {
			return false
		}
	}
	for key, value := range b.Tags {
		if key != fuelStatusTag && a.Tags[key] != value {
			return false
		}
	}
	return slices.EqualFunc(a.Waypoints[1:], b.Waypoints[1:], func(a, b Waypoint) bool {
		a.EstFuelUsage, b.EstFuelUsage = 0, 0
		return a == b
	})
}

func combatMergeReferences(player *Player, fleets []*Fleet) map[int]bool {
	referenced := make(map[int]bool)
	for num, rebuild := range player.Automation.RebuildFleets {
		if rebuild {
			referenced[num] = true
		}
	}
	for _, assembly := range player.Automation.FleetAssemblies {
		if assembly.CompletedRevision == assembly.Revision {
			continue
		}
		referenced[assembly.LeaderFleetNum] = true
		for num := range assembly.SourceFleets {
			referenced[num] = true
		}
	}
	for _, fleet := range fleets {
		for _, wp := range fleet.Waypoints {
			if wp.TargetType == MapObjectTypeFleet && wp.TargetPlayerNum == player.Num {
				referenced[wp.TargetNum] = true
			}
		}
		if job := logisticsWork(fleet); job != nil && job.Kind == "fleet" {
			referenced[job.TargetNum] = true
		}
		if req := readAutomaticFuelRequest(fleet); req != nil {
			for _, num := range req.Members {
				referenced[num] = true
			}
		}
		if job := readTankerJob(fleet); job != nil {
			referenced[job.TargetNum] = true
			for _, num := range job.Members {
				referenced[num] = true
			}
		}
	}
	return referenced
}
