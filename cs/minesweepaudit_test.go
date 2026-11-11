//go:build !wasi && !wasm

package cs

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinesweeperIgnoresHypotheticalAIChaseButChecksArrival(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f := u.Player(1), u.Fleet("Sweeper")
	u.Player(2).AIControlled = true
	tg := newTurnGenerator(u.Game)
	u.turn = &tg
	enemy := patrolTestEnemy(1000, p)
	enemy.Position = Vector{50, 20}
	enemy.WarpSpeed = 9
	enemy.Heading = VectorFloat64{X: 1}
	p.FleetIntels = []*Fleet{enemy}
	require.Zero(t, bomberChaseThreat(p, f), "planet-targeting AI cannot invent a pursuit")
	u.Run((*turnGenerator).planAutomaticSafety)
	require.False(t, f.InAutomaticSafety())
	delete(p.planetOnlyAI, 2)
	require.Positive(t, bomberChaseThreat(p, f), "human-controlled pursuit remains possible")
	p.planetOnlyAI[2] = true
	enemy.Position = Vector{25, 0}
	enemy.WarpSpeed = 5
	enemy.Heading = VectorFloat64{X: -1}
	require.Positive(t, interceptionThreat(p, f), "actual end-of-turn encounters remain dangerous")
}

func TestTankerRejectsDistantRescueAndFullTank(t *testing.T) {
	u := tankerUniverse(t)
	p, customer, tanker := u.Player(1), u.Fleet("Customer"), u.Fleet("Tanker")
	customer.Fuel = customer.Spec.FuelCapacity
	requestAutomaticRescue(p, customer, true)
	require.Nil(t, readAutomaticFuelRequest(customer))
	customer.Fuel = 0
	customer.Position = Vector{180, 0}
	customer.Waypoints = []Waypoint{{Position: customer.Position}}
	requestAutomaticRescue(p, customer, true)
	u.Run((*turnGenerator).planAutomaticTankers)
	require.Empty(t, customer.GetTag(tankerAwaitTag), "no reservation for a distant tanker")
	job := readTankerJob(tanker)
	if job != nil {
		require.Zero(t, job.TargetNum)
	}
}

func TestMinesweeperReportsActualRemovalAwayFromPlanets(t *testing.T) {
	u := minesweeperUniverse(t)
	p, f := u.Player(1), u.Fleet("Sweeper")
	f.Position = Vector{100, 0}
	f.Waypoints = []Waypoint{{Position: f.Position}}
	field := newMinefield(u.Player(2), MinefieldTypeStandard, 100, 1, f.Position)
	u.Game.Universe.addMinefield(field)
	u.Run((*turnGenerator).fleetSweepMines)
	require.True(t, field.Delete || field.NumMines == 0)
	require.Contains(t, f.GetTag("last-mine-sweep"), "100 mines removed from 1 fields")
	require.Equal(t, 100, p.Messages[len(p.Messages)-1].Spec.Amount)
	u.Run((*turnGenerator).fleetSweepMines)
	require.Contains(t, f.GetTag("last-mine-sweep"), "0 mines removed")
}

func TestArchivedMineSwapResumesPurposefulSweeping(t *testing.T) {
	path := os.Getenv("STARS_SWEEP_ARCHIVE")
	if path == "" {
		t.Skip("optional read-only archived-turn diagnostic")
	}
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	r, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer r.Close()
	var archive struct{ Game FullGame }
	require.NoError(t, json.NewDecoder(r).Decode(&archive))
	g := &archive.Game
	g.TechStore = &StaticTechStore
	g.Rules.SetTechStore(&StaticTechStore)
	g.Rules.ResetSeed(1)
	g.Universe.setLogger(testLogger)
	require.NoError(t, g.computeSpecs())
	tg := newTurnGenerator(g)
	tg.log = testLogger
	tg.enablePlanningThreatIndex()
	tg.refreshAutomaticFuelIntel()
	tg.resumeCompletedEvades()
	tg.planMinesweeperAutomation()
	tg.planAutomaticSafety()
	for _, num := range []int{298, 988, 1072} {
		f := g.getFleet(1, num)
		t.Logf("fleet%d safety=%s sweep=%s position=%v route=%v", num, f.GetTag(safetyTag), f.GetTag(patrolMineSweepTag), f.Position, f.Waypoints)
		require.False(t, f.InAutomaticSafety(), "hypothetical AI chase must not cancel fleet %d mission", num)
		require.NotEmpty(t, f.GetTag(patrolMineSweepTag))
		require.Greater(t, len(f.Waypoints), 1)
	}
	f := g.getFleet(1, 1072)
	before := f.Position
	g.getPlayer(1).Messages = nil
	tg.fleetMove()
	tg.fleetSweepMines()
	require.NotEqual(t, before, f.Position, "the sweeper must actively enter the field")
	removed := 0
	for _, m := range g.getPlayer(1).Messages {
		if m.Type == PlayerMessageFleetMinefieldSweptMines && m.TargetNum == f.Num {
			removed += m.Spec.Amount
		}
	}
	require.Positive(t, removed)
	for _, planet := range g.Planets {
		require.NotEqual(t, planet.Position, f.Position, "sweeping must work away from every planet")
	}
	t.Logf("Fleet1072 actively moved %v -> %v and removed %d mines away from planets", before, f.Position, removed)
}
