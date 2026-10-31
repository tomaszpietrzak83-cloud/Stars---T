package cs

import (
	"encoding/json"
	"fmt"
	"slices"
)

const miningTag = "mining-job"
const miningMinimumStay = 10

type miningJob struct {
	PlanetNum   int    `json:"planetNum"`
	ArrivedYear int    `json:"arrivedYear"`
	Status      string `json:"status"`
}

func readMiningJob(fleet *Fleet) *miningJob {
	var job miningJob
	if json.Unmarshal([]byte(fleet.GetTag(miningTag)), &job) != nil {
		return nil
	}
	return &job
}

// Queue deficits are measured at each colony, so a large surplus on another
// world does not hide a local production bottleneck. Baseline weights keep
// mining useful even before the player queues new production.
func (t *turnGenerator) miningWeights(player *Player) [3]float64 {
	weights := [3]float64{1, 1, 1}
	deficit := [3]int{}
	for _, planet := range t.game.Planets {
		if !planet.OwnedBy(player.Num) {
			continue
		}
		need := t.productionMinerals(player, planet, false)
		for i, mineral := range MineralTypes {
			deficit[i] += max(0, need.GetAmount(mineral)-planet.Cargo.GetAmount(mineral)-planet.Spec.MiningOutput.GetAmount(mineral)*miningMinimumStay)
		}
	}
	total := max(1, deficit[0]+deficit[1]+deficit[2])
	for i := range weights {
		weights[i] += 9 * float64(deficit[i]) / float64(total)
	}
	return weights
}

func miningValue(planet *Planet, weights [3]float64) float64 {
	value := 0.0
	for i, mineral := range MineralTypes {
		value += float64(max(0, planet.MineralConcentration.GetAmount(mineral))) * weights[i]
	}
	return value
}

func (t *turnGenerator) planMiningAutomation() {
	for _, player := range t.game.Players {
		weights := t.miningWeights(player)
		fleets := slices.Clone(t.game.getFleets(player.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		assigned := map[int]int{}
		for _, fleet := range fleets {
			if player.Automation.FleetRoles[fleet.Num] == "miner" && !fleet.Delete && !fleet.InDesignMaintenance() {
				if job := readMiningJob(fleet); job != nil {
					assigned[job.PlanetNum]++
				}
			}
		}
		for _, fleet := range fleets {
			if fuelNetworkManaged(fleet) || fleet.InAutomaticSafety() || fleet.Delete || fleet.Starbase || fleet.InDesignMaintenance() || player.Automation.FleetRoles[fleet.Num] != "miner" {
				continue
			}
			job := readMiningJob(fleet)
			if job == nil {
				job = &miningJob{}
			} else {
				assigned[job.PlanetNum]--
			}
			var current *Planet
			for _, planet := range player.PlanetIntels {
				if planet != nil && planet.Num == job.PlanetNum {
					current = planet
					break
				}
			}
			valid := current != nil && current.Explored() && fleet.CanRemoteMine(player, current)
			if valid && fleet.Position == current.Position && job.ArrivedYear == 0 {
				job.ArrivedYear = t.game.Year
			}
			chosen := current
			// Keep a valid travel assignment, and guarantee ten working turns
			// before considering economics again. Safety overrides the lease.
			locked := valid && (fleet.Position != current.Position || t.game.Year-job.ArrivedYear < miningMinimumStay)
			if !valid {
				chosen = nil
			}
			if !locked {
				best := 0.0
				if valid {
					best = miningValue(current, weights) / float64(1+assigned[current.Num]) * 1.5
				}
				for _, planet := range player.PlanetIntels {
					if planet == nil || !planet.Explored() || !fleet.CanRemoteMine(player, planet) || !scoutRouteSafe(player, fleet.Position, planet.Position) {
						continue
					}
					route := planExpeditionRoute(&t.game.Rules, player, fleet, planet)
					if route == nil {
						continue
					}
					travel := float64(route.Turns)
					score := miningValue(planet, weights) / float64(1+assigned[planet.Num]) * miningMinimumStay / (miningMinimumStay + travel)
					if score > best {
						chosen, best = planet, score
					}
				}
			}
			if chosen == nil {
				routeAutomatically(&t.game.Rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
				job.PlanetNum, job.ArrivedYear, job.Status = 0, 0, "Waiting for an explored, reachable mining planet"
				if expeditionRefuel(&t.game.Rules, player, fleet) {
					job.Status = "Returning or waiting at dock for mining expedition fuel"
				}
			} else {
				if job.PlanetNum != chosen.Num {
					job.ArrivedYear = 0
				}
				job.PlanetNum = chosen.Num
				assigned[chosen.Num]++
				route := t.planAutomaticMission(player, []*Fleet{fleet}, chosen.Position, chosen.MapObject.ToTarget(), automaticRoutePolicy{})
				if !scoutRouteSafe(player, fleet.Position, chosen.Position) || route == nil {
					routeAutomatically(&t.game.Rules, player, fleet, fleet.Position, MapObjectTarget{}, 0)
					job.Status = "Holding mining assignment: route unsafe or insufficient fuel"
					if route == nil && expeditionRefuel(&t.game.Rules, player, fleet) {
						job.Status = "Refuelling before resuming mining assignment"
					}
				} else {
					applyExpeditionRoute(player, fleet, route)
					if fleet.Position == chosen.Position {
						fleet.Waypoints = []Waypoint{{Position: fleet.Position, MapObjectTarget: chosen.MapObject.ToTarget()}}
					}
					fleet.Waypoints[len(fleet.Waypoints)-1].Task = WaypointTaskRemoteMining
					job.Status = fmt.Sprintf("Mining assignment: %s (minimum %d working turns)", chosen.Name, miningMinimumStay)
					if fleet.Position == chosen.Position && job.ArrivedYear == 0 {
						job.ArrivedYear = t.game.Year
					}
				}
			}
			data, _ := json.Marshal(job)
			fleet.SetTag(miningTag, string(data))
		}
	}
}
