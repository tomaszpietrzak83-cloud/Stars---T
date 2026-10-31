package cs

import (
	"fmt"
	"maps"
)

// Family IDs are negative counters, independent of reusable design numbers.
// The latest version is the successor of every previous version on the same hull.
func (p *Player) RegisterDesignUpgrade(source, next *ShipDesign) error {
	if source == nil || next == nil || source.PlayerNum != p.Num || next.PlayerNum != p.Num || source.OriginalPlayerNum != 0 || source.Spec.Starbase || source.Hull != next.Hull || source.Num == next.Num {
		return fmt.Errorf("upgrade requires a different owned design on the same ship hull")
	}
	if p.Automation.DesignFamilies == nil {
		p.Automation.DesignFamilies = map[int]int{}
	}
	family := p.Automation.DesignFamilies[source.Num]
	if family == 0 {
		family = -1
		for _, id := range p.Automation.DesignFamilies {
			family = min(family, id-1)
		}
		p.Automation.DesignFamilies[source.Num] = family
	}
	next.Version = max(1, source.Version) + 1
	for _, design := range p.Designs {
		if p.Automation.DesignFamilies[design.Num] == family {
			if design.Hull != next.Hull {
				return fmt.Errorf("design family cannot span different hulls")
			}
			next.Version = max(next.Version, design.Version+1)
		}
	}
	p.Automation.DesignFamilies[next.Num] = family
	if p.DesignMaintenance == nil {
		p.DesignMaintenance = DesignMaintenance{}
	}
	for _, design := range p.Designs {
		if design.Num != next.Num && p.Automation.DesignFamilies[design.Num] == family {
			p.DesignMaintenance[design.Num] = next.Num
		}
	}
	delete(p.DesignMaintenance, next.Num)
	// Pending compositions follow the replacement, just like production. Keep
	// completed one-shot orders historical; upgrading must not rebuild them.
	for id, plan := range p.Automation.FleetAssemblies {
		if plan.CompletedRevision == plan.Revision {
			continue
		}
		designs := maps.Clone(plan.Designs)
		changed := false
		for num, count := range plan.Designs {
			if num != next.Num && p.Automation.DesignFamilies[num] == family {
				delete(designs, num)
				designs[next.Num] += count
				changed = true
			}
		}
		if changed {
			plan.Designs = designs
			plan.Revision++
			plan.Status = "Composition updated to the latest design version"
			p.Automation.FleetAssemblies[id] = plan
		}
	}
	return nil
}
