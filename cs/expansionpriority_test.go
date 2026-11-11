//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func setExpansionPotential(t *testing.T, p *Player, planet *Planet, low, high int) {
	t.Helper()
	for grav := 0; grav <= 100; grav++ {
		for temp := 0; temp <= 100; temp++ {
			planet.Hab = Hab{grav, temp, 50}
			planet.BaseHab = planet.Hab
			q := terraformedPlanetQuality(p, planet)
			if q >= low && q <= high {
				return
			}
		}
	}
	t.Fatalf("no test environment within potential %d..%d", low, high)
}

func TestExpansionPriorityStrictThresholdAndManualOverride(t *testing.T) {
	u := colonizerUniverse(t)
	p := u.Player(1)
	planet := u.Planet("Far")
	setExpansionPotential(t, p, planet, 75, 75)
	require.Equal(t, 0, expansionPriority(p, planet))
	setExpansionPotential(t, p, planet, 76, 80)
	require.Equal(t, 1, expansionPriority(p, planet))
	p.Automation.PriorityPlanets = map[int]bool{planet.Num: true}
	require.Equal(t, 2, expansionPriority(p, planet))
}

func TestHighPotentialColoniesOutrankOrdinaryModeScores(t *testing.T) {
	for _, mode := range []string{"nearest", "auto", "best", "farthest"} {
		t.Run(mode, func(t *testing.T) {
			u := colonizerUniverse(t)
			p, f := u.Player(1), u.Fleet("Colony Ship")
			p.Automation.FleetRoles[f.Num] = "colonizer:" + mode
			near, far := u.Planet("Near"), u.Planet("Far")
			setExpansionPotential(t, p, near, 60, 70)
			setExpansionPotential(t, p, far, 76, 80)
			u.Run((*turnGenerator).planColonizationAutomation)
			require.Equal(t, far.Num, readColonizationJob(f).TargetNum)
		})
	}
}

func TestManualColonyPriorityOutranksHighPotential(t *testing.T) {
	u := colonizerUniverse(t)
	p, f := u.Player(1), u.Fleet("Colony Ship")
	p.Automation.FleetRoles[f.Num] = "colonizer:best"
	near, far := u.Planet("Near"), u.Planet("Far")
	setExpansionPotential(t, p, near, 30, 40)
	setExpansionPotential(t, p, far, 90, 100)
	p.Automation.PriorityPlanets = map[int]bool{near.Num: true}
	u.Run((*turnGenerator).planColonizationAutomation)
	require.Equal(t, near.Num, readColonizationJob(f).TargetNum)
}

func TestEnemyHighPotentialWorldRemainsLandingCandidate(t *testing.T) {
	u := colonizerUniverse(t)
	p := u.Player(1)
	planet := u.Planet("Far")
	setExpansionPotential(t, p, planet, 76, 80)
	planet.PlayerNum = 2
	require.True(t, invasionCandidate(p, planet))
	require.Equal(t, 1, expansionPriority(p, planet))
	planet.PlayerNum = p.Num
	require.False(t, invasionCandidate(p, planet))
}

func TestLandingStartsWithHighPotentialUnlessPlayerMarksAnotherWorld(t *testing.T) {
	for _, manual := range []bool{false, true} {
		u := landingOperationUniverse(t)
		p := u.Player(1)
		high := p.GetPlanetIntel(u.Planet("Target").Num)
		setExpansionPotential(t, p, high, 76, 85)
		near := *high
		near.Num = 999
		near.Name = "Closer ordinary world"
		near.Position = Vector{40, 0}
		setExpansionPotential(t, p, &near, 55, 65)
		p.PlanetIntels = append(p.PlanetIntels, &near)
		if manual {
			p.Automation.PriorityPlanets = map[int]bool{near.Num: true}
		}
		u.Run(func(tg *turnGenerator) {
			leader := tg.startLandingOperation(p, map[int]bool{})
			require.NotNil(t, leader)
			want := high.Num
			if manual {
				want = near.Num
			}
			require.Equal(t, want, readLandingOperation(leader).Target)
		})
	}
}
