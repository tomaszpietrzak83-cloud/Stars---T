package cs

import (
	"fmt"
	"slices"
	"strings"
)

type maintenanceReport struct {
	scrapped, built     map[int]int
	progress, completed map[string]bool
	recovered           Cost
}

func (t *turnGenerator) maintenanceReport(p *Player) *maintenanceReport {
	if t.maintenanceReports == nil {
		t.maintenanceReports = map[int]*maintenanceReport{}
	}
	r := t.maintenanceReports[p.Num]
	if r == nil {
		r = &maintenanceReport{scrapped: map[int]int{}, built: map[int]int{}, progress: map[string]bool{}, completed: map[string]bool{}}
		t.maintenanceReports[p.Num] = r
	}
	return r
}

func maintenanceDesignCounts(p *Player, counts map[int]int) (int, string) {
	keys := make([]int, 0, len(counts))
	for num := range counts {
		keys = append(keys, num)
	}
	slices.Sort(keys)
	total := 0
	parts := []string{}
	for _, num := range keys {
		name := fmt.Sprintf("design #%d", num)
		if design := p.GetDesign(num); design != nil {
			name = design.Name
		}
		total += counts[num]
		parts = append(parts, fmt.Sprintf("%d x %s", counts[num], name))
	}
	return total, strings.Join(parts, ", ")
}

// Count physical events, not requested orders. A multi-year rebuild reports
// scrapping now and completed construction only in the year it actually occurs.
func (t *turnGenerator) reportShipMaintenance() {
	for _, p := range t.game.Players {
		r := t.maintenanceReports[p.Num]
		if r == nil {
			continue
		}
		parts := []string{}
		if n, detail := maintenanceDesignCounts(p, r.scrapped); n > 0 {
			parts = append(parts, fmt.Sprintf("Scrapped %d ships (%s)", n, detail))
			parts = append(parts, fmt.Sprintf("recovered %d kT Ironium, %d kT Boranium, %d kT Germanium and %d resources", r.recovered.Ironium, r.recovered.Boranium, r.recovered.Germanium, r.recovered.Resources))
		}
		if n, detail := maintenanceDesignCounts(p, r.built); n > 0 {
			parts = append(parts, fmt.Sprintf("Built %d replacement ships (%s)", n, detail))
		}
		if len(r.completed) > 0 {
			parts = append(parts, fmt.Sprintf("Completed %d fleet rebuilds", len(r.completed)))
		}
		for group := range r.completed {
			delete(r.progress, group)
		}
		if len(r.progress) > 0 {
			parts = append(parts, fmt.Sprintf("%d other fleet rebuilds reached 70%% regrouping", len(r.progress)))
		}
		if len(parts) > 0 {
			p.Messages = append(p.Messages, PlayerMessage{Type: PlayerMessageShipUpgrades, Text: fmt.Sprintf("Ship upgrades and retirements (%d): %s.", t.game.Year, strings.Join(parts, "; "))})
		}
	}
	t.maintenanceReports = nil
}
