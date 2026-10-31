//go:build !wasi && !wasm

package cs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"log/slog"
	"math"
	"testing"
)

func TestEEECostsIncludeAllIFEARMURCombinations(t *testing.T) {
	for combination := range 8 {
		r := NewRace()
		if combination&1 != 0 {
			r.WithLRT(IFE)
		}
		if combination&2 != 0 {
			r.WithLRT(ARM)
		}
		if combination&4 != 0 {
			r.WithLRT(UR)
		}
		before := r.ComputeRacePoints(10000)
		r.EEETraits = EEEEfficientDrives | EEEExtractionSpecialists | EEEMaterialRecovery
		want := 100
		if combination&1 != 0 {
			want += 10
		}
		if combination&2 != 0 {
			want += 15
		}
		if combination&4 != 0 {
			want += 10
		}
		require.Equal(t, want, r.EEECost())
		require.Equal(t, want, before-r.ComputeRacePoints(10000))
	}
	for i, want := range []int{35, 25, 40, 45, 20, 30, 45, 70, 30, 55, 45, 45, -25, -35, -40, -20} {
		r := NewRace()
		before := r.ComputeRacePoints(10000)
		r.EEETraits = 1 << i
		if r.HasEEE(EEEResearchSpecialization) {
			r.EEEResearchField = Energy
		}
		require.NoError(t, r.Validate())
		require.Equal(t, want, before-r.ComputeRacePoints(10000))
	}
	r := NewRace().WithLRT(IFE)
	r.EEETraits = EEEInefficientPropulsion
	require.Equal(t, -15, r.EEECost())
}

func TestEEERejectsOppositesAndInvalidSpecializations(t *testing.T) {
	for _, pair := range eeeOpposites {
		r := NewRace()
		r.EEETraits = pair[0] | pair[1]
		require.Error(t, r.Validate())
	}
	for _, mask := range []int{-1, 1 << 16} {
		r := NewRace()
		r.EEETraits = mask
		require.Error(t, r.Validate())
	}
	r := NewRace()
	r.EEETraits = EEEResearchSpecialization
	require.Error(t, r.Validate())
	r.EEEResearchField = TechField("bogus")
	require.Error(t, r.Validate())
	r.EEEResearchField = Weapons
	require.NoError(t, r.Validate())
	r.EEETraits = 0
	require.Error(t, r.Validate())
}

func TestEEEEngineScannerAndMineEffects(t *testing.T) {
	r := NewRace().WithLRT(IFE)
	base := ComputeRaceSpec(r, &rules)
	r.EEETraits = EEEEfficientDrives | EEESurveyNetworks | EEEMineNavigation
	spec := ComputeRaceSpec(r, &rules)
	require.InDelta(t, (1+base.FuelEfficiencyOffset)*0.9, 1+spec.FuelEfficiencyOffset, 1e-9)
	require.InDelta(t, base.ScanRangeFactor*1.15, spec.ScanRangeFactor, 1e-9)
	require.Equal(t, base.MinefieldSafeWarpBonus+1, spec.MinefieldSafeWarpBonus)
	r.EEETraits = EEEInefficientPropulsion
	require.InDelta(t, (1+base.FuelEfficiencyOffset)*1.15, 1+ComputeRaceSpec(r, &rules).FuelEfficiencyOffset, 1e-9)
}

func TestEEEDesignEconomicsAndCargo(t *testing.T) {
	// A binary floating-point 1.1 multiplier must not turn an exact 110
	// resource price into 111 when the normal final ceiling is applied.
	exact := eeeResourceCost(CostFloat64{Resources: 100}, RaceSpec{EEETraits: EEEExpensiveShipbuilding}, false).Round(math.Ceil).ToCost()
	require.Equal(t, 110, exact.Resources)
	r := NewRace()
	design := DesignTeamster
	base, err := ComputeShipDesignSpec(&rules, TechLevel{}, ComputeRaceSpec(r, &rules), &design)
	require.NoError(t, err)
	r.EEETraits = EEEExtendedTanks | EEECargoEngineering | EEELightweightHulls | EEEOrbitalAssembly
	upgraded, err := ComputeShipDesignSpec(&rules, TechLevel{}, ComputeRaceSpec(r, &rules), &design)
	require.NoError(t, err)
	require.Equal(t, base.FuelCapacity*12/10, upgraded.FuelCapacity)
	require.Equal(t, base.CargoCapacity/5, upgraded.MineralCargoBonus)
	require.Equal(t, base.CargoCapacity+upgraded.MineralCargoBonus, upgraded.CargoCapacity)
	require.Less(t, upgraded.Armor, base.Armor)
	require.Less(t, upgraded.Cost.Resources, base.Cost.Resources)
	require.Equal(t, base.Cost.ToMineral(), upgraded.Cost.ToMineral())
	fleet := &Fleet{Spec: FleetSpec{ShipDesignSpec: upgraded}}
	require.Equal(t, base.CargoCapacity, fleet.availableColonistSpace())
	fleet.Cargo.Colonists = base.CargoCapacity
	require.Zero(t, fleet.availableColonistSpace())
	require.Equal(t, upgraded.MineralCargoBonus, fleet.availableCargoSpace())
	r.EEETraits = EEEExpensiveShipbuilding
	expensive, err := ComputeShipDesignSpec(&rules, TechLevel{}, ComputeRaceSpec(r, &rules), &design)
	require.NoError(t, err)
	require.Greater(t, expensive.Cost.Resources, base.Cost.Resources)
	r.EEETraits = EEEStationEngineers
	station := DesignSpaceStation
	standard, err := ComputeShipDesignSpec(&rules, TechLevel{}, ComputeRaceSpec(NewRace(), &rules), &station)
	require.NoError(t, err)
	discount, err := ComputeShipDesignSpec(&rules, TechLevel{}, ComputeRaceSpec(r, &rules), &station)
	require.NoError(t, err)
	require.Less(t, discount.Cost.Resources, standard.Cost.Resources)
	require.Equal(t, standard.Cost.ToMineral(), discount.Cost.ToMineral())
}

