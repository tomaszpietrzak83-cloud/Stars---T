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
		if raw := f.GetTag(infrastructureCargoTag); raw != "" {
			_ = json.Unmarshal([]byte(raw), &c)
		}
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
func installationSurplus(stock, capacity int, configured ...int) int {
	percent := 95
	if len(configured) > 0 {
		percent = configured[0]
	}
	if capacity <= 0 {
		return 0
	}
	return max(0, stock-(capacity*percent+99)/100)
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
func (t *turnGenerator) loadOpportunisticInfrastructure(player *Player, f *Fleet, source, dest *Planet, passengers int, reservedPassengers ...int) {
	if !player.Automation.InfrastructureTransport || source == nil || dest == nil || !source.OwnedBy(player.Num) || !suitableColony(player, dest) || source.Num == dest.Num || f.Position != source.Position {
		return
	}
	packed := readInfrastructure(f)
	policy := player.Automation.transportPolicy()
	if packed.mass() > 0 && packed.Destination != dest.Num {
		return
	}
	sf, sm, _ := installationCapacity(player, source, max(0, source.GetPopulation()-passengers*100))
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
	factories := min(installationSurplus(source.Factories, sf, policy.InstallationReservePercent), max(0, df-dest.Factories-packed.Factories), room/5)
	room -= factories * 5
	mines := min(installationSurplus(source.Mines, sm, policy.InstallationReservePercent), max(0, dm-dest.Mines-packed.Mines), room/3)
	room -= mines * 3
	defenses := min(max(0, source.Defenses-policy.DefenseReserve), max(0, dd-dest.Defenses-packed.Defenses), room)
	source.Factories -= factories
	source.Mines -= mines
	source.Defenses -= defenses
	// Packed buildings displace some of the proposed passengers. Recheck the
	// retained source capacity against the ACTUAL mixed load, not the optimistic
	// all-passenger proposal. Returning excess buildings frees still more seats.
	reserved := 0
	if len(reservedPassengers) > 0 {
		reserved = reservedPassengers[0]
	}
	actualPassengers := min(passengers, max(0, f.availableColonistSpace()+reserved-factories*5-mines*3-defenses))
	sf, sm, _ = installationCapacity(player, source, max(0, source.GetPopulation()-actualPassengers*100))
	restoreFactories := min(factories, max(0, (sf*policy.InstallationReservePercent+99)/100-source.Factories))
	restoreMines := min(mines, max(0, (sm*policy.InstallationReservePercent+99)/100-source.Mines))
	source.Factories += restoreFactories
	source.Mines += restoreMines
	factories -= restoreFactories
	mines -= restoreMines
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
				if !suitableColony(player, p) || p.Num != c.Destination && suitableColony(player, destination) {
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
	if colonizationMode(player.Automation.FleetRoles[f.Num]) != "" {
		return
	}
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
