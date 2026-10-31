//go:build !wasi && !wasm

package cs

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRoutePreviewDoesNotAdvanceOrMutateGame(t *testing.T) {
	u := passengerUniverse(t)
	p := u.Player(1)
	p.Automation.FleetRoles[u.Fleet("Escort").Num] = "escort"
	before, err := json.Marshal(u.Game)
	require.NoError(t, err)
	plan, err := u.Game.PreviewAutomaticRoutes(p.Num)
	require.NoError(t, err)
	after, err := json.Marshal(u.Game)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after), "preview must not mutate source, cargo, fleets, year or production")
	require.Equal(t, u.Game.Year, plan.Year)
	require.NotEmpty(t, plan.Fleets)
	var escort *FleetRoutePreview
	for i := range plan.Fleets {
		if plan.Fleets[i].Num == u.Fleet("Escort").Num {
			escort = &plan.Fleets[i]
		}
	}
	require.NotNil(t, escort)
	require.Greater(t, len(escort.Waypoints), 1)
	require.NotEmpty(t, escort.Tags[escortStagingTag])
}

func TestEscortsKeepStagingRouteAndCanStillAcceptConvoy(t *testing.T) {
	u := logisticsUniverse(t)
	p, e, f := u.Player(1), u.Fleet("Escort"), u.Fleet("Transport")
	p.Automation.FleetRoles[e.Num] = "escort"
	p.PlanetIntels = append(p.PlanetIntels, &Planet{MapObject: MapObject{Num: 2, Position: Vector{100, 0}, ReportAge: ReportAgeUnexplored}})
	u.Run((*turnGenerator).planEscortStaging, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	require.Greater(t, len(e.Waypoints), 1)
	require.LessOrEqual(t, e.Waypoints[1].WarpSpeed, 4)
	goal := e.Waypoints[1].Position
	u.Run((*turnGenerator).planLogisticsAutomation, (*turnGenerator).planEscortStaging)
	require.Equal(t, goal, e.Waypoints[1].Position)
	p.FleetIntels = []*Fleet{patrolTestEnemy(1)}
	job := &logisticsJob{Kind: "salvage", TargetNum: 1, Position: Vector{50, 0}}
	u.Run(func(tg *turnGenerator) { tg.prepareConvoy(p, f, job, MapObjectTarget{}) })
	require.NotEmpty(t, e.GetTag(escortTag))
	require.Empty(t, e.GetTag(escortStagingTag))
}

func TestScoutReturnsDespiteSurroundingEnemyReports(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles[f.Num] = "scout"
	p.PlanetIntels = nil
	addExpeditionTestDock(p, Vector{200, 0})
	for i, pos := range []Vector{{30, 0}, {-30, 0}, {0, 30}, {0, -30}} {
		e := patrolTestEnemy(10)
		e.Num = i + 1
		e.Position = pos
		p.FleetIntels = append(p.FleetIntels, e)
	}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.routeEmergencyReturn(p, f)) }, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	require.NotEmpty(t, f.GetTag(emergencyRiskTag))
	require.True(t, fundedEmergencyReturn(&u.Game.Rules, p, f))
	require.Greater(t, len(f.Waypoints), 1)
	u.Run((*turnGenerator).fleetMove)
	require.NotEqual(t, Vector{}, f.Position)
}

func TestLateStationsAreAvailableToCorrectRacesAndPreserveNativeLimits(t *testing.T) {
	b, s := StaticTechStore.GetHull("EEE Bastion Station"), StaticTechStore.GetHull("EEE Sanctuary Star")
	require.NotNil(t, b)
	require.NotNil(t, s)
	count := func(h *TechHull, kind HullSlotType) int {
		n := 0
		for _, slot := range h.Slots {
			if slot.Type == kind {
				n += slot.Capacity
			}
		}
		return n
	}
	require.Equal(t, 120, count(b, HullSlotTypeWeapon))
	require.Equal(t, 128, count(s, HullSlotTypeWeapon))
	require.Equal(t, 3_750_000, s.MaxPopulation)
	require.Equal(t, []PRT{AR}, s.Requirements.PRTsRequired)
	require.Empty(t, b.Requirements.LRTsRequired)
	for _, h := range []*TechHull{b, s} {
		require.True(t, h.Starbase)
		require.Equal(t, UnlimitedSpaceDock, h.SpaceDock)
		seen := map[VectorFloat64]bool{h.SpaceDockSlotPosition: true}
		for _, slot := range h.Slots {
			require.False(t, seen[slot.Position])
			seen[slot.Position] = true
		}
	}
}

func TestRoutePreviewRestoresPacketAndMinefieldLookupMaps(t *testing.T) {
	u := logisticsUniverse(t)
	u.Player(1).Automation.FleetRoles = nil
	u.Game.MineralPackets = append(u.Game.MineralPackets, &MineralPacket{MapObject: MapObject{Num: 99, PlayerNum: 1, Position: Vector{500, 500}}})
	u.Game.Minefields = append(u.Game.Minefields, &Minefield{MapObject: MapObject{Num: 99, PlayerNum: 1, Position: Vector{600, 600}}})
	_, err := u.Game.PreviewAutomaticRoutes(1)
	require.NoError(t, err)
}

func TestPreviewCanPlanSecondPlayerWithoutChangingFirstPlayer(t *testing.T) {
	u := newTestUniverse(t, TestScenario{Players: []ScenarioPlayer{
		{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation)},
		{Designs: Designs(DesignDestroyerDelta, DesignSpaceStation), Fleets: []ScenarioFleet{{Name: "Second escort", Design: DesignDestroyerDelta.Name, At: "Second home"}}}},
		Planets: []ScenarioPlanet{{Name: "First home", Owner: 1, Position: Vector{0, 0}}, {Name: "Second home", Owner: 2, Position: Vector{1000, 1000}, Starbase: DesignSpaceStation.Name, Cargo: Cargo{Colonists: 2500}}, {Name: "Frontier", Position: Vector{1100, 1000}}}})
	p := u.Player(2)
	p.Automation.FleetRoles = map[int]string{u.Fleet("Second escort").Num: "escort"}
	plan, err := u.Game.PreviewAutomaticRoutes(2)
	require.NoError(t, err)
	require.Equal(t, 2, plan.PlayerNum)
	require.NotEmpty(t, plan.Fleets)
	for _, f := range plan.Fleets {
		require.Equal(t, "escort", f.Role)
	}
}

func TestImpossibleSalvageCoverReleasesEscortsAndPassengersPreemptWaitingSalvage(t *testing.T) {
	u := passengerUniverse(t)
	p, f, e := u.Player(1), u.Fleet("Transport"), u.Fleet("Escort")
	p.Automation.FleetRoles[e.Num] = "escort"
	e.SetTag(escortTag, "1")
	saveLogisticsWork(f, &logisticsJob{Kind: "salvage", TargetNum: 1, Position: Vector{50, 0}, Status: "Waiting for enough escort strength"})
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.Equal(t, "passengers", logisticsWork(f).Kind)
	require.Empty(t, e.GetTag(escortTag))
	p.FleetIntels = []*Fleet{patrolTestEnemy(100000000)}
	job := &logisticsJob{Kind: "salvage", TargetNum: 1, Position: Vector{50, 0}}
	u.Run(func(tg *turnGenerator) { tg.prepareConvoy(p, f, job, MapObjectTarget{}) })
	require.Empty(t, e.GetTag(escortTag), "an impossible mission must not reserve all free escorts indefinitely")
	require.Contains(t, job.Status, "Waiting")
}
