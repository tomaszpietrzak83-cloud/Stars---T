package cs

import (
	"fmt"
	"strconv"
)

type Salvage struct {
	GameDBObject
	MapObject
	Cargo Cargo `json:"cargo,omitempty"`
}

// create a new salvage object
func newSalvage(position Vector, num int, playerNum int, cargo Cargo) *Salvage {
	return &Salvage{
		MapObject: MapObject{
			Type:      MapObjectTypeSalvage,
			Position:  position,
			Num:       num,
			PlayerNum: playerNum,
			Name:      fmt.Sprintf("Salvage #%d", num),
		},
		Cargo: cargo,
	}
}

// Salvage at one position can mix battle debris with cargo dumps. Track the
// battle portion in the existing persisted tags, separately for each mineral.
var battleMineralTags = [3]string{"battle-ironium", "battle-boranium", "battle-germanium"}

func (s *Salvage) battleMinerals() (minerals Mineral) {
	for _, kind := range MineralTypes {
		amount, _ := strconv.Atoi(s.GetTag(battleMineralTags[kind]))
		minerals.Set(kind, Clamp(amount, 0, s.Cargo.GetAmount(kind)))
	}
	return
}

func (s *Salvage) setBattleMinerals(minerals Mineral) {
	for _, kind := range MineralTypes {
		key := battleMineralTags[kind]
		if amount := minerals.GetAmount(kind); amount > 0 {
			s.SetTag(key, strconv.Itoa(amount))
		} else {
			delete(s.Tags, key)
		}
	}
}

// A load takes battle minerals first when both sources share a debris field.
func (s *Salvage) recordRecovery(player *Player, year int, loaded Mineral) {
	if loaded == (Mineral{}) {
		return
	}
	// Call before updating the cargo, so the source amount can be bounded.
	remaining := s.battleMinerals()
	var recovered Mineral
	for _, kind := range MineralTypes {
		available := remaining.GetAmount(kind)
		taken := min(available, loaded.GetAmount(kind))
		recovered.Set(kind, taken)
		remaining.Set(kind, available-taken)
	}
	if recovered != (Mineral{}) {
		s.setBattleMinerals(remaining)
		player.recordRecoveredBattleMinerals(year, recovered)
	}
}

func (t *cargoTransferer) recordSalvageLoad(player *Player, source CargoHolder, kind CargoType, amount int) {
	if salvage, ok := source.(*Salvage); ok && amount > 0 && kind <= Germanium {
		var loaded Mineral
		loaded.Set(kind, amount)
		salvage.recordRecovery(player, t.game.Year, loaded)
	}
}

func (t *turnGenerator) recoverSalvage(player *Player, fleet *Fleet, salvage *Salvage) {
	after := salvage.Cargo
	takeMinerals(fleet, &after)
	salvage.recordRecovery(player, t.game.Year, salvage.Cargo.Subtract(after).ToMineral())
	salvage.Cargo = after
}

// decay this salvage
// https://wiki.starsautohost.org/wiki/Guts_of_scrapping
// Our edition uses 8% or 8kT per mineral/year (original Stars!: 10% or 10kT).
// Salvage deposited on planets does not decay.
func (salvage *Salvage) decay(rules *Rules) {
	before := salvage.Cargo
	battle := salvage.battleMinerals()
	salvage.Cargo = Cargo{
		Ironium: max(0, min(
			salvage.Cargo.Ironium-int(float64(salvage.Cargo.Ironium)*rules.SalvageDecayRate),
			salvage.Cargo.Ironium-rules.SalvageDecayMin,
		)),
		Boranium: max(0, min(
			salvage.Cargo.Boranium-int(float64(salvage.Cargo.Boranium)*rules.SalvageDecayRate),
			salvage.Cargo.Boranium-rules.SalvageDecayMin,
		)),
		Germanium: max(0, min(
			salvage.Cargo.Germanium-int(float64(salvage.Cargo.Germanium)*rules.SalvageDecayRate),
			salvage.Cargo.Germanium-rules.SalvageDecayMin,
		)),
	}
	// Apply decay to both sources in proportion to their share of each mineral.
	if battle != (Mineral{}) {
		for _, kind := range MineralTypes {
			if total := before.GetAmount(kind); total > 0 {
				battle.Set(kind, battle.GetAmount(kind)*salvage.Cargo.GetAmount(kind)/total)
			}
		}
		salvage.setBattleMinerals(battle)
	}
}
