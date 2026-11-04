package cs

import "fmt"

// Design defaults apply only to new construction. A replacement's actual
// existing role and unfinished mission take precedence over a design default.
func ValidateDesignAutoRole(role string, spec ShipDesignSpec) error {
	if role == "" || role == "manual" {
		return nil
	}
	if spec.Starbase {
		return fmt.Errorf("starbases do not use fleet Auto orders")
	}
	if dedicatedMinesweeperSpec(spec) && role != "minesweeper" {
		return fmt.Errorf("Gatling-only ships are dedicated minesweepers; choose Minesweeper or Manual")
	}
	valid := false
	switch role {
	case "scout":
		valid = spec.Scanner || !spec.HasWeapons
	case "freighter":
		valid = spec.CargoCapacity > 0
	case "patrol", "escort", "combat":
		valid = spec.HasWeapons
	case "miner":
		valid = spec.MiningRate > 0
	case "minesweeper":
		valid = spec.MineSweep > 0
	case "terraformer":
		valid = spec.TerraformRate > 0
	case "bomber":
		valid = spec.Bomber
	case "tanker":
		valid = spec.FuelCapacity > 0
	default:
		valid = colonizationMode(role) != "" && spec.Colonizer
	}
	if !valid {
		return fmt.Errorf("Auto orders role %q requires suitable ship equipment", role)
	}
	return nil
}
