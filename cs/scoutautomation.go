package cs

import (
	"encoding/json"
	"math"
	"slices"
)

const scoutTag = "scout-job"
const scoutMissionTag = "scout-mission"

type scoutMission struct {
	Warps      []int   `json:"warps,omitempty"`
	Plan       []int   `json:"plan,omitempty"`
	Leg        int     `json:"leg,omitempty"`
	ReturnAt   int     `json:"returnAt,omitempty"`
	Home       int     `json:"home,omitempty"`
	Cycle      int     `json:"cycle,omitempty"`
	Refuelling bool    `json:"refuelling,omitempty"`
	Origin     Vector  `json:"origin"`
	Radius     float64 `json:"radius"`
	Target     int     `json:"target"`
	Complete   bool    `json:"complete"`
}

func readScoutMission(fleet *Fleet) *scoutMission {
	var mission scoutMission
	if json.Unmarshal([]byte(fleet.GetTag(scoutMissionTag)), &mission) != nil {
		return nil
	}
	return &mission
}

func saveScoutMission(fleet *Fleet, mission *scoutMission) {
	data, _ := json.Marshal(mission)
	fleet.SetTag(scoutMissionTag, string(data))
}

func scoutRetirementDeferred(player *Player, fleet *Fleet) bool {
	if player.Automation.FleetRoles[fleet.Num] != "scout" || fleet.InDesignMaintenance() {
		return false
	}
	mission := readScoutMission(fleet)
	if mission != nil && (mission.Refuelling || len(fleet.Waypoints) <= 1) {
		for _, dock := range expeditionStations(player, fleet) {
			if dock.Position == fleet.Position {
				return false
			}
		}
	}
	return mission == nil || !mission.Complete
}

// Use only the player's reports. A stale explored report is still explored;
// basic non-penetrating scans leave ReportAgeUnexplored unchanged.
func scoutRouteSafe(player *Player, from, to Vector) bool {
	if knownRouteThreat(player, from, to) > 0 {
		return false
	}
	// Minefield traversal is checked at each candidate warp by the shared planner.
	return true
}

func (t *turnGenerator) planScoutAutomation() {
	for _, player := range t.automaticPlayers() {
		fleets := slices.Clone(t.game.getFleets(player.Num))
		slices.SortFunc(fleets, func(a, b *Fleet) int { return a.Num - b.Num })
		claimed := map[int]int{}
		active := 0
		unknown := 0
		for _, planet := range player.PlanetIntels {
			if planet != nil && planet.Unexplored() {
				unknown++
			}
		}
		for _, fleet := range fleets {
			if !fleet.Delete && player.Automation.FleetRoles[fleet.Num] == "scout" {
				active++
				if mission := readScoutMission(fleet); mission != nil {
					for _, num := range mission.Plan[min(mission.Leg, len(mission.Plan)):] {
						if planet := player.GetPlanetIntel(num); planet != nil && planet.Unexplored() && claimed[num] == 0 {
							claimed[num] = fleet.Num
						}
					}
				}
			}
		}
		quota := max(1, min(32, unknown/max(1, active)))
		for _, fleet := range fleets {
			if fuelNetworkManaged(fleet) || fleet.InAutomaticSafety() || fleet.Delete || fleet.Starbase || fleet.InDesignMaintenance() || player.Automation.FleetRoles[fleet.Num] != "scout" || fleet.GetTag(tankerAwaitTag) != "" {
				continue
			}
			if fleet.GetTag(preyWatchTag) != "" {
				continue
			}

			if !scoutHasRetirement(player, fleet) && t.planPriorityColonyRecon(player, fleet) {
				continue
			}
			if !scoutHasRetirement(player, fleet) && t.planScoutRecon(player, fleet) {
				continue
			}
			if t.planScannerExploration(player, fleet, claimed, quota) {
				continue
			}
			if unknown == 0 && !scoutHasRetirement(player, fleet) && t.planColonyRecon(player, fleet, false) {
				continue
			}
			if unknown == 0 && !scoutHasRetirement(player, fleet) && !fleet.InAutomaticSafety() && !scoutReconActive(player, fleet) && t.patrolFrontier(player, fleet) {
				if mission := readScoutMission(fleet); mission != nil && atOwnFuelDock(player, fleet) {
					mission.Complete = true
					saveScoutMission(fleet, mission)
				}
				fleet.SetTag(scoutTag, "Frontier reconnaissance: scanning for enemy traffic")
				continue
			}
			mission := readScoutMission(fleet)
			if mission == nil {
				mission = &scoutMission{Origin: fleet.Position}
				nearest := math.Inf(1)
				for _, station := range expeditionStations(player, fleet) {
					if distance := fleet.Position.DistanceTo(station.Position); distance < nearest {
						mission.Origin, mission.Home, nearest = station.Position, station.Num, distance
					}
				}
			}
			for num, owner := range claimed {
				if owner == fleet.Num {
					delete(claimed, num)
				}
			}
			t.advanceScoutPetal(player, fleet, mission, claimed, quota)
			for _, num := range mission.Plan[min(mission.Leg, len(mission.Plan)):] {
				if planet := player.GetPlanetIntel(num); planet != nil && planet.Unexplored() {
					claimed[num] = fleet.Num
				}
			}
			saveScoutMission(fleet, mission)
		}
	}
}
