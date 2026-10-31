package cs

import (
	"fmt"
	"reflect"
)

type PlanetProductionPreview struct {
	ID              int64                 `json:"id"`
	Num             int                   `json:"num"`
	Name            string                `json:"name"`
	ProductionQueue []ProductionQueueItem `json:"productionQueue"`
	Before          []int                 `json:"before"`
	After           []int                 `json:"after"`
}
type ProductionOrdersPreview struct {
	GameID    int64                     `json:"gameId"`
	Year      int                       `json:"year"`
	PlayerNum int                       `json:"playerNum"`
	Planets   []PlanetProductionPreview `json:"planets"`
}

// Preview queued development without changing the source's year, cargo,
// installations, designs or existing paid allocations.
func (g *FullGame) PreviewProductionOrders(playerNum int) (*ProductionOrdersPreview, error) {
	work, err := g.cloneAutomationState(playerNum)
	if err != nil {
		return nil, err
	}
	p := work.getPlayer(playerNum)
	if p == nil {
		return nil, fmt.Errorf("player does not exist")
	}
	result := &ProductionOrdersPreview{GameID: g.ID, Year: g.Year, PlayerNum: playerNum, Planets: []PlanetProductionPreview{}}
	for _, planet := range work.getPlanets(p.Num) {
		original := g.Planets[planet.Num-1]
		hasWork := false
		for _, item := range planet.ProductionQueue {
			hasWork = hasWork || manufacturingItem(item)
		}
		if !hasWork {
			continue
		}
		before, err := forecastManufacturing(&work.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
		if err != nil {
			return nil, err
		}
		PlanDefaultAuto(&work.Rules, p, planet)
		after, err := forecastManufacturing(&work.Rules, p, planet, planet.ProductionQueue, 0, Mineral{})
		if err != nil {
			return nil, err
		}
		if !manufacturingDatesNoLater(after, before) {
			return nil, fmt.Errorf("production preview delayed explicit work on planet %d", planet.Num)
		}
		if err = planet.PopulateProductionQueueEstimates(&work.Rules, p); err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(planet.ProductionQueue, original.ProductionQueue) {
			result.Planets = append(result.Planets, PlanetProductionPreview{ID: original.ID, Num: planet.Num, Name: planet.Name, ProductionQueue: planet.ProductionQueue, Before: before.Finish, After: after.Finish})
		}
	}
	return result, nil
}
