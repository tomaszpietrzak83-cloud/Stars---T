package cs

// Occupation support is a destination policy, not a reason to raise the size of
// the initial invasion. Existing manual passenger thresholds remain available.
func colonyPopulationPercent(p *Player, planet *Planet) int {
	if p.Automation.OccupationSupport[planet.Num] {
		return 75
	}
	percent := p.Automation.destinationPopulationPercent()
	if !p.Automation.ManualPassengerThresholds && planet.Spec.Habitability > 0 && planet.Spec.Habitability < 75 {
		percent = max(percent, 50)
	}
	return percent
}

func (t *turnGenerator) updateOccupationSupport() {
	for _, p := range t.automaticPlayers() {
		for num := range p.Automation.OccupationSupport {
			planet := t.game.getPlanet(num)
			if planet == nil || !planet.OwnedBy(p.Num) || planet.Spec.HasStarbase {
				delete(p.Automation.OccupationSupport, num)
			}
		}
	}
}
