package cs

import "fmt"

type TransportPolicy struct {
	ColonyHabitabilityCutoff   int  `json:"colonyHabitabilityCutoff,omitempty"`
	ColonyPopulationTarget     int  `json:"colonyPopulationTarget,omitempty"`
	CapturedPopulationTarget   int  `json:"capturedPopulationTarget,omitempty"`
	DefenseReserve             int  `json:"defenseReserve,omitempty"`
	InstallationReservePercent int  `json:"installationReservePercent,omitempty"`
	AttritionMinimumPeople     int  `json:"attritionMinimumPeople,omitempty"`
	LandingEscortShips         int  `json:"landingEscortShips,omitempty"`
	LandingsFirst              bool `json:"landingsFirst,omitempty"`
	DisableAttrition           bool `json:"disableAttrition,omitempty"`
	DisableRetarget            bool `json:"disableRetarget,omitempty"`
	DisableCapturedSupport     bool `json:"disableCapturedSupport,omitempty"`
}

func (a LogisticsAutomation) transportPolicy() TransportPolicy {
	p := TransportPolicy{}
	if a.TransportPolicy != nil {
		p = *a.TransportPolicy
	}
	for _, entry := range []struct {
		value    *int
		fallback int
	}{
		{&p.ColonyHabitabilityCutoff, 75}, {&p.ColonyPopulationTarget, 50},
		{&p.CapturedPopulationTarget, 75}, {&p.DefenseReserve, 80},
		{&p.InstallationReservePercent, 95}, {&p.AttritionMinimumPeople, 200000}, {&p.LandingEscortShips, 2},
	} {
		if *entry.value == 0 {
			*entry.value = entry.fallback
		}
	}
	return p
}

func (p TransportPolicy) Validate() error {
	for name, bound := range map[string][2]int{
		"colony habitability cutoff": {p.ColonyHabitabilityCutoff, 100},
		"colony population target":   {p.ColonyPopulationTarget, 99},
		"captured population target": {p.CapturedPopulationTarget, 99},
		"defense reserve":            {p.DefenseReserve, 1000},
		"installation reserve":       {p.InstallationReservePercent, 100},
		"attrition minimum people":   {p.AttritionMinimumPeople, 10000000},
		"landing escort ships":       {p.LandingEscortShips, 100},
	} {
		if bound[0] < 0 || bound[0] > bound[1] {
			return fmt.Errorf("%s must be between 1 and %d (0 uses default)", name, bound[1])
		}
	}
	if p.AttritionMinimumPeople != 0 && (p.AttritionMinimumPeople < 100 || p.AttritionMinimumPeople%100 != 0) {
		return fmt.Errorf("attrition minimum must be a positive multiple of 100 people")
	}
	return nil
}
