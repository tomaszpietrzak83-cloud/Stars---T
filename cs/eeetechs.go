package cs

// EEE components use multi-field requirements and prices comparable to their
// ordinary counterparts. They use the normal research and design systems.
func EEETechHullComponents() []TechHullComponent {
	return []TechHullComponent{
		{Tech: NewTech("EEE Survey Drive", NewCost(6, 0, 5, 16), TechRequirements{TechLevel: TechLevel{Propulsion: 6, Electronics: 4, Construction: 3}, LRTsDenied: NRSE}, 68, TechCategoryEngine, TechTagEngine, TechTagRamscoop), Mass: 8, HullSlotType: HullSlotTypeEngine, Engine: Engine{IdealSpeed: 6, FreeSpeed: 4, MaxSafeSpeed: 8, FuelUsage: []int{0, 0, 0, 0, 0, 25, 75, 160, 260, 550, 1000}}},
		{Tech: NewTech("EEE Courier Drive", NewCost(14, 0, 8, 32), TechRequirements{TechLevel: TechLevel{Propulsion: 13, Energy: 10, Construction: 9}}, 85, TechCategoryEngine, TechTagEngine), Mass: 12, HullSlotType: HullSlotTypeEngine, Engine: Engine{IdealSpeed: 8, MaxSafeSpeed: 9, FuelUsage: []int{0, 5, 5, 5, 8, 12, 20, 35, 65, 130, 450}}},
		{Tech: NewTech("EEE Cryogenic Tank", NewCost(10, 0, 2, 12), TechRequirements{TechLevel: TechLevel{Energy: 8, Propulsion: 6, Construction: 9}}, 55, TechCategoryMechanical, TechTagFuelTank), Mass: 12, FuelBonus: 650, HullSlotType: HullSlotTypeMechanical},
		{Tech: NewTech("EEE Deep Reservoir", NewCost(16, 0, 4, 18), TechRequirements{TechLevel: TechLevel{Energy: 15, Propulsion: 12, Construction: 17}}, 60, TechCategoryMechanical, TechTagFuelTank), Mass: 12, FuelBonus: 1000, HullSlotType: HullSlotTypeMechanical},
		{Tech: NewTech("EEE Modular Cargo", NewCost(10, 0, 3, 22), TechRequirements{TechLevel: TechLevel{Energy: 5, Construction: 7}}, 35, TechCategoryMechanical, TechTagCargoPod), Mass: 9, CargoBonus: 160, HullSlotType: HullSlotTypeMechanical},
		{Tech: NewTech("EEE Gravity Cargo", NewCost(16, 0, 6, 34), TechRequirements{TechLevel: TechLevel{Energy: 12, Construction: 15, Propulsion: 8}}, 40, TechCategoryMechanical, TechTagCargoPod), Mass: 12, CargoBonus: 320, HullSlotType: HullSlotTypeMechanical},
		{Tech: NewTech("EEE Adaptive Miner", NewCost(30, 0, 10, 100), TechRequirements{TechLevel: TechLevel{Construction: 10, Electronics: 8}, LRTsDenied: OBRM}, 45, TechCategoryMineRobot, TechTagMiningRobot), Mass: 200, MiningRate: 24, HullSlotType: HullSlotTypeMining},
		{Tech: NewTech("EEE Bio Miner", NewCost(34, 0, 12, 120), TechRequirements{TechLevel: TechLevel{Construction: 17, Electronics: 14, Biotechnology: 12}, LRTsDenied: OBRM}, 65, TechCategoryMineRobot, TechTagMiningRobot), Mass: 180, MiningRate: 42, HullSlotType: HullSlotTypeMining},
		{Tech: NewTech("EEE Deep Survey", NewCost(4, 0, 12, 45), TechRequirements{TechLevel: TechLevel{Electronics: 12, Biotechnology: 6}, LRTsDenied: NAS}, 145, TechCategoryScanner, TechTagScanner), Mass: 5, Scanner: true, ScanRange: 320, ScanRangePen: 100, HullSlotType: HullSlotTypeScanner},
		{Tech: NewTech("EEE Quantum Survey", NewCost(6, 0, 24, 85), TechRequirements{TechLevel: TechLevel{Electronics: 20, Biotechnology: 14}, LRTsDenied: NAS}, 155, TechCategoryScanner, TechTagScanner), Mass: 8, Scanner: true, ScanRange: 600, ScanRangePen: 250, HullSlotType: HullSlotTypeScanner},
		{Tech: NewTech("EEE Ceramic Armor", NewCost(10, 0, 4, 26), TechRequirements{TechLevel: TechLevel{Construction: 10, Biotechnology: 8}}, 55, TechCategoryArmor, TechTagArmor), Mass: 24, Armor: 220, HullSlotType: HullSlotTypeArmor},
		{Tech: NewTech("EEE Phase Barrier", NewCost(8, 0, 12, 19), TechRequirements{TechLevel: TechLevel{Energy: 18, Electronics: 12}}, 85, TechCategoryShield, TechTagShield), Mass: 2, Shield: 350, HullSlotType: HullSlotTypeShield},
		{Tech: NewTech("EEE Pulse Lance", NewCost(0, 16, 3, 20), TechRequirements{TechLevel: TechLevel{Weapons: 13, Energy: 10}}, 85, TechCategoryBeamWeapon, TechTagBeamWeapon), Mass: 14, Power: 95, Range: 3, Initiative: 4, HullSlotType: HullSlotTypeWeapon},
		{Tech: NewTech("EEE Needle Torpedo", NewCost(18, 16, 12, 16), TechRequirements{TechLevel: TechLevel{Weapons: 16, Propulsion: 10, Electronics: 8}}, 35, TechCategoryTorpedo, TechTagTorpedo), Mass: 16, Power: 130, Range: 5, Initiative: 2, Accuracy: 75, HullSlotType: HullSlotTypeWeapon},
	}
}
