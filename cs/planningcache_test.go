//go:build !wasi && !wasm

package cs

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaintenanceDestinationMatchesDecoderAndRefreshes(t *testing.T) {
	f := &Fleet{}
	for _, raw := range []string{
		"", "broken", "null", `{}`, `{"stage":"scrap","planetNum":3}`,
		`{"stage":"scrap","planetNum":7,"originalOrders":{"waypoints":[]}}`,
		`{"stage":"scrap","planetNum":7,"originalOrders":{"waypoints":"invalid"}}`,
		`{"stage":"travel","planetNum":9}`, `{"planetNum":"invalid"}`, "",
	} {
		f.SetTag(maintenanceTag, raw)
		want := decodeMaintenance(raw)
		for range 2 {
			stage, planet, valid := maintenanceDestination(f)
			require.Equal(t, want != nil, valid)
			if want != nil {
				require.Equal(t, want.Stage, stage)
				require.Equal(t, want.PlanetNum, planet)
			}
		}
	}
	f.SetTag(maintenanceTag, `{"stage":"scrap","planetNum":3}`)
	_, _, _ = maintenanceDestination(f)
	copy := *f
	copy.Tags = maps.Clone(f.Tags)
	copy.SetTag(maintenanceTag, `{"stage":"travel","planetNum":9}`)
	stage, planet, valid := maintenanceDestination(&copy)
	require.True(t, valid)
	require.Equal(t, "travel", stage)
	require.Equal(t, 9, planet)
	stage, planet, valid = maintenanceDestination(f)
	require.True(t, valid)
	require.Equal(t, "scrap", stage)
	require.Equal(t, 3, planet)
}

func TestPatrolReconBatchMatchesReportsAndDoesNotSurviveBatch(t *testing.T) {
	p := &Player{Num: 1}
	for i := range 150 {
		f := &Fleet{MapObject: MapObject{Num: i + 1, PlayerNum: 2, Position: Vector{X: i * 15, Y: i % 3}, ReportAge: i % 3}}
		f.Tokens = []ShipToken{{DesignNum: 999, Quantity: 1 + i%9}}
		f.Heading, f.WarpSpeed = VectorFloat64{X: float64(i%3) - 1}, 6
		p.FleetIntels = append(p.FleetIntels, f)
	}
	check := func() {
		cached := patrolReconSnapshot(p)
		for _, f := range p.FleetIntels {
			want := patrolNeedsRecon(p, f)
			require.Equal(t, want, cached(f))
			require.Equal(t, want, cached(f))
		}
		require.True(t, cached(nil))
	}
	check()
	for i, f := range p.FleetIntels {
		f.Position = Vector{X: i * 300}
		f.Heading = VectorFloat64{}
		f.Tokens[0].Quantity = 1
	}
	check()
	// A fresh snapshot must see a changed verdict for the same report pointer.
	f := p.FleetIntels[0]
	require.False(t, patrolReconSnapshot(p)(f))
	f.Tokens[0].Quantity = 8
	require.True(t, patrolReconSnapshot(p)(f))
}

func referenceFleetFuelUsage(fleet *Fleet, player *Player, warpSpeed int, distance float64) float64 {
	ifeFactor := 1 + player.Race.Spec.FuelEfficiencyOffset

	// most efficient engines first, keeping token order for ties
	tokens := make([]ShipToken, len(fleet.Tokens))
	copy(tokens, fleet.Tokens)
	slices.SortStableFunc(tokens, func(a, b ShipToken) int {
		return a.design.Spec.Engine.fuelUsageAt(warpSpeed) - b.design.Spec.Engine.fuelUsageAt(warpSpeed)
	})

	cargo := fleet.CargoMass()
	fuel := 0.0
	for _, token := range tokens {
		tokenCargo := min(cargo, token.design.Spec.CargoCapacity*token.Quantity)
		cargo -= tokenCargo

		mass := token.design.Spec.Mass*token.Quantity + tokenCargo
		fuel += token.design.Spec.Engine.fuelUsage(warpSpeed, mass, distance, ifeFactor)
	}

	return fuel
}

func TestSingleDesignFuelUsageMatchesSortedReference(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	for _, name := range []string{"Escort", "Transport", "Combat"} {
		f := u.Fleet(name)
		for _, cargo := range []Cargo{{}, {Ironium: 10}, {Colonists: 1000}} {
			f.Cargo = cargo
			for warp := 0; warp <= 10; warp++ {
				for _, distance := range []float64{0, 0.1, 16, 123.4} {
					require.Equal(t, referenceFleetFuelUsage(f, p, warp, distance), f.fuelUsage(p, warp, distance))
				}
			}
		}
	}
	f := *u.Fleet("Escort")
	f.Tokens = nil
	require.Equal(t, referenceFleetFuelUsage(&f, p, 5, 100), f.fuelUsage(p, 5, 100))
}
