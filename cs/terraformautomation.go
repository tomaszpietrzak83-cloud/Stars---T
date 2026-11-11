package cs

import (
	"encoding/json"
	"fmt"
	"slices"
)

const terraformTag = "terraform-job"

type terraformJob struct {
	PlanetNum int    `json:"planetNum"`
	Status    string `json:"status"`
}

func readTerraformJob(f *Fleet) *terraformJob {
	var job terraformJob
	if json.Unmarshal([]byte(f.GetTag(terraformTag)), &job) != nil {
		return nil
	}
	return &job
}

// Orbital assistance is for other players marked as friends, never our own
// colonies. Probe a real step so instaformed or capped worlds cannot trap a fleet.
func (t *turnGenerator) terraformWork(player *Player, planet *Planet) bool {
	if planet == nil || !planet.Owned() || planet.OwnedBy(player.Num) || !player.IsFriend(planet.PlayerNum) {
		return false
	}
	owner := t.game.getPlayer(planet.PlayerNum)
	if owner == nil {
		return false
	}
	probe := *planet
	result := NewTerraformer().TerraformOneStep(&probe, owner, player, false)
	return result.Terraformed() && Abs(owner.Race.HabCenter().Get(result.Type)-probe.Hab.Get(result.Type)) < Abs(owner.Race.HabCenter().Get(result.Type)-planet.Hab.Get(result.Type))
}

func (t *turnGenerator) terraformTarget(player *Player, fleet *Fleet, num int) *Planet {
	if num != None && fleet.OrbitingPlanetNum == num {
		return t.game.getPlanet(num)
	}
	intel := player.GetPlanetIntel(num)
	if intel == nil || intel.ReportAge == ReportAgeUnexplored {
		return nil
	}
	// Foreign reports contain current/base conditions, but not the stored
	// terraformed amount. Reconstruct it without changing the player's report.
	planet := *intel
	if owner := t.game.getPlayer(planet.PlayerNum); owner != nil && !owner.Race.Spec.Instaforming {
		planet.TerraformedAmount = planet.Hab.Subtract(planet.BaseHab)
	}
	return &planet
}
func (t *turnGenerator) planTerraformAutomation() {
	for _, player := range t.automaticPlayers() {
		fleets := slices.Clone(t.game.getFleets(player.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		assigned := map[int]int{}
		for _, f := range fleets {
			if !f.Delete && player.Automation.FleetRoles[f.Num] == "terraformer" {
				if job := readTerraformJob(f); job != nil {
					assigned[job.PlanetNum]++
				}
			}
		}
		for _, fleet := range fleets {
			if fleet.GetTag("maintenance-pending") != "" || fuelNetworkManaged(fleet) || fleet.InAutomaticSafety() || fleet.Delete || fleet.Starbase || fleet.InDesignMaintenance() || fleet.GetTag(tankerAwaitTag) != "" || player.Automation.FleetRoles[fleet.Num] != "terraformer" || fleet.Spec.TerraformRate <= 0 {
				continue
			}
			job := readTerraformJob(fleet)
			if job == nil {
				job = &terraformJob{}
			} else {
				assigned[job.PlanetNum]--
			}
			chosen := t.terraformTarget(player, fleet, job.PlanetNum)
			if !t.terraformWork(player, chosen) {
				chosen = nil
			}
			// Keep an assignment until its available terraforming is finished.
			if chosen == nil {
				best := -1.0
				for _, intel := range player.PlanetIntels {
					if intel == nil || intel.ReportAge == ReportAgeUnexplored {
						continue
					}
					planet := t.terraformTarget(player, fleet, intel.Num)
					if !t.terraformWork(player, planet) {
						continue
					}
					route := t.planAutomaticMission(player, []*Fleet{fleet}, planet.Position, planet.MapObject.ToTarget(), automaticRoutePolicy{})
					if route == nil {
						continue
					}
					probe := *planet
					owner := t.game.getPlayer(planet.PlayerNum)
					NewTerraformer().TerraformOneStep(&probe, owner, player, false)
					gain := max(1, owner.Race.GetPlanetHabitability(probe.Hab)-owner.Race.GetPlanetHabitability(planet.Hab))
					score := float64(gain) * (1 + float64(planet.Cargo.Colonists)/1000) / float64((route.Turns+1)*(1+assigned[planet.Num]))
					if score > best {
						best, chosen = score, planet
					}
				}
			}
			if chosen == nil {
				job.PlanetNum = 0
				holdAutomaticFleet(player, fleet, "Waiting for an allied colony with available terraforming")
				job.Status = "No reachable allied colony needs terraforming"
				expeditionRefuel(&t.game.Rules, player, fleet)
			} else {
				job.PlanetNum = chosen.Num
				assigned[chosen.Num]++
				route := t.planAutomaticMission(player, []*Fleet{fleet}, chosen.Position, chosen.MapObject.ToTarget(), automaticRoutePolicy{})
				if route == nil {
					holdAutomaticFleet(player, fleet, "Waiting for a safe terraforming route")
					expeditionRefuel(&t.game.Rules, player, fleet)
					job.Status = "Refuelling for terraforming assignment"
				} else {
					applyExpeditionRoute(player, fleet, route)
					job.Status = fmt.Sprintf("Allied terraforming assignment: %s", chosen.Name)
					if fleet.Position == chosen.Position {
						fleet.Waypoints = []Waypoint{{Position: fleet.Position, MapObjectTarget: chosen.MapObject.ToTarget()}}
						job.Status = fmt.Sprintf("Terraforming %s until no further improvement is available", chosen.Name)
					}
				}
			}
			data, _ := json.Marshal(job)
			fleet.SetTag(terraformTag, string(data))
		}
	}
}
