package cs

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
)

type FleetRoutePreview struct {
	ID        int64      `json:"id"`
	Num       int        `json:"num"`
	Name      string     `json:"name"`
	Role      string     `json:"role"`
	Waypoints []Waypoint `json:"waypoints"`
	Tags      Tags       `json:"tags"`
}
type AutomaticRoutesPreview struct {
	GameID    int64               `json:"gameId"`
	Year      int                 `json:"year"`
	PlayerNum int                 `json:"playerNum"`
	Fleets    []FleetRoutePreview `json:"fleets"`
}

// Preview replans a private snapshot. It does not advance time, move a ship,
// spend fuel, resolve combat or change the source game's cargo/production.
// Only waypoints/tags are returned; callers must never save this scratch game.
func (g *FullGame) cloneAutomationState(playerNum int) (*FullGame, error) {
	type snapshot struct {
		Game     *Game
		Universe *Universe
		Players  []*Player
	}
	players := make([]*Player, len(g.Players))
	for i, original := range g.Players {
		local := *original
		if local.Num != playerNum {
			local.Intels = Intels{}
			local.Messages = nil
			local.Stats = nil
		}
		players[i] = &local
	}
	data, err := json.Marshal(snapshot{g.Game, g.Universe, players})
	if err != nil {
		return nil, err
	}
	var copied snapshot
	if err = json.Unmarshal(data, &copied); err != nil {
		return nil, err
	}
	work := &FullGame{copied.Game, copied.Universe, g.TechStore, copied.Players}
	work.Rules = g.Rules
	work.Universe.setLogger(slog.Default())
	if err = work.computeSpecs(); err != nil {
		return nil, err
	}
	return work, nil
}

func (g *FullGame) PreviewAutomaticRoutes(playerNum int) (*AutomaticRoutesPreview, error) {
	work, err := g.cloneAutomationState(playerNum)
	if err != nil {
		return nil, err
	}
	p := work.getPlayer(playerNum)
	if p == nil {
		return nil, fmt.Errorf("player does not exist")
	}
	t := newTurnGenerator(work)
	t.ordersOnly = true
	// Keep all player indices intact; filter only the automation controllers.
	t.ordersPlayerNum = p.Num
	t.refreshAutomaticFuelIntel()
	t.planScoutAutomation()
	t.planMiningAutomation()
	t.planTerraformAutomation()
	t.planColonizationAutomation()
	t.planLogisticsAutomation()
	sourceJobs := map[int]string{}
	for _, f := range work.getFleets(p.Num) {
		sourceJobs[f.Num] = f.GetTag(logisticsTag)
	}
	// Predict loading only inside the snapshot, so delivery/return waypoints
	// can already be displayed before the actual next turn loads the ship.
	t.resolveLogisticsAutomation()
	t.dispatchReadyLogistics()
	for _, f := range work.getFleets(p.Num) {
		if !patrolAvailable(p, f) {
			continue
		}
		if f.Fuel < f.Spec.FuelCapacity/4 {
			t.returnPatrolToFuel(p, f)
		} else if readPatrolJob(f).Leader == 0 && f.Idle() {
			t.patrolFrontier(p, f)
		}
	}
	t.planEscortStaging()
	t.guardAutomaticFuel()
	t.planAutomaticTankers()
	t.enforceAutomaticDeparture()
	result := &AutomaticRoutesPreview{GameID: g.ID, Year: g.Year, PlayerNum: playerNum, Fleets: []FleetRoutePreview{}}
	for _, original := range g.Fleets {
		if original.PlayerNum != playerNum || original.Delete || original.Starbase {
			continue
		}
		f := work.getFleet(playerNum, original.Num)
		if f == nil || f.Delete {
			return nil, fmt.Errorf("route preview cannot remove fleets")
		}
		// The real loading phase must still execute. Persist its source job,
		// not the simulated already-loaded delivery state.
		if f.Cargo != original.Cargo {
			if job := sourceJobs[f.Num]; job != "" {
				f.SetTag(logisticsTag, job)
			}
		}
		if !reflect.DeepEqual(f.Waypoints, original.Waypoints) || !reflect.DeepEqual(f.Tags, original.Tags) {
			result.Fleets = append(result.Fleets, FleetRoutePreview{ID: original.ID, Num: f.Num, Name: f.Name, Role: p.Automation.FleetRoles[f.Num], Waypoints: f.Waypoints, Tags: f.Tags})
		}
	}
	return result, nil
}