func TestEEEMineralSpaceCannotBeUsedForPassengers(t *testing.T) {
	fleet := &Fleet{Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{CargoCapacity: 120, MineralCargoBonus: 20}}}
	planet := NewPlanet().WithCargo(Cargo{Colonists: 1000, Ironium: 1000})
	transfer := cargoTransferer{}
	people, _, _ := transfer.getCargoLoadAmount(fleet, planet, Colonists, WaypointTransportTask{Action: TransportActionLoadAll})
	minerals, _, _ := transfer.getCargoLoadAmount(fleet, planet, Ironium, WaypointTransportTask{Action: TransportActionLoadAll})
	require.Equal(t, 100, people)
	require.Equal(t, 120, minerals)
	a := &Fleet{Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{CargoCapacity: 120, MineralCargoBonus: 20}}, Cargo: Cargo{Colonists: 102, Ironium: 18}}
	b := &Fleet{Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{CargoCapacity: 20}}, Cargo: Cargo{Colonists: 18, Ironium: 2}}
	before := a.Cargo.Add(b.Cargo)
	rebalanceEEEColonists(a, b)
	require.Equal(t, before, a.Cargo.Add(b.Cargo))
	require.Equal(t, 100, a.Cargo.Colonists)
	require.Equal(t, 20, b.Cargo.Colonists)
	require.LessOrEqual(t, a.Cargo.Total(), a.Spec.CargoCapacity)
	require.LessOrEqual(t, b.Cargo.Total(), b.Spec.CargoCapacity)
}

func TestEEERepairsChangeActualDamage(t *testing.T) {
	for _, tc := range []struct {
		trait    int
		factor   float64
		workshop bool
	}{{EEEMaintenanceCrews, 1.25, false}, {EEESlowMaintenance, 0.75, false}, {EEEFieldWorkshops, 1, true}} {
		t.Run(fmt.Sprint(tc.trait), func(t *testing.T) {
			u := logisticsUniverse(t)
			p, fleet := u.Player(1), u.Fleet("Escort")
			p.Race.EEETraits = tc.trait
			p.Race.Spec = ComputeRaceSpec(&p.Race, &rules)
			fleet.Tokens[0].QuantityDamaged = 1
			fleet.Tokens[0].Damage = 100
			r := rules
			r.RepairRates = map[RepairRate]float64{RepairRateStopped: 0.04}
			rate := (0.04 + fleet.Spec.RepairBonus) * tc.factor
			if tc.workshop {
				rate += 0.01
			}
			want := math.Floor(max(0, 100-float64(max(1, int(float64(fleet.Tokens[0].design.Spec.Armor)*rate)))))
			fleet.repairFleet(slog.Default(), &r, p, nil)
			require.Equal(t, want, fleet.Tokens[0].Damage)
		})
	}
}

func TestEEERecyclingWithAndWithoutUR(t *testing.T) {
	for _, ur := range []bool{false, true} {
		u := logisticsUniverse(t)
		p, fleet, yard := u.Player(1), u.Fleet("Escort"), u.Planet("Home")
		if ur {
			p.Race.WithLRT(UR)
		}
		p.Race.Spec = ComputeRaceSpec(&p.Race, &rules)
		fleet.Spec.Cost = Cost{Ironium: 1000, Boranium: 1000, Germanium: 1000, Resources: 1000}
		before := fleet.getScrapAmount(&rules, p, yard, false)
		p.Race.EEETraits = EEEMaterialRecovery
		after := fleet.getScrapAmount(&rules, p, yard, false)
		require.Equal(t, before.Ironium+50, after.Ironium)
		require.Equal(t, before.Resources, after.Resources)
		require.LessOrEqual(t, after.Ironium, 950)
	}
}

func TestEEEExtractionAndResearchHaveGameplayEffects(t *testing.T) {
	u := newTestUniverse(t, remoteMiningScenario("Mine", 0, DesignPotatoBug, WaypointTaskRemoteMining, JoaT))
	p, mine := u.Player(1), u.Planet("Mine")
	p.Race.EEETraits = EEEExtractionSpecialists
	fleet := u.Game.Fleets[0]
	output := mine.getMineralOutput(&u.Game.Rules, fleet.Spec.MiningRate, u.Game.Rules.RemoteMiningMineOutput)
	before := mine.Cargo
	u.Run((*turnGenerator).fleetRemoteMine)
	require.Equal(t, output.Ironium*5/4, mine.Cargo.Ironium-before.Ironium)
	base := nextLevelCost(&rules, p, Energy)
	p.Race.EEETraits = EEEResearchSpecialization
	p.Race.EEEResearchField = Energy
	cost := nextLevelCost(&rules, p, Energy)
	require.Less(t, cost, base)
	require.Equal(t, (base*9+9)/10, cost)
	r := newResearcher(&rules)
	level := p.TechLevels.Energy
	gained, _ := r.researchFieldOnce(p, Energy, cost)
	require.True(t, gained)
	require.Equal(t, level+1, p.TechLevels.Energy)
}
