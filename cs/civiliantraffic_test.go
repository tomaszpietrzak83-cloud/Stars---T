//go:build !wasi && !wasm

package cs

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCivilianTrafficReleasesEscortAndDeliversSalvage(t *testing.T) {
	u := logisticsUniverse(t)
	p, transport, escort := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	enemy := patrolTestEnemy(1000)
	enemy.Position = Vector{55, 6}
	enemy.WarpSpeed = 6
	enemy.Heading = VectorFloat64{X: 1}
	p.FleetIntels = []*Fleet{enemy}
	p.Automation.FleetRoles[escort.Num] = "escort"
	escort.SetTag(escortTag, strconv.Itoa(transport.Num))
	u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).enforceAutomaticDeparture)
	require.GreaterOrEqual(t, len(transport.Waypoints), 2)
	require.Empty(t, escort.GetTag(escortTag), "a receding report must release the escort before departure")
	for range 6 {
		u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).fleetRefuel, (*turnGenerator).resolveLogisticsAutomation)
		if u.Planet("Home").Cargo.Ironium == 100 {
			break
		}
	}
	require.Equal(t, 100, u.Planet("Home").Cargo.Ironium)
}

func TestCivilianTrafficKeepsMandatoryEscortAndRechecksCourse(t *testing.T) {
	u := logisticsUniverse(t)
	p, transport := u.Player(1), u.Fleet("Transport")
	enemy := patrolTestEnemy(1000)
	enemy.Position = Vector{55, 6}
	enemy.WarpSpeed = 6
	enemy.Heading = VectorFloat64{X: 1}
	p.FleetIntels = []*Fleet{enemy}
	destination := Vector{50, 0}
	job := &logisticsJob{Kind: "return", Position: destination}
	u.Run(func(tg *turnGenerator) {
		tg.prepareConvoyWithMinimum(p, transport, job, MapObjectTarget{}, 1)
	})
	require.Len(t, transport.Waypoints, 1)
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.GreaterOrEqual(t, len(transport.Waypoints), 2)
	enemy.Heading = VectorFloat64{X: -1}
	u.Run((*turnGenerator).enforceAutomaticDeparture)
	require.Len(t, transport.Waypoints, 1)
}

func TestScoutTrafficAvoidsConvergingCourseAndMines(t *testing.T) {
	u := newTestUniverse(t, TestScenario{
		Players: []ScenarioPlayer{{Designs: Designs(DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Scout", Design: DesignLongRangeScout.Name, At: "Home"}}}},
		Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Survey", Position: Vector{36, 0}}},
	})
	p, scout, survey := u.Player(1), u.Fleet("Scout"), u.Planet("Survey")
	p.Automation.FleetRoles = map[int]string{scout.Num: "scout"}
	enemy := patrolTestEnemy(1000)
	enemy.Position = Vector{43, 6}
	enemy.WarpSpeed = 6
	enemy.Heading = VectorFloat64{X: 1}
	p.FleetIntels = []*Fleet{enemy}
	require.NotNil(t, scoutEconomicalLeg(&u.Game.Rules, p, scout, survey))
	p.MinefieldIntels = []*Minefield{{MapObject: MapObject{PlayerNum: 2, Position: survey.Position}, MinefieldType: MinefieldTypeStandard, NumMines: 100, Detonate: true}}
	require.Nil(t, scoutEconomicalLeg(&u.Game.Rules, p, scout, survey))
	p.MinefieldIntels = nil
	enemy.Heading = VectorFloat64{X: -1}
	require.Nil(t, scoutEconomicalLeg(&u.Game.Rules, p, scout, survey))
}

func TestCivilianTrafficCountsReportedArmedScout(t *testing.T) {
	u := logisticsUniverse(t)
	p, transport := u.Player(1), u.Fleet("Transport")
	report := patrolTestEnemy(0, p)
	p.FleetIntels = []*Fleet{report}
	destination := Vector{50, 0}
	require.Zero(t, transitRouteThreat(p, transport, transport.Position, destination))
	p.ShipDesignIntels[len(p.ShipDesignIntels)-1].Spec.HasWeapons = true
	require.Positive(t, transitRouteThreat(p, transport, transport.Position, destination))
}
