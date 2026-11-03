package cs

import "fmt"

func priorityFleetKey(playerNum, fleetNum int) string {
	return fmt.Sprintf("%d:%d", playerNum, fleetNum)
}

func priorityFleet(p *Player, f *Fleet) bool {
	return f != nil && f.PlayerNum != p.Num && !p.IsFriend(f.PlayerNum) && p.Automation.PriorityFleets[priorityFleetKey(f.PlayerNum, f.Num)]
}

// Disappearing from scanner range is not proof that a fleet was destroyed.
// Only a battle report received by this player completes the destruction order.
func completeFleetPriorities(p *Player, record *BattleRecord) {
	for _, fleet := range record.fleets {
		if fleet.Delete {
			delete(p.Automation.PriorityFleets, priorityFleetKey(fleet.PlayerNum, fleet.Num))
		}
	}
}

func (t *turnGenerator) completePlanetPriorities() {
	for _, p := range t.automaticPlayers() {
		for num := range p.Automation.PriorityPlanets {
			if planet := t.game.getPlanet(num); planet != nil && planet.OwnedBy(p.Num) {
				delete(p.Automation.PriorityPlanets, num)
			}
		}
	}
}
