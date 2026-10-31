package cs

import "fmt"

// Population thresholds are people, while Cargo.Colonists is hundreds of people.
type ResearchTaxPolicy struct {
	Basis           string `json:"basis,omitempty"`
	Enabled         bool   `json:"enabled,omitempty"`
	LeftoverUntil   int    `json:"leftoverUntil,omitempty"`
	MediumThreshold int    `json:"mediumThreshold,omitempty"`
	LargeThreshold  int    `json:"largeThreshold,omitempty"`
	SmallPercent    int    `json:"smallPercent,omitempty"`
	MediumPercent   int    `json:"mediumPercent,omitempty"`
	LargePercent    int    `json:"largePercent,omitempty"`
}

func (p ResearchTaxPolicy) Validate() error {
	if p.Basis != "" && p.Basis != "population" && p.Basis != "resources" {
		return fmt.Errorf("research tax basis must be population or resources")
	}
	if !p.Enabled {
		return nil
	}
	if p.LeftoverUntil < 0 || p.MediumThreshold < p.LeftoverUntil || p.LargeThreshold <= p.MediumThreshold || p.LargeThreshold > 100000000 {
		return fmt.Errorf("research population thresholds must satisfy 0 <= surplus-only < medium < large (maximum 100 million)")
	}
	for _, rate := range []int{p.SmallPercent, p.MediumPercent, p.LargePercent} {
		if rate < 0 || rate > 100 {
			return fmt.Errorf("colony research tax must be 0-100 percent")
		}
	}
	return nil
}
func (p ResearchTaxPolicy) Contribution(value int) (percent int, leftoversOnly bool) {
	if value < p.LeftoverUntil {
		return 0, true
	}
	if value >= p.LargeThreshold {
		return p.LargePercent, false
	}
	if value >= p.MediumThreshold {
		return p.MediumPercent, false
	}
	return p.SmallPercent, false
}
func (planet *Planet) applyResearchTax(player *Player) {
	if policy := player.Automation.ResearchTax; policy != nil && policy.Enabled {
		_, planet.ContributesOnlyLeftoverToResearch = policy.Contribution(policy.value(planet.Cargo.Colonists*100, planet.Spec.ResourcesPerYear))
	}
}

func (p ResearchTaxPolicy) value(population, resources int) int {
	if p.Basis == "resources" {
		return resources
	}
	return population
}
