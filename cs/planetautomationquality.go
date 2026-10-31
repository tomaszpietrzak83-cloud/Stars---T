package cs

// Instaforming uses the base environment, exactly as the turn engine does.
// Do not reject a currently red planet which becomes habitable on occupation.
func prospectivePlanetQuality(player *Player, planet *Planet) int {
	hab := planet.Hab
	if player.Race.Spec.Instaforming || player.Automation.ServeTerraformingWorlds {
		base := planet.BaseHab
		if base == (Hab{}) {
			base = hab
		}
		from := hab
		if player.Race.Spec.Instaforming {
			from = base
		}
		amount := NewTerraformer().GetTerraformAmount(from, base, player, player)
		hab = from.Add(amount)
	}
	return player.Race.GetPlanetHabitability(hab)
}
