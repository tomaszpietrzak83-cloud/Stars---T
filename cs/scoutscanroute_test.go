//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEdgeStartPrefersFivePercentInsetOnRectangularMaps(t *testing.T) {
	for _, size := range []Vector{{1000, 1000}, {2000, 1000}} {
		for _, points := range [][]Vector{
			{{1, 500}, {size.X / 20, 500}, {size.X / 2, 500}},
			{{size.X / 2, 1}, {size.X / 2, 50}, {size.X / 2, 500}},
			{{size.X - 1, 500}, {size.X - size.X/20, 500}, {size.X / 2, 500}},
			{{size.X / 2, 999}, {size.X / 2, 950}, {size.X / 2, 500}},
		} {
			ug := newPlacementGenerator(PlayerPositionsModerate, 1, size, newIntRandom(0), points...)
			ug.HostStartPosition = "edge"
			positions, err := ug.getPlayerPlanets()
			require.NoError(t, err)
			require.Equal(t, 1, positions[0])
		}
	}
}

func TestScannerExplorationCoversClusterWithoutOrbitAndFundsReturn(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	f.Spec.ScanRangePen = 32
	p.PlanetIntels = nil
	addExpeditionTestDock(p, Vector{})
	for i, point := range []Vector{{100, -20}, {100, 0}, {100, 20}} {
		p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{Num: i + 1, Position: point, ReportAge: ReportAgeUnexplored}})
	}
	claimed := map[int]int{}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.planScannerExploration(p, f, claimed, 1)) })
	require.Len(t, f.Waypoints, 3, "one shared observation stop and the return dock")
	stop := f.Waypoints[1]
	require.Empty(t, stop.TargetType)
	for _, planet := range p.PlanetIntels[1:] {
		require.NotEqual(t, planet.Position, stop.Position)
		require.LessOrEqual(t, stop.Position.DistanceTo(planet.Position), float64(f.Spec.ScanRangePen))
		require.Equal(t, f.Num, claimed[planet.Num])
	}
	require.True(t, fundedScoutWaypoints(&u.Game.Rules, p, f))
	// Verify these coordinates through native scanning rather than granting intel.
	f.Position = stop.Position
	scan := newPlayerScanner(u.Game.Universe, u.Game.Players, &u.Game.Rules, p)
	for _, planet := range p.PlanetIntels[1:] {
		scanned, err := scan.scanPlanet(planet, scanner{Position: f.Position, RangePen: f.Spec.ScanRangePen})
		require.NoError(t, err)
		require.True(t, scanned)
	}
}

func TestOrdinaryScannerStillRequiresPlanetOrbit(t *testing.T) {
	f := &Fleet{Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{ScanRange: 500, ScanRangePen: NoScanner}}}
	require.Zero(t, scoutPlanetScanRadius(f))
	require.Equal(t, Vector{100, 0}, scoutScanPoint(Vector{}, Vector{100, 0}, scoutPlanetScanRadius(f)))
}

func TestFleetScanStopUsesReportedMotionCloakingAndOrbitRules(t *testing.T) {
	f := &Fleet{Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{ScanRange: 100, ScanRangePen: 20}}}
	e := &Fleet{MapObject: MapObject{Position: Vector{200, 0}}, Heading: VectorFloat64{-1, 0}, WarpSpeed: 5}
	point := scoutFleetScanPoint(f, e)
	predicted := e.Position.ToFloat64().Add(e.Heading.Normalized().Scale(e.observedStrategicSpeed())).ToInt(true)
	require.LessOrEqual(t, point.DistanceTo(predicted), 100.0)
	require.NotEqual(t, predicted, point)
	e.OrbitingPlanetNum = 1
	require.LessOrEqual(t, scoutFleetScanPoint(f, e).DistanceTo(e.Position), 20.0)
	f.Spec.ScanRangePen = NoScanner
	require.Equal(t, e.Position, scoutFleetScanPoint(f, e))
}

func TestPriorityColonyReconRefreshesSeveralOldReportsFromSpace(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{{Designs: Designs(DesignLongRangeScout, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Scout", Design: DesignLongRangeScout.Name, At: "Home"}}}, {}}, Planets: []ScenarioPlanet{{Name: "Home", Owner: 1, Starbase: DesignSpaceStation.Name}, {Name: "Enemy A", Owner: 2, Position: Vector{100, -20}}, {Name: "Enemy B", Owner: 2, Position: Vector{100, 0}}, {Name: "Enemy C", Owner: 2, Position: Vector{100, 20}}}})
	p, f := u.Player(1), u.Fleet("Scout")
	f.Spec.ScanRangePen = 32
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.Automation.PriorityPlanets = map[int]bool{u.Planet("Enemy A").Num: true}
	p.PlanetIntels = []*Planet{u.Planet("Home")}
	for _, name := range []string{"Enemy A", "Enemy B", "Enemy C"} {
		report := *u.Planet(name)
		report.ReportAge = 6
		p.PlanetIntels = append(p.PlanetIntels, &report)
	}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.planPriorityColonyRecon(p, f)) })
	last := f.Waypoints[len(f.Waypoints)-1]
	require.Empty(t, last.TargetType)
	for _, planet := range p.PlanetIntels[1:] {
		require.LessOrEqual(t, last.Position.DistanceTo(planet.Position), float64(f.Spec.ScanRangePen))
		require.Equal(t, 6, planet.ReportAge, "planning cannot grant a scan before movement")
	}
	reserve := automaticReturnFuel(&u.Game.Rules, p, f, last.Position, expeditionStations(p, f), true)
	_, _, _, ok := automaticLegFuel(p, f, last.WarpSpeed, f.Position.DistanceTo(last.Position), f.Fuel, reserve)
	require.True(t, ok, "actual movement must leave a funded return")
}

func TestFleetReconChoosesSharedScanAreaForIncomingReports(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	f.Spec.ScanRange = 32
	f.Spec.ScanRangePen = NoScanner
	p.PlanetIntels = []*Planet{{MapObject: MapObject{Num: 1, PlayerNum: p.Num, Position: Vector{}}}}
	p.FleetIntels = nil
	for i, y := range []int{-15, 0, 15} {
		p.FleetIntels = append(p.FleetIntels, &Fleet{MapObject: MapObject{Num: i + 1, PlayerNum: 2, Position: Vector{100, y}}, WarpSpeed: 5, Heading: VectorFloat64{-1, 0}, Spec: FleetSpec{ShipDesignSpec: ShipDesignSpec{HasWeapons: true}}})
	}
	point := scoutFleetReconPoint(p, f, p.FleetIntels[0])
	for _, enemy := range p.FleetIntels {
		require.LessOrEqual(t, point.DistanceTo(scoutReportedFleetPosition(enemy)), scoutFleetScanRadius(f, enemy))
	}
}
