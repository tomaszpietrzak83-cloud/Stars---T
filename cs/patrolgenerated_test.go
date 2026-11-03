//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func proposalTestPlayer() *Player {
	p := testPlayer().WithTechLevels(TechLevel{26, 26, 26, 26, 26, 26})
	p.Relations = nil
	p.Race.WithLRT(IFE).WithSpec(&rules)
	return p
}

// Construct from hull slots and available components, never from preset designs.
func generatedPatrolDesign(hullName string, profile, seed int) ShipDesign {
	p := proposalTestPlayer()
	h := StaticTechStore.GetHull(hullName)
	rng := rand.New(rand.NewSource(int64(seed)))
	d := ShipDesign{Name: fmt.Sprintf("G%03d-P%d", seed, profile), Hull: h.Name}
	armed := false
	for i, slot := range h.Slots {
		available := StaticTechStore.GetHullComponentsByHullSlotType(p, slot.Type, h.Name)
		choices := []*TechHullComponent{}
		filter := func(match func(*TechHullComponent) bool) {
			choices = nil
			for _, c := range available {
				if match(c) {
					choices = append(choices, c)
				}
			}
		}
		if slot.Type&HullSlotTypeEngine != 0 {
			filter(func(c *TechHullComponent) bool { return c.Category == TechCategoryEngine })
		} else if slot.Type&HullSlotTypeWeapon != 0 {
			category := TechCategoryBeamWeapon
			if profile%2 != 0 {
				category = TechCategoryTorpedo
			}
			filter(func(c *TechHullComponent) bool { return c.Category == category && c.Power > 0 && !c.DamageShieldsOnly })
		} else {
			category := []TechCategory{TechCategoryArmor, TechCategoryShield, TechCategoryElectrical, TechCategoryMechanical}[profile%4]
			filter(func(c *TechHullComponent) bool { return c.Category == category })
			if len(choices) == 0 {
				filter(func(c *TechHullComponent) bool {
					return c.Armor > 0 || c.Shield > 0 || c.TorpedoJamming > 0 || c.TorpedoBonus > 0 || c.MovementBonus > 0 || c.BeamBonus > 0
				})
			}
		}
		if len(choices) == 0 {
			if slot.Required {
				panic("no required component: " + h.Name)
			}
			continue
		}
		// Sorted names make seed behavior independent of store ranking order.
		slices.SortFunc(choices, func(a, b *TechHullComponent) int {
			if a.Name < b.Name {
				return -1
			}
			if a.Name > b.Name {
				return 1
			}
			return 0
		})
		c := choices[rng.Intn(len(choices))]
		quantity := slot.Capacity
		if !slot.Required && profile%2 == 0 {
			quantity = max(1, (quantity+1)/2)
		}
		d.Slots = append(d.Slots, ShipDesignSlot{HullSlotIndex: i + 1, HullComponent: c.Name, Quantity: quantity})
		armed = armed || c.Power > 0 && !c.DamageShieldsOnly
	}
	if !armed {
		panic("unarmed generated design: " + h.Name)
	}
	if err := d.Validate(&rules, p); err != nil {
		panic(err)
	}
	return d
}

func generatedPatrolDesigns() []ShipDesign {
	designs := []ShipDesign{}
	for i, h := range []string{Scout.Name, Destroyer.Name, Cruiser.Name, Battleship.Name, Nubian.Name, Privateer.Name} {
		for profile := 0; profile < 2; profile++ {
			designs = append(designs, generatedPatrolDesign(h, profile, 100+i*13+profile*7))
		}
	}
	return designs
}

func proposalUtilityDesigns() (ShipDesign, ShipDesign) {
	return ShipDesign{Name: "Generated observer", Hull: Scout.Name, Slots: []ShipDesignSlot{
		{HullSlotIndex: 1, HullComponent: FuelMizer.Name, Quantity: 1},
		{HullSlotIndex: 2, HullComponent: RhinoScanner.Name, Quantity: 1},
	}}, ShipDesign{Name: "Generated dock", Hull: SpaceStation.Name}
}

func TestGeneratedPatrolManifest(t *testing.T) {
	p := proposalTestPlayer()
	seen := map[string]bool{}
	for _, d := range generatedPatrolDesigns() {
		require.NoError(t, d.Validate(&rules, p))
		raw, err := json.Marshal(d)
		require.NoError(t, err)
		t.Logf("GENERATED %s", raw)
		seen[d.Hull] = true
	}
	require.Len(t, seen, 6)
}
