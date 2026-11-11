package cs

// Explicit player orders outrank high-potential expansion, which outranks
// ordinary targets. Potential uses terraforming available with current tech.
func expansionPriority(player *Player, planet *Planet) int {
	if planet == nil {
		return -1
	}
	if player.Automation.PriorityPlanets[planet.Num] {
		return 2
	}
	if terraformedPlanetQuality(player, planet) > 75 {
		return 1
	}
	return 0
}

// Instaforming uses the base environment, exactly as the turn engine does.
// Do not reject a currently red planet which becomes habitable on occupation.
func prospectivePlanetQuality(player *Player, planet *Planet) int {
	hab := planet.Hab
	if player.Race.Spec.Instaforming || player.Automation.ServeTerraformingWorlds {
		return terraformedPlanetQuality(player, planet)
	}
	return player.Race.GetPlanetHabitability(hab)
}

func terraformedPlanetQuality(player *Player, planet *Planet) int {
	hab := planet.Hab
	base := planet.BaseHab
	if base == (Hab{}) {
		base = hab
	}
	from := hab
	if player.Race.Spec.Instaforming {
		from = base
	}
	amount := NewTerraformer().GetTerraformAmount(from, base, player, player)
	return player.Race.GetPlanetHabitability(from.Add(amount))
}
