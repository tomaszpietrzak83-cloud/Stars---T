package cs

import "encoding/json"

const deliveryProgressTag = "logistics-progress"

type deliveryProgress struct {
	Target   int     `json:"target"`
	Year     int     `json:"year"`
	Distance float64 `json:"distance"`
}

// Reservations must not repeatedly evict cargo that has already been loaded.
// A failed mission may choose another receiver only after several complete
// years without getting closer, or when its receiver is no longer usable.
func (t *turnGenerator) deliveryStalled(f *Fleet, target *Planet) bool {
	var progress deliveryProgress
	_ = json.Unmarshal([]byte(f.GetTag(deliveryProgressTag)), &progress)
	distance := f.Position.DistanceTo(target.Position)
	if progress.Target != target.Num || distance+1 < progress.Distance || progress.Year > t.game.Year {
		progress = deliveryProgress{Target: target.Num, Year: t.game.Year, Distance: distance}
		raw, _ := json.Marshal(progress)
		f.SetTag(deliveryProgressTag, string(raw))
	}
	return distance > 0 && t.game.Year-progress.Year >= 6
}

func (t *turnGenerator) alignDeliveryInfrastructure(f *Fleet, dest *Planet) {
	if c := readInfrastructure(f); c.mass() > 0 && c.Destination != dest.Num {
		c.Destination = dest.Num
		saveInfrastructure(f, c)
	}
}

// A saved cargo hold is stronger evidence of unfinished work than the
// presence of a planning tag. Recover older saves which lost that tag.
func (t *turnGenerator) recoverLoadedDelivery(p *Player, f *Fleet) *logisticsJob {
	if f.Cargo.Colonists > 0 && automaticPassengersAllowed(p, f) {
		if dest := t.passengerDestination(p, f, 0); dest != nil {
			t.alignDeliveryInfrastructure(f, dest)
			return &logisticsJob{Kind: "settle", TargetNum: dest.Num, DeliveryNum: dest.Num, Position: dest.Position, Passengers: f.Cargo.Colonists}
		}
	}
	if f.Cargo.Colonists == 0 && f.carriesInfrastructure() {
		c := readInfrastructure(f)
		if dest := t.game.getPlanet(c.Destination); dest != nil && dest.OwnedBy(p.Num) {
			return &logisticsJob{Kind: "return", TargetNum: dest.Num, DeliveryNum: dest.Num, Position: dest.Position}
		}
	}
	return nil
}

// Mineral prefetch can also occupy passenger seats. Validate the final packed
// hold once all non-passenger cargo is known; restore only this pickup's exports.
func (t *turnGenerator) reconcileMixedInstallations(p *Player, f *Fleet, source, dest *Planet, passengers int, before infrastructureCargo) {
	packed := readInfrastructure(f)
	actual := min(passengers, f.availableColonistSpace())
	sf, sm, _ := installationCapacity(p, source, max(0, source.GetPopulation()-actual*100))
	df, dm, _ := installationCapacity(p, dest, dest.GetPopulation()+actual*100)
	for _, other := range t.game.getFleets(p.Num) {
		if other.Num != f.Num && !other.Delete {
			c := readInfrastructure(other)
			if c.Destination == dest.Num {
				df -= c.Factories
				dm -= c.Mines
			}
		}
	}
	factories := min(max(0, packed.Factories-before.Factories), max(0, (sf*95+99)/100-source.Factories, packed.Factories+dest.Factories-df))
	mines := min(max(0, packed.Mines-before.Mines), max(0, (sm*95+99)/100-source.Mines, packed.Mines+dest.Mines-dm))
	source.Factories += factories
	source.Mines += mines
	packed.Factories -= factories
	packed.Mines -= mines
	saveInfrastructure(f, packed)
}
