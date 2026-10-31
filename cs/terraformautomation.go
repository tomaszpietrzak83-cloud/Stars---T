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

// Only owned colonies are selected: automatic terraforming never starts a war.
func terraformWork(player *Player, planet *Planet) bool {
	return planet != nil && planet.OwnedBy(player.Num) && NewTerraformer().GetBestTerraform(planet, player, player) != nil
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
			chosen := t.game.getPlanet(job.PlanetNum)
			if !terraformWork(player, chosen) {
				chosen = nil
			}
			// Keep an assignment until its available terraforming is finished.
			if chosen == nil {
				best := -1.0
				for _, planet := range t.game.Planets {
					if !terraformWork(player, planet) {
						continue
					}
					route := t.planAutomaticMission(player, []*Fleet{fleet}, planet.Position, planet.MapObject.ToTarget(), automaticRoutePolicy{})
					if route == nil {
						continue
					}
					probe := *planet
					NewTerraformer().TerraformOneStep(&probe, player, player, false)
					gain := max(1, player.Race.GetPlanetHabitability(probe.Hab)-player.Race.GetPlanetHabitability(planet.Hab))
					score := float64(gain) * (1 + float64(planet.Cargo.Colonists)/1000) / float64((route.Turns+1)*(1+assigned[planet.Num]))
					if score > best {
						best, chosen = score, planet
					}
				}
			}
			if chosen == nil {
				job.PlanetNum = 0
				holdAutomaticFleet(player, fleet, "Waiting for an owned colony with available terraforming")
				job.Status = "No reachable colony needs terraforming"
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
					job.Status = fmt.Sprintf("Terraforming assignment: %s", chosen.Name)
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
