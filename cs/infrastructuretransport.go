package cs

import (
	"encoding/json"
	"fmt"
)

const infrastructureCargoTag = "infrastructure-cargo"

// Installations are actual dismantled buildings, not minerals or free production.
// Their packed mass participates in native fleet mass, fuel and cargo limits.
type infrastructureCargo struct {
	Factories   int `json:"factories"`
	Mines       int `json:"mines"`
	Defenses    int `json:"defenses"`
	Origin      int `json:"origin"`
	Destination int `json:"destination"`
}

func readInfrastructure(f *Fleet) infrastructureCargo {
	var c infrastructureCargo
	if f != nil {
		_ = json.Unmarshal([]byte(f.GetTag(infrastructureCargoTag)), &c)
	}
	c.Factories = max(0, c.Factories)
	c.Mines = max(0, c.Mines)
	c.Defenses = max(0, c.Defenses)
	return c
}
func (c infrastructureCargo) mass() int { return c.Factories*5 + c.Mines*3 + c.Defenses }
func saveInfrastructure(f *Fleet, c infrastructureCargo) {
	if c.mass() == 0 {
		delete(f.Tags, infrastructureCargoTag)
		return
	}
	raw, _ := json.Marshal(c)
	f.SetTag(infrastructureCargoTag, string(raw))
}
func (f *Fleet) CargoMass() int              { return f.Cargo.Total() + readInfrastructure(f).mass() }
func (f *Fleet) carriesInfrastructure() bool { return f != nil && readInfrastructure(f).mass() > 0 }
func installationSurplus(stock, capacity int) int {
	if capacity <= 0 {
		return 0
	}
	return max(0, stock-(capacity*95+99)/100)
}
func installationCapacity(player *Player, p *Planet, population int) (int, int, int) {
	population = min(max(0, population), p.Spec.MaxPopulation)
	factories, mines := 0, 0
	if !player.Race.Spec.InnateResources {
		factories = max(1, getMaxInstallations(player.Race.NumFactories, population))
	}
	if !player.Race.Spec.InnateMining {
		mines = max(1, getMaxInstallations(player.Race.NumMines, population))
	}
	return factories, mines, p.Spec.MaxDefenses
}

// Pickup is only called during another already funded logistics pickup.
// It never creates a route or goes searching for a source planet.
func (t *turnGenerator) loadOpportunisticInfrastructure(player *Player, f *Fleet, source, dest *Planet, passengers int) {
	if !player.Automation.InfrastructureTransport || source == nil || dest == nil || !source.OwnedBy(player.Num) || !dest.OwnedBy(player.Num) || source.Num == dest.Num || f.Position != source.Position {
		return
	}
	packed := readInfrastructure(f)
	if packed.mass() > 0 && packed.Destination != dest.Num {
		return
	}
	sf, sm, sd := installationCapacity(player, source, source.GetPopulation())
	df, dm, dd := installationCapacity(player, dest, dest.GetPopulation()+passengers*100)
	for _, other := range t.game.getFleets(player.Num) {
		if other.Num != f.Num && !other.Delete {
			incoming := readInfrastructure(other)
			if incoming.Destination == dest.Num {
				df -= incoming.Factories
				dm -= incoming.Mines
				dd -= incoming.Defenses
			}
		}
	}
	room := min(f.availableCargoSpace()-min(1, passengers), max(0, f.Spec.CargoCapacity/4-packed.mass()))
	if room <= 0 {
		return
	}
	factories := min(installationSurplus(source.Factories, sf), max(0, df-dest.Factories-packed.Factories), room/5)
	room -= factories * 5
	mines := min(installationSurplus(source.Mines, sm), max(0, dm-dest.Mines-packed.Mines), room/3)
	room -= mines * 3
	defenses := min(installationSurplus(source.Defenses, sd), max(0, dd-dest.Defenses-packed.Defenses), room)
	source.Factories -= factories
	source.Mines -= mines
	source.Defenses -= defenses
	packed.Factories += factories
	packed.Mines += mines
	packed.Defenses += defenses
	packed.Origin = source.Num
	packed.Destination = dest.Num
	saveInfrastructure(f, packed)
}
func (t *turnGenerator) unloadOpportunisticInfrastructure() {
	for _, player := range t.game.Players {
		for _, f := range t.game.getFleets(player.Num) {
			if f.Delete || !f.carriesInfrastructure() {
				continue
			}
			c := readInfrastructure(f)
			destination := t.game.getPlanet(c.Destination)
			for _, p := range t.game.getPlanets(player.Num) {
				if p.Position != f.Position {
					continue
				}
				if p.Num != c.Destination && destination != nil && destination.OwnedBy(player.Num) {
					continue
				}
				p.Factories += c.Factories
				p.Mines += c.Mines
				p.Defenses += c.Defenses
				delete(f.Tags, infrastructureCargoTag)
				f.SetTag(fuelStatusTag, fmt.Sprintf("Delivered %d factories, %d mines and %d defenses", c.Factories, c.Mines, c.Defenses))
				break
			}
		}
	}
}
func (t *turnGenerator) topUpLogisticsCargo(player *Player, f *Fleet, source, dest *Planet) {
	if source == nil || dest == nil || source.Num == dest.Num || !source.OwnedBy(player.Num) || !dest.OwnedBy(player.Num) {
		return
	}
	demand := t.productionMinerals(player, dest, false)
	reserve := t.productionMinerals(player, source, false)
	room := f.availableCargoSpace()
	// Germanium first supports factories; only load minerals actually needed there.
	for _, kind := range []CargoType{Germanium, Ironium, Boranium} {
		amount := min(room, max(0, demand.GetAmount(kind)-dest.Cargo.GetAmount(kind)-f.Cargo.GetAmount(kind)), max(0, source.Cargo.GetAmount(kind)-reserve.GetAmount(kind)))
		if amount > 0 {
			source.Cargo = source.Cargo.SubtractAmount(kind, amount)
			f.Cargo = f.Cargo.AddAmount(kind, amount)
			room -= amount
		}
	}
}
