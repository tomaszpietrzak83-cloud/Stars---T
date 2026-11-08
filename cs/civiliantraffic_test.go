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
	enemy.Position = Vector{12, 0}
	p.FleetIntels = []*Fleet{enemy}
	p.Automation.FleetRoles[escort.Num] = "escort"
	escort.SetTag(escortTag, strconv.Itoa(transport.Num))
	u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).enforceAutomaticDeparture)
	require.GreaterOrEqual(t, len(transport.Waypoints), 2)
	require.Empty(t, escort.GetTag(escortTag), "a stationary contact passed in transit must release the escort")
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
	enemy.Position = destination.Add(Vector{X: 36})
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
	enemy.Position = survey.Position.Add(Vector{X: 36})
	enemy.Heading = VectorFloat64{X: -1}
	require.Nil(t, scoutEconomicalLeg(&u.Game.Rules, p, scout, survey))
}

func TestCivilianTrafficCountsReportedArmedScout(t *testing.T) {
	u := logisticsUniverse(t)
	p, transport := u.Player(1), u.Fleet("Transport")
	report := patrolTestEnemy(0, p)
	p.FleetIntels = []*Fleet{report}
	destination := report.Position
	require.Zero(t, transitRouteThreat(p, transport, transport.Position, destination))
	p.ShipDesignIntels[len(p.ShipDesignIntels)-1].Spec.HasWeapons = true
	require.Positive(t, transitRouteThreat(p, transport, transport.Position, destination))
}

func TestCivilianTrafficChecksActualMovementStop(t *testing.T) {
	u := logisticsUniverse(t)
	p, transport := u.Player(1), u.Fleet("Transport")
	enemy := patrolTestEnemy(100)
	enemy.Position = Vector{25, 0}
	p.FleetIntels = []*Fleet{enemy}
	require.False(t, automaticFleetLegSafe(&u.Game.Rules, p, transport, transport.Position, Vector{75, 0}, 5, false))
	require.True(t, automaticFleetLegSafe(&u.Game.Rules, p, transport, transport.Position, Vector{75, 0}, 6, false))
}

func TestGalleonWaitsForScoutScanThenLandsWithEscort(t *testing.T) {
	galleon := ShipDesign{Name: "Landing carrier", Hull: Galleon.Name, Slots: []ShipDesignSlot{
		{HullComponent: SubGalacticFuelScoop.Name, HullSlotIndex: 1, Quantity: 4},
	}}
	u := newTestUniverse(t, TestScenario{
		Players: []ScenarioPlayer{{Designs: Designs(galleon, DesignLongRangeScout, DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{
			{Name: "Carrier", Design: galleon.Name, At: "Home", Cargo: Cargo{Colonists: 300}},
			{Name: "Scout", Design: DesignLongRangeScout.Name, At: "Home"},
			{Name: "Escort", Design: DesignDestroyerDelta.Name, At: "Home"},
		}}, {}},
		Planets: []ScenarioPlanet{
			{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 5000}},
			{Name: "Beachhead", Owner: 2, Position: Vector{36, 0}, Cargo: Cargo{Colonists: 10}},
		},
	})
	p, carrier, scout, escort := u.Player(1), u.Fleet("Carrier"), u.Fleet("Scout"), u.Fleet("Escort")
	home, target := u.Planet("Home"), u.Planet("Beachhead")
	target.Hab, target.BaseHab = home.Hab, home.BaseHab
	stale := *target
	stale.ReportAge = 4
	p.PlanetIntels = []*Planet{home, &stale}
	p.Automation.FleetRoles = map[int]string{carrier.Num: "freighter", scout.Num: "scout"}
	saveLogisticsWork(carrier, &logisticsJob{Kind: "invade", TargetNum: target.Num, DeliveryNum: target.Num, Position: target.Position})
	carrier.Waypoints = append(carrier.Waypoints, Waypoint{Position: target.Position, MapObjectTarget: target.ToTarget(), WarpSpeed: 6})
	u.Run((*turnGenerator).enforceAutomaticDeparture)
	require.Len(t, carrier.Waypoints, 1, "a previously saved approach must also wait for a scan")
	u.Run((*turnGenerator).planLogisticsAutomation, func(tg *turnGenerator) {
		require.Contains(t, logisticsWork(carrier).Status, "fresh reconnaissance")
		require.Len(t, carrier.Waypoints, 1)
		require.True(t, tg.planRecaptureReconnaissance(p, u.Game.getFleets(p.Num))[scout.Num])
	})
	u.Run((*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).scanColonyReconContacts)
	require.Zero(t, p.GetPlanetIntel(target.Num).ReportAge)
	enemy := patrolTestEnemy(1)
	enemy.Position = target.Position
	p.FleetIntels = []*Fleet{enemy}
	u.Run(func(tg *turnGenerator) {
		saved := logisticsWork(carrier)
		delete(carrier.Tags, logisticsTag)
		newJob := tg.findAutomaticInvasion(p, carrier)
		require.NotNil(t, newJob, "a fresh orbital guard report should allow a Galleon to request a landing convoy")
		require.Equal(t, target.Num, newJob.DeliveryNum)
		saveLogisticsWork(carrier, saved)
	})
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.Len(t, carrier.Waypoints, 1)
	require.Contains(t, logisticsWork(carrier).Status, "escort strength")
	p.Automation.FleetRoles[escort.Num] = "escort"
	u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).planAutomaticSafety, (*turnGenerator).enforceAutomaticDeparture)
	require.Equal(t, strconv.Itoa(carrier.Num), escort.GetTag(escortTag))
	require.Greater(t, len(carrier.Waypoints), 1)
	require.Equal(t, carrier.Waypoints[1].WarpSpeed, escort.Waypoints[1].WarpSpeed)
	p.FleetIntels = nil // The orbital guard has withdrawn before arrival.
	for range 4 {
		u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).enforceAutomaticDeparture, (*turnGenerator).fleetMove, (*turnGenerator).resolveAutomaticInvasions, (*turnGenerator).resolveColonistDrops)
		if target.PlayerNum == p.Num {
			break
		}
	}
	require.Equal(t, p.Num, target.PlayerNum)
}
