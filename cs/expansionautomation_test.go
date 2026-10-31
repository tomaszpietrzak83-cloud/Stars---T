//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestInstallationExportRetainsNinetyFivePercentOfOperatingCapacity(t *testing.T) {
	for _, tc := range []struct{ stock, capacity, want int }{{95, 100, 0}, {100, 100, 5}, {110, 100, 15}, {94, 100, 0}, {1, 0, 0}, {96, 101, 0}, {97, 101, 1}} {
		require.Equal(t, tc.want, installationSurplus(tc.stock, tc.capacity))
	}
}
func TestOpportunisticInstallationsConserveBuildingsAndUseRealCargoMass(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	source := u.Planet("Home")
	source.Cargo.Colonists = 1000
	p.Race.NumFactories = 10
	p.Race.NumMines = 10
	source.Spec.MaxPopulation = 1000000
	source.Factories = 110
	source.Mines = 95
	dest := *source
	dest.Num = 2
	dest.Position = Vector{10, 0}
	dest.Cargo.Colonists = 200
	dest.Factories = 0
	dest.Mines = 0
	u.Game.Planets = append(u.Game.Planets, &dest)
	p.Automation.InfrastructureTransport = true
	f.Spec.CargoCapacity = 400
	beforeMass := f.CargoMass()
	u.Run(func(tg *turnGenerator) { tg.loadOpportunisticInfrastructure(p, f, source, &dest, 0) })
	cargo := readInfrastructure(f)
	require.Equal(t, 15, cargo.Factories)
	require.Zero(t, cargo.Mines)
	require.Equal(t, 95, source.Factories)
	require.Equal(t, beforeMass+75, f.CargoMass())
	require.Equal(t, 325, f.GetCargoCapacity())
	f.Position = dest.Position
	u.Run((*turnGenerator).unloadOpportunisticInfrastructure)
	require.Equal(t, 15, dest.Factories)
	require.Equal(t, 110, source.Factories+dest.Factories)
	require.False(t, f.carriesInfrastructure())
	require.Equal(t, beforeMass, f.CargoMass())
}
func TestFutureTerraformingWorldsAreExplicitOptInExceptClaimAdjuster(t *testing.T) {
	p := NewPlayer(1, NewRace().WithSpec(&rules)).WithTechLevels(TechLevel{10, 10, 10, 10, 10, 10}).withSpec(&rules)
	var world *Planet
	for grav := 0; grav <= 100; grav++ {
		planet := &Planet{MapObject: MapObject{Num: 1}, Hab: Hab{grav, 50, 50}, BaseHab: Hab{grav, 50, 50}}
		if p.Race.GetPlanetHabitability(planet.Hab) >= 0 {
			continue
		}
		p.Automation.ServeTerraformingWorlds = true
		if prospectivePlanetQuality(p, planet) > 0 {
			world = planet
			break
		}
	}
	require.NotNil(t, world)
	require.Positive(t, colonyPlanetQuality(p, world))
	p.Automation.ServeTerraformingWorlds = false
	require.Zero(t, colonyPlanetQuality(p, world))
	p.Race.Spec.Instaforming = true
	require.Positive(t, colonyPlanetQuality(p, world))
}
func TestExplicitStartLocationsWorkWithoutOpponents(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want int
	}{{"center", 1}, {"edge", 0}} {
		ug := newPlacementGenerator(PlayerPositionsModerate, 1, Vector{1000, 1000}, newIntRandom(0, 1), Vector{30, 500}, Vector{500, 500}, Vector{800, 500})
		ug.HostStartPosition = tc.mode
		positions, err := ug.getPlayerPlanets()
		require.NoError(t, err)
		require.Equal(t, tc.want, positions[0])
	}
}
func TestPackedInstallationsCannotHideCargoAfterHoldDamage(t *testing.T) {
	u := logisticsUniverse(t)
	f := u.Fleet("Transport")
	saveInfrastructure(f, infrastructureCargo{Factories: 15, Origin: 1, Destination: 2})
	f.Spec.CargoCapacity = 80
	f.Cargo = Cargo{Ironium: 80}
	f.reduceCargoToMax()
	require.Equal(t, 80, f.CargoMass())
	require.Equal(t, 15, readInfrastructure(f).Factories)
	f.Spec.CargoCapacity = 20
	f.reduceCargoToMax()
	require.LessOrEqual(t, f.CargoMass(), 20)
	f.Spec.CargoCapacity = 0
	f.reduceCargoToMax()
	require.Zero(t, f.CargoMass())
	require.False(t, f.carriesInfrastructure())
}
