//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBombingForecastStagesWithoutEarlyLandingOrHiddenData(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b, target := u.Player(1), u.Fleet("Bomber"), u.Planet("Enemy")
	intel := *target
	intel.Cargo.Colonists = 2000
	intel.ReportAge = 0
	p.PlanetIntels = []*Planet{u.Planet("Home"), &intel}
	b.Position = target.Position
	b.Spec.Bombs = []Bomb{{Quantity: 1, KillRate: 90}}
	forecast := u.turn.forecastLandingAfterBombing(p, &intel)
	require.NotNil(t, forecast)
	require.Less(t, forecast.Cargo.Colonists, 1000)
	require.Equal(t, 2000, intel.Cargo.Colonists, "forecast must not alter the observed population")
	job := &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num, BombingStage: true}
	f := u.Fleet("Cover")
	p.Automation.FleetRoles[f.Num] = "freighter"
	f.Cargo.Colonists = 100
	u.turn.invasionDestination(p, f, job, &intel)
	require.True(t, job.BombingStage)
	require.NotEqual(t, target.Position, job.Position)
	f.Position = job.Position
	saveLogisticsWork(f, job)
	target.Cargo.Colonists = 1 // The unseen real population must not unlock a forecast landing.
	u.Run((*turnGenerator).dispatchReadyLogistics, (*turnGenerator).resolveAutomaticInvasions)
	require.True(t, logisticsWork(f).BombingStage)
	require.Equal(t, 100, f.Cargo.Colonists)
	intel.Cargo.Colonists = 200
	u.turn.invasionDestination(p, f, job, &intel)
	require.False(t, job.BombingStage)
	require.Equal(t, target.Position, job.Position)
	intel.ReportAge = 42
	require.Nil(t, u.turn.forecastLandingAfterBombing(p, &intel), "Covenant's old report needs scout reconnaissance first")
}
