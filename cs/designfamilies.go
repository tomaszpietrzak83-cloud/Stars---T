package cs

import "fmt"

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
	return nil
}
