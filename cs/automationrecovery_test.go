//go:build !wasi && !wasm

package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestThreatZoneAllowsWithdrawalWithoutAllowingApproach(t *testing.T) {
	u := logisticsUniverse(t)
	p := u.Player(1)
	p.FleetIntels = []*Fleet{patrolTestEnemy(10)}
	require.True(t, automaticLegSafe(&u.Game.Rules, p, Vector{}, Vector{-30, 0}, 5, false))
	require.False(t, automaticLegSafe(&u.Game.Rules, p, Vector{}, Vector{30, 0}, 5, false))
}
func TestLegacyStrandedScoutMovesOnFundedSlowReturnAndStillRequestsRescue(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles = map[int]string{f.Num: "scout"}
	p.PlanetIntels = nil
	addExpeditionTestDock(p, Vector{})
	f.Position = Vector{100, 0}
	f.OrbitingPlanetNum = 0
	f.Fuel = 1
	for _, token := range f.Tokens {
		token.design.Spec.Engine.FuelUsage = []int{0, 0, 0, 0, 100000, 100000, 100000, 100000, 100000, 100000, 100000}
	}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.routeEmergencyReturn(p, f)) }, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	require.True(t, fundedEmergencyReturn(&u.Game.Rules, p, f))
	require.True(t, readAutomaticFuelRequest(f).Urgent)
	u.Run((*turnGenerator).fleetMove)
	require.Less(t, f.Position.X, 100)
	require.GreaterOrEqual(t, f.Fuel, 0)
}
func TestVanishedSalvageReleasesTransportForANewPassengerJob(t *testing.T) {
	u := passengerUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	f.Position = Vector{50, 0}
	p.SalvageIntels = append(p.SalvageIntels, &Salvage{MapObject: MapObject{Num: 999, Position: f.Position}, Cargo: Cargo{Ironium: 100}})
	saveLogisticsWork(f, &logisticsJob{Kind: "salvage", TargetNum: 999, Position: f.Position, Status: "Waiting for escort"})
	u.Run((*turnGenerator).planLogisticsAutomation)
	require.NotNil(t, logisticsWork(f))
	require.Equal(t, "passengers", logisticsWork(f).Kind)
	require.Equal(t, "999", f.GetTag("empty-salvage"))
	require.Greater(t, len(f.Waypoints), 1)
}
func TestPatrolSelectsFrontierAndKeepsEconomicalRouteThroughFuelGuard(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Escort")
	p.Automation.FleetRoles = map[int]string{f.Num: "patrol"}
	p.PlanetIntels = []*Planet{u.Planet("Home"), {MapObject: MapObject{Num: 2, Position: Vector{100, 0}, ReportAge: ReportAgeUnexplored}}}
	u.Run((*turnGenerator).planPatrolAutomation, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	require.Greater(t, len(f.Waypoints), 1)
	require.GreaterOrEqual(t, f.Waypoints[len(f.Waypoints)-1].Position.X, 60)
	require.LessOrEqual(t, f.Waypoints[1].WarpSpeed, 4)
}
func TestBombingWolfPackMergesAndRestoresOriginalRolesAfterMission(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b, e := u.Player(1), u.Fleet("Bomber"), u.Fleet("Cover")
	p.Automation.FleetRoles[e.Num] = "escort"
	u.Run((*turnGenerator).planBomberEscorts, (*turnGenerator).planBombingWolfPacks)
	require.NotNil(t, readBombingPack(b))
	require.True(t, e.Delete)
	require.Equal(t, 2, b.Spec.TotalShips)
	u.Planet("Enemy").PlayerNum = p.Num
	u.Run((*turnGenerator).finishBombingPacks)
	roles := map[string]int{}
	for _, f := range u.Game.getFleets(p.Num) {
		if !f.Delete {
			roles[p.Automation.FleetRoles[f.Num]] += f.Spec.TotalShips
			require.Empty(t, f.GetTag(bombingPackTag))
		}
	}
	require.Equal(t, 1, roles["bomber"])
	require.Equal(t, 1, roles["escort"])
}
func TestPatrolRecruitsFreeEscortAgainstBomberAndRestoresItsRole(t *testing.T) {
	u := logisticsUniverse(t)
	p, leader, escort := u.Player(1), u.Fleet("Escort"), u.Fleet("Combat")
	escort.Position = leader.Position
	p.Automation.FleetRoles = map[int]string{leader.Num: "patrol", escort.Num: "escort"}
	enemy := patrolTestEnemy(leader.Spec.PowerRating)
	enemy.Spec.Bomber = true
	p.FleetIntels = []*Fleet{enemy}
	addExpeditionTestDock(p, Vector{})
	u.Run((*turnGenerator).planPatrolAutomation)
	require.True(t, readPatrolJob(leader).Grouped)
	require.True(t, escort.Delete)
	p.FleetIntels = nil
	u.Run((*turnGenerator).planPatrolAutomation)
	escorts := 0
	for _, f := range u.Game.getFleets(p.Num) {
		if !f.Delete && p.Automation.FleetRoles[f.Num] == "escort" {
			escorts += f.Spec.TotalShips
		}
	}
	require.Equal(t, 2, escorts)
}

func TestManualSplitAllRestoresWolfPackShipRoles(t *testing.T) {
	u := automaticBomberUniverse(t)
	p, b, e := u.Player(1), u.Fleet("Bomber"), u.Fleet("Cover")
	p.Automation.FleetRoles[e.Num] = "escort"
	u.Run((*turnGenerator).planBomberEscorts, (*turnGenerator).planBombingWolfPacks)
	children, err := NewOrderer().SplitAll(&u.Game.Rules, p, u.Game.getFleets(p.Num), b)
	require.NoError(t, err)
	roles := map[string]int{}
	for _, f := range append([]*Fleet{b}, children...) {
		roles[p.Automation.FleetRoles[f.Num]] += f.Spec.TotalShips
		require.Empty(t, f.GetTag(bombingPackTag))
	}
	require.Equal(t, 1, roles["bomber"])
	require.Equal(t, 1, roles["escort"])
}
func TestFullNativeTurnLoadsTransportAndStartsMovingImmediately(t *testing.T) {
	u := passengerUniverse(t)
	source, f := u.Planet("Home"), u.Fleet("Transport")
	u.Planet("Young colony").Hab = Hab{50, 50, 50}
	source.Cargo.Colonists = 20000
	u.GenerateTurn()
	require.NotEqual(t, source.Position, f.Position)
	require.Positive(t, f.Cargo.Colonists)
}

func TestEmergencyReturnCanChooseFundedRiskInsteadOfAnExcessivelyLongThreatDetour(t *testing.T) {
	u := logisticsUniverse(t)
	p, f := u.Player(1), u.Fleet("Transport")
	p.Automation.FleetRoles = map[int]string{f.Num: "miner"}
	p.PlanetIntels = nil
	addExpeditionTestDock(p, Vector{200, 0})
	p.FleetIntels = []*Fleet{patrolTestEnemy(10)}
	p.FleetIntels[0].Position = Vector{90, 0}
	f.Fuel = 2
	for _, token := range f.Tokens {
		token.design.Spec.Engine.FuelUsage = []int{0, 0, 0, 0, 100000, 100000, 100000, 100000, 100000, 100000, 100000}
	}
	u.Run(func(tg *turnGenerator) { require.True(t, tg.routeEmergencyReturn(p, f)) }, (*turnGenerator).guardAutomaticFuel, (*turnGenerator).enforceAutomaticDeparture)
	require.Greater(t, len(f.Waypoints), 1)
	require.True(t, fundedEmergencyReturn(&u.Game.Rules, p, f))
	for i := 1; i < len(f.Waypoints); i++ {
		require.True(t, automaticLegSafe(&u.Game.Rules, p, f.Waypoints[i-1].Position, f.Waypoints[i].Position, f.Waypoints[i].WarpSpeed, f.GetTag(emergencyRiskTag) != ""))
	}
}
