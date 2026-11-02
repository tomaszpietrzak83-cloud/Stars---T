package cs

import (
	"strconv"
	"strings"
)

func (t *turnGenerator) currentLogisticsWork(p *Player, f *Fleet) *logisticsJob {
	job := logisticsWork(f)
	if job == nil {
		return nil
	}
	valid := true
	switch job.Kind {
	case "salvage":
		known := false
		for _, intel := range p.SalvageIntels {
			if intel != nil && intel.Num == job.TargetNum && !intel.Delete && intel.Cargo.ToMineral().Total() > 0 {
				known = true
				break
			}
		}
		valid = known
		if f.Position == job.Position {
			actual := t.game.getSalvage(job.TargetNum)
			valid = valid && actual != nil && !actual.Delete && actual.Cargo.ToMineral().Total() > 0
		}
		if !valid {
			f.SetTag("empty-salvage", strconv.Itoa(job.TargetNum))
		}
	case "fleet":
		source := t.game.getFleet(p.Num, job.TargetNum)
		valid = source != nil && !source.Delete && source.Cargo.ToMineral().Total() > 0
	case "planet", "passengers", "invade-load":
		source, dest := t.game.getPlanet(job.TargetNum), t.game.getPlanet(job.DeliveryNum)
		valid = source != nil && dest != nil && job.TargetNum != job.DeliveryNum
		if valid && job.Kind == "planet" && f.CargoMass() == 0 {
			spare := source.Cargo.ToMineral().ToCost().Subtract(t.productionMinerals(p, source, false).ToCost()).MinZero().ToMineral()
			useful := min(spare.Ironium, job.Load.Ironium) + min(spare.Boranium, job.Load.Boranium) + min(spare.Germanium, job.Load.Germanium)
			valid = useful > 0
			if valid && strings.HasPrefix(job.Status, "Waiting empty at source for a full") {
				probe, _ := t.previewShipyardPickup(p, f, source, dest, job.Load)
				valid = probe.CargoMass() == f.Spec.CargoCapacity
			}
		}
		if job.Kind == "passengers" {
			valid = valid && source.OwnedBy(p.Num) && suitableColony(p, dest)
			if valid {
				incoming, _ := t.passengerReservations(p, f.Num)
				room := populationTarget(dest, p.Automation.destinationPopulationPercent()) - dest.Cargo.Colonists - incoming[dest.Num]
				valid = room > 0 && t.destinationCanFillTransport(p, f, source, dest, room)
			}
		}
	}
	if valid {
		return job
	}
	delete(f.Tags, logisticsTag)
	t.releaseEscorts(p, f)
	return nil
}
