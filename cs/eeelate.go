package cs

// These drives buy strategic travel time with more mass, research, resources
// and fuel per ly. Rankings deliberately keep mature economical engines useful
// to the generic designer, which has no route distance to price the time saved.
func EEEStrategicEngines() []TechHullComponent {
	return []TechHullComponent{
		{Tech: NewTech("EEE Horizon Drive", NewCost(8, 2, 6, 18), TechRequirements{TechLevel: TechLevel{Propulsion: 20, Energy: 16, Electronics: 12}}, 120, TechCategoryEngine, TechTagEngine), Mass: 12, HullSlotType: HullSlotTypeEngine,
			Engine: Engine{IdealSpeed: 10, FreeSpeed: 1, MaxSafeSpeed: 10, StrategicSpeedFactor: 1.5, FuelUsage: []int{0, 0, 5, 15, 20, 25, 30, 35, 40, 50, 60}}},
		{Tech: NewTech("EEE Deep-Space Drive", NewCost(10, 4, 8, 24), TechRequirements{TechLevel: TechLevel{Propulsion: 23, Energy: 20, Electronics: 16}}, 121, TechCategoryEngine, TechTagEngine), Mass: 16, HullSlotType: HullSlotTypeEngine,
			Engine: Engine{IdealSpeed: 10, FreeSpeed: 1, MaxSafeSpeed: 10, StrategicSpeedFactor: 2, FuelUsage: []int{0, 0, 5, 15, 20, 25, 30, 35, 40, 55, 65}}},
		{Tech: NewTech("EEE Galactic Drive", NewCost(14, 6, 12, 32), TechRequirements{TechLevel: TechLevel{Propulsion: 26, Energy: 24, Electronics: 20, Construction: 22}}, 122, TechCategoryEngine, TechTagEngine), Mass: 22, HullSlotType: HullSlotTypeEngine,
			Engine: Engine{IdealSpeed: 10, FreeSpeed: 1, MaxSafeSpeed: 10, StrategicSpeedFactor: 3, FuelUsage: []int{0, 0, 5, 15, 20, 25, 30, 35, 40, 65, 75}}},
	}
}

// Keep each battery a separate slot: quantities affect firing stacks as well
// as capacity. The perimeter layout leaves cargo in the centre of transports.
func eeeLateSlots(slots ...TechHullSlot) []TechHullSlot {
	positions := []VectorFloat64{{-2, 0}, {-1, -1.5}, {0, -1.5}, {1, -1.5}, {2, -.5}, {2, .5}, {1, 1.5}, {0, 1.5}, {-1, 1.5}, {-1, -.5}, {0, -.5}}
	for i := range slots {
		slots[i].Position = positions[i]
		slots[i].Required = slots[i].Type == HullSlotTypeEngine
	}
	return slots
}

