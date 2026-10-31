package server

import (
	"github.com/sirgwain/craig-stars/ai"
	"github.com/sirgwain/craig-stars/cs"
)

// Negative IDs identify immutable templates, never rows in the user's library.
func raceTemplates() []cs.Race {
	races := []cs.Race{cs.Humanoids(), cs.Rabbitoids(), cs.Insectoids()}
	names := map[cs.PRT][2]string{
		cs.HE: {"Hiveling", "Hiveminds"}, cs.SS: {"Veilwalker", "Veilwalkers"}, cs.WM: {"Ironclad", "Ironclads"}, cs.CA: {"Verdant", "Verdants"}, cs.IS: {"Sanctuary", "Sanctuaries"}, cs.SD: {"Warden", "Wardens"}, cs.PP: {"Lithovore", "Lithovores"}, cs.IT: {"Gatekeeper", "Gatekeepers"}, cs.AR: {"Aetherial", "Aetherials"}, cs.JoaT: {"Free Trader", "Free Traders"},
	}
	for _, base := range ai.Races {
		r := base
		r.Name, r.PluralName = names[r.PRT][0], names[r.PRT][1]
		r.Description = "Gotowy wariant rasy: " + string(r.PRT) + ". Można skopiować i zmodyfikować w kreatorze."
		races = append(races, r)
	}
	for _, entry := range []struct {
		name, plural string
		prt          cs.PRT
		traits       int
		field        cs.TechField
	}{
		{"EEE Asterian", "EEE Asterian Nomads", cs.IT, cs.EEEEfficientDrives | cs.EEEExtendedTanks | cs.EEELightweightHulls | cs.EEESlowMaintenance, cs.TechFieldNone},
		{"EEE Lithic", "EEE Lithic Guilds", cs.AR, cs.EEEExtractionSpecialists | cs.EEECargoEngineering | cs.EEEExpensiveShipbuilding | cs.EEEInefficientPropulsion | cs.EEELightweightHulls, cs.TechFieldNone},
		{"EEE Helix", "EEE Helix Conclave", cs.CA, cs.EEEResearchSpecialization | cs.EEESurveyNetworks | cs.EEELightweightHulls | cs.EEEExpensiveShipbuilding | cs.EEEInefficientPropulsion, cs.Biotechnology},
		{"EEE Reclaimer", "EEE Reclaimer Consortium", cs.IS, cs.EEEMaterialRecovery | cs.EEEMaintenanceCrews | cs.EEEFieldWorkshops | cs.EEEExpensiveShipbuilding | cs.EEEInefficientPropulsion | cs.EEELightweightHulls, cs.TechFieldNone},
	} {
		for _, base := range ai.Races {
			if base.PRT == entry.prt {
				r := base
				r.Name, r.PluralName = entry.name, entry.plural
				r.EEETraits, r.EEEResearchField = entry.traits, entry.field
				r.Description = "EEE — gotowa rasa z zaletami i wadami logistycznymi, przemysłowymi lub badawczymi."
				races = append(races, r)
				break
			}
		}
	}
	for i := range races {
		r := &races[i]
		// Keep every ready-to-play template within the ordinary point budget.
		for r.ComputeRacePoints(1650) < 0 && r.PopEfficiency < 25 {
			r.PopEfficiency++
		}
		r.ID = -int64(i + 1)
		r.UserID = 0
	}
	return races
}

func raceTemplate(id int64) *cs.Race {
	if id >= 0 {
		return nil
	}
	for _, race := range raceTemplates() {
		if race.ID == id {
			return &race
		}
	}
	return nil
}