func EEELateHulls() []TechHull {
	return []TechHull{
		{Tech: NewTech("EEE Pathfinder", NewCost(18, 2, 16, 55), TechRequirements{TechLevel: TechLevel{Construction: 12, Propulsion: 10, Electronics: 12}}, 45, TechCategoryShipHull), Type: TechHullTypeScout, Class: ShipClassScout,
			Mass: 40, Armor: 120, CargoCapacity: 80, FuelCapacity: 1600, Initiative: 2, BuiltInScanner: true, CargoSlotSize: VectorFloat64{1, 1},
			Slots: eeeLateSlots(TechHullSlot{Type: HullSlotTypeEngine, Capacity: 1}, TechHullSlot{Type: HullSlotTypeScanner, Capacity: 2}, TechHullSlot{Type: HullSlotTypeElectrical, Capacity: 2}, TechHullSlot{Type: HullSlotTypeMechanical, Capacity: 2}, TechHullSlot{Type: HullSlotTypeShieldArmor, Capacity: 1})},
		{Tech: NewTech("EEE Bulk Carrier", NewCost(55, 0, 24, 145), TechRequirements{TechLevel: TechLevel{Construction: 18, Propulsion: 14}}, 35, TechCategoryShipHull), Type: TechHullTypeFreighter, Class: ShipClassFreighter,
			Mass: 180, Armor: 250, CargoCapacity: 2200, FuelCapacity: 5500, CargoSlotSize: VectorFloat64{2, 2},
			Slots: eeeLateSlots(TechHullSlot{Type: HullSlotTypeEngine, Capacity: 3}, TechHullSlot{Type: HullSlotTypeScannerElectricalMechanical, Capacity: 2}, TechHullSlot{Type: HullSlotTypeShieldArmor, Capacity: 3}, TechHullSlot{Type: HullSlotTypeElectrical, Capacity: 1})},
		{Tech: NewTech("EEE Fleet Tender", NewCost(44, 0, 16, 150), TechRequirements{TechLevel: TechLevel{Construction: 17, Propulsion: 14, Energy: 12}}, 275, TechCategoryShipHull), Type: TechHullTypeFuelTransport, Class: ShipClassFuelTransport,
			Mass: 150, Armor: 160, FuelCapacity: 6500, FuelGeneration: 200, RepairBonus: .1,
			Slots: eeeLateSlots(TechHullSlot{Type: HullSlotTypeEngine, Capacity: 3}, TechHullSlot{Type: HullSlotTypeShield, Capacity: 2}, TechHullSlot{Type: HullSlotTypeScanner, Capacity: 1}, TechHullSlot{Type: HullSlotTypeMechanical, Capacity: 2}, TechHullSlot{Type: HullSlotTypeElectrical, Capacity: 1})},
		{Tech: NewTech("EEE Expedition Cruiser", NewCost(70, 10, 16, 150), TechRequirements{TechLevel: TechLevel{Construction: 18, Propulsion: 16, Electronics: 12}}, 95, TechCategoryShipHull), Type: TechHullTypeFighter, Class: ShipClassWarship,
			Mass: 150, Armor: 1100, FuelCapacity: 3500, Initiative: 5,
			Slots: eeeLateSlots(TechHullSlot{Type: HullSlotTypeEngine, Capacity: 3}, TechHullSlot{Type: HullSlotTypeWeapon, Capacity: 4}, TechHullSlot{Type: HullSlotTypeWeapon, Capacity: 4}, TechHullSlot{Type: HullSlotTypeShieldArmor, Capacity: 4}, TechHullSlot{Type: HullSlotTypeElectrical, Capacity: 2}, TechHullSlot{Type: HullSlotTypeElectrical, Capacity: 2}, TechHullSlot{Type: HullSlotTypeScannerElectricalMechanical, Capacity: 2})},
		{Tech: NewTech("EEE Linebreaker", NewCost(135, 28, 24, 260), TechRequirements{TechLevel: TechLevel{Construction: 23, Propulsion: 20, Electronics: 16}}, 105, TechCategoryShipHull), Type: TechHullTypeCapitalShip, Class: ShipClassWarship,
			Mass: 270, Armor: 2600, FuelCapacity: 5000, Initiative: 8,
			Slots: eeeLateSlots(TechHullSlot{Type: HullSlotTypeEngine, Capacity: 5}, TechHullSlot{Type: HullSlotTypeWeapon, Capacity: 6}, TechHullSlot{Type: HullSlotTypeWeapon, Capacity: 6}, TechHullSlot{Type: HullSlotTypeWeapon, Capacity: 3}, TechHullSlot{Type: HullSlotTypeWeapon, Capacity: 3}, TechHullSlot{Type: HullSlotTypeShield, Capacity: 8}, TechHullSlot{Type: HullSlotTypeArmor, Capacity: 6}, TechHullSlot{Type: HullSlotTypeElectrical, Capacity: 3}, TechHullSlot{Type: HullSlotTypeElectrical, Capacity: 3}, TechHullSlot{Type: HullSlotTypeScannerElectricalMechanical, Capacity: 2})},
		{Tech: NewTech("EEE Expedition Bomber", NewCost(95, 15, 14, 260), TechRequirements{TechLevel: TechLevel{Construction: 21, Propulsion: 18}}, 205, TechCategoryShipHull), Type: TechHullTypeBomber, Class: ShipClassBomber,
			Mass: 140, Armor: 600, FuelCapacity: 3200,
			Slots: eeeLateSlots(TechHullSlot{Type: HullSlotTypeEngine, Capacity: 4}, TechHullSlot{Type: HullSlotTypeBomb, Capacity: 4}, TechHullSlot{Type: HullSlotTypeBomb, Capacity: 4}, TechHullSlot{Type: HullSlotTypeBomb, Capacity: 4}, TechHullSlot{Type: HullSlotTypeShield, Capacity: 2}, TechHullSlot{Type: HullSlotTypeScannerElectricalMechanical, Capacity: 2}, TechHullSlot{Type: HullSlotTypeElectrical, Capacity: 1})},
		{Tech: NewTech("EEE Prospector", NewCost(42, 0, 10, 155), TechRequirements{TechLevel: TechLevel{Construction: 19, Propulsion: 15, Electronics: 12}, LRTsDenied: OBRM}, 245, TechCategoryShipHull), Type: TechHullTypeMiner, Class: ShipClassMiner,
			Mass: 140, Armor: 1400, FuelCapacity: 3500,
			Slots: eeeLateSlots(TechHullSlot{Type: HullSlotTypeEngine, Capacity: 3}, TechHullSlot{Type: HullSlotTypeMining, Capacity: 4}, TechHullSlot{Type: HullSlotTypeMining, Capacity: 4}, TechHullSlot{Type: HullSlotTypeArmorScannerElectricalMechanical, Capacity: 2}, TechHullSlot{Type: HullSlotTypeMechanical, Capacity: 2})},
	}
}
